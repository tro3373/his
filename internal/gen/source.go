package gen

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// TagResolver finds the project of a working directory and caches the answer.
type TagResolver struct {
	cache map[string]resolved
}

type resolved struct{ tag, repo string }

// NewTagResolver returns an empty resolver.
func NewTagResolver() *TagResolver { return &TagResolver{cache: map[string]resolved{}} }

var regJobTag = regexp.MustCompile(`^\s*export\s+JOB_TAG=["']?([^"'\s]+)`)

// Resolve returns the project tag and the repository name of cwd.
// The tag is the first "export JOB_TAG=..." found in .envrc from cwd up to the root.
// Only the lines are read: direnv is not run, because a finished worktree's
// directory is gone and some .envrc build a nix flake or decrypt secrets.
// Without a JOB_TAG, the repository name is the tag.
func (r *TagResolver) Resolve(cwd string) (string, string) {
	if v, ok := r.cache[cwd]; ok {
		return v.tag, v.repo
	}
	repo := repoName(cwd)
	tag := repo
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if t, ok := readJobTag(filepath.Join(dir, ".envrc")); ok {
			tag = t
			break
		}
		if dir == filepath.Dir(dir) {
			break
		}
	}
	r.cache[cwd] = resolved{tag, repo}
	return tag, repo
}

func readJobTag(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	for l := range strings.SplitSeq(string(data), "\n") {
		if m := regJobTag.FindStringSubmatch(l); m != nil {
			return m[1], true
		}
	}
	return "", false
}

// repoName is the repository of cwd, named as tasks names it: the directory
// holding .git, and for a worktree at "<repo>-worktree/<name>", <repo>.
// A session may run in a subdirectory, so the search goes up from cwd.
// A finished worktree is deleted; then the "-worktree" directory in cwd decides.
func repoName(cwd string) string {
	for dir := cwd; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return repoAt(dir)
		}
	}
	for dir := cwd; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if repo, ok := worktreeOf(dir); ok {
			return repo
		}
	}
	return filepath.Base(cwd)
}

func repoAt(top string) string {
	if repo, ok := worktreeOf(top); ok {
		return repo
	}
	return filepath.Base(top)
}

// worktreeOf returns <repo> when dir is "<repo>-worktree/<name>".
func worktreeOf(dir string) (string, bool) {
	return strings.CutSuffix(filepath.Base(filepath.Dir(dir)), "-worktree")
}

type sessionEntry struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Cwd       string `json:"cwd"`
	GitBranch string `json:"gitBranch"`
	// Entrypoint is "cli" for an interactive session and "sdk-cli" for claude -p.
	Entrypoint string `json:"entrypoint"`
}

// ReadActivities reads every user and assistant message at or after since
// in the session logs under root, subagents included.
// Files last written before since are skipped without opening them.
// A missing root (a config dir never used) has no activity.
func ReadActivities(root string, since time.Time, r *TagResolver) ([]Activity, error) {
	if _, err := os.Stat(root); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	var acts []Activity
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().Before(since) {
			return nil
		}
		got, err := readSession(path, since, r)
		if err != nil {
			return err
		}
		acts = append(acts, got...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions under %s: %w", root, err)
	}
	return acts, nil
}

func readSession(path string, since time.Time, r *TagResolver) ([]Activity, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var acts []Activity
	// A line holds a whole message and can be megabytes long, more than bufio.Scanner allows.
	br := bufio.NewReader(f)
	for {
		line, err := br.ReadBytes('\n')
		if a, ok := parseSessionLine(line, since, r); ok {
			acts = append(acts, a)
		}
		if errors.Is(err, io.EOF) {
			return acts, nil
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", path, err)
		}
	}
}

func parseSessionLine(line []byte, since time.Time, r *TagResolver) (Activity, bool) {
	var e sessionEntry
	if json.Unmarshal(line, &e) != nil {
		return Activity{}, false
	}
	// claude -p is run by cron and by tools (rec's minutes, the daily report):
	// nobody is at work then.
	if e.Type != "user" && e.Type != "assistant" || e.Cwd == "" || e.Entrypoint == "sdk-cli" {
		return Activity{}, false
	}
	at, err := time.Parse(time.RFC3339Nano, e.Timestamp)
	if err != nil || at.Before(since) {
		return Activity{}, false
	}
	tag, repo := r.Resolve(e.Cwd)
	return Activity{At: at, Cwd: e.Cwd, Tag: tag, Repo: repo, Branch: e.GitBranch}, true
}

// Recording is a call recorded by rec.
type Recording struct {
	Start time.Time
	End   time.Time
	App   string // the process that held the microphone: slack, zoom, chrome
	Title string
}

const (
	recDirLayout = "20060102-150405"
	// minRecording drops calls too short to be a meeting (a mic test, a misfire).
	minRecording = 5 * time.Minute
)

var regOffset = regexp.MustCompile(`^- \[(\d+):(\d{2}):(\d{2})\]`)

// ReadRecordings reads the recordings rec wrote under dir that started at or after since.
func ReadRecordings(dir string, since time.Time) ([]Recording, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read recordings under %s: %w", dir, err)
	}
	var recs []Recording
	for _, e := range entries {
		start, err := time.ParseInLocation(recDirLayout, e.Name(), since.Location())
		if !e.IsDir() || err != nil || start.Before(since) {
			continue
		}
		read := func(name string) []byte {
			b, _ := os.ReadFile(filepath.Join(dir, e.Name(), name))
			return b
		}
		if r, ok := parseRecording(e.Name(), read("meta.json"), read("transcript.md"), read("minutes.md"), since.Location()); ok {
			recs = append(recs, r)
		}
	}
	return recs, nil
}

// parseRecording builds a recording from the files of one rec output directory.
// The length is the offset of the last utterance. A recording without the app
// (made before rec kept it), without utterances, or under 5 minutes is dropped.
func parseRecording(name string, meta, transcript, minutes []byte, loc *time.Location) (Recording, bool) {
	start, err := time.ParseInLocation(recDirLayout, name, loc)
	if err != nil {
		return Recording{}, false
	}
	var m struct {
		App string `json:"app"`
	}
	if json.Unmarshal(meta, &m) != nil || m.App == "" {
		return Recording{}, false
	}
	var length time.Duration
	for l := range strings.SplitSeq(string(transcript), "\n") {
		g := regOffset.FindStringSubmatch(l)
		if g == nil {
			continue
		}
		d, err := time.ParseDuration(g[1] + "h" + g[2] + "m" + g[3] + "s")
		if err == nil {
			length = d
		}
	}
	if length < minRecording {
		return Recording{}, false
	}
	return Recording{Start: start, End: start.Add(length), App: m.App, Title: minutesTitle(minutes)}, true
}

// minutesTitle is the first heading of the minutes, without "議事録:".
// Minutes that do not start with a heading (a failed generation) have no title.
func minutesTitle(minutes []byte) string {
	first, _, _ := strings.Cut(string(minutes), "\n")
	title, ok := strings.CutPrefix(first, "# ")
	if !ok {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(title), "議事録:"))
}

// Task is a task directory under the store, made by the tasks command or task2.
type Task struct {
	Path    string // relative to the store root: tasks/<repo>/<dir>/<one of taskFiles>
	Title   string // the directory name
	Repo    string
	Branch  string
	Status  taskStatus
	Created time.Time
	Started time.Time // zero when the file has none: index.md, or a task2 task never started
	Ended   time.Time // zero while the task is open, and always for index.md
}

// taskStatus is where a task stands, whichever file it came from.
type taskStatus int

const (
	taskOpen taskStatus = iota
	taskNotStarted
	taskDone
)

// parseTaskStatus reads the status of either task file:
// a check mark in index.md, a word in task.md. Anything else is open.
func parseTaskStatus(v string) taskStatus {
	switch {
	case strings.Contains(v, "✅") || v == "done":
		return taskDone
	case v == "todo" || v == "ready":
		return taskNotStarted
	}
	return taskOpen
}

// workedIn reports whether a was work on this task: on its branch, while it was open.
// The window matters for a branch that outlives a task, such as main:
// without it every later and earlier day on that branch would list the task.
func (t Task) workedIn(a Activity) bool {
	if t.Branch == "" || a.Repo != t.Repo || a.Branch != t.Branch || a.At.Before(t.opened()) {
		return false
	}
	return t.Ended.IsZero() || !a.At.After(t.Ended)
}

// opened is when work on the task can begin: when it was started, else when it was split.
func (t Task) opened() time.Time {
	if t.Started.IsZero() {
		return t.Created
	}
	return t.Started
}

const taskDirLayout = "20060102-150405"

// taskFiles are the names a task's file goes by: index.md from the tasks command,
// task.md from task2.
var taskFiles = []string{"index.md", "task.md"}

// ReadTasks reads every task under storeRoot/tasks.
func ReadTasks(storeRoot string, loc *time.Location) ([]Task, error) {
	var paths []string
	for _, name := range taskFiles {
		found, err := filepath.Glob(filepath.Join(storeRoot, "tasks", "*", "*", name))
		if err != nil {
			return nil, err
		}
		paths = append(paths, found...)
	}
	var tasks []Task
	for _, p := range paths {
		rel, err := filepath.Rel(storeRoot, p)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("failed to read task %s: %w", p, err)
		}
		if t, ok := parseTask(rel, data, loc); ok {
			tasks = append(tasks, t)
		}
	}
	return tasks, nil
}

// parseTask reads a task's front matter. Only directories named
// "<YYYYMMDD-HHMMSS>_<name>" are tasks; the time is when it was split.
func parseTask(rel string, content []byte, loc *time.Location) (Task, bool) {
	dir := filepath.Base(filepath.Dir(rel))
	stamp, _, ok := strings.Cut(dir, "_")
	if !ok {
		return Task{}, false
	}
	created, err := time.ParseInLocation(taskDirLayout, stamp, loc)
	if err != nil {
		return Task{}, false
	}
	t := Task{Path: rel, Title: dir, Repo: filepath.Base(filepath.Dir(filepath.Dir(rel))), Created: created}
	for l := range strings.SplitSeq(frontMatter(string(content)), "\n") {
		k, v, _ := strings.Cut(l, ":")
		v = strings.TrimSpace(v)
		switch k {
		case "branch":
			t.Branch = v
		case "status":
			t.Status = parseTaskStatus(v)
		case "started":
			t.Started = parseFrontMatterTime(v, loc)
		case "ended":
			t.Ended = parseFrontMatterTime(v, loc)
		}
	}
	return t, true
}

// parseFrontMatterTime is zero for an unparsable value:
// the task then counts as open, so its link stays rather than vanishing.
func parseFrontMatterTime(v string, loc *time.Location) time.Time {
	at, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}
	}
	return at.In(loc)
}

func frontMatter(content string) string {
	rest, ok := strings.CutPrefix(content, "---\n")
	if !ok {
		return ""
	}
	fm, _, _ := strings.Cut(rest, "\n---")
	return fm
}
