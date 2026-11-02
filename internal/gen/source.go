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
	"slices"
	"strings"
	"time"
)

// TagResolver finds the project of a working directory and caches the answer.
type TagResolver struct {
	cache map[string][2]string
}

// NewTagResolver returns an empty resolver.
func NewTagResolver() *TagResolver { return &TagResolver{cache: map[string][2]string{}} }

var regJobTag = regexp.MustCompile(`^\s*export\s+JOB_TAG=["']?([^"'\s]+)`)

// Resolve returns the project tag and the repository name of cwd.
// The tag is the first "export JOB_TAG=..." found in .envrc from cwd up to the root.
// Only the lines are read: direnv is not run, because a finished worktree's
// directory is gone and some .envrc build a nix flake or decrypt secrets.
// Without a JOB_TAG, the repository name is the tag.
func (r *TagResolver) Resolve(cwd string) (string, string) {
	if v, ok := r.cache[cwd]; ok {
		return v[0], v[1]
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
	r.cache[cwd] = [2]string{tag, repo}
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
			return worktreeRepo(dir)
		}
	}
	for dir := cwd; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if before, ok := strings.CutSuffix(filepath.Base(filepath.Dir(dir)), "-worktree"); ok {
			return before
		}
	}
	return filepath.Base(cwd)
}

func worktreeRepo(top string) string {
	if before, ok := strings.CutSuffix(filepath.Base(filepath.Dir(top)), "-worktree"); ok {
		return before
	}
	return filepath.Base(top)
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

func sortActivities(acts []Activity) {
	slices.SortFunc(acts, func(a, b Activity) int { return a.At.Compare(b.At) })
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

// Task is a task directory split by the tasks command.
type Task struct {
	Path    string // relative to the store root: tasks/<repo>/<dir>/index.md
	Title   string // the directory name
	Repo    string
	Branch  string
	Done    bool
	Created time.Time
}

const taskDirLayout = "20060102-150405"

// ReadTasks reads every task under storeRoot/tasks.
func ReadTasks(storeRoot string, loc *time.Location) ([]Task, error) {
	paths, err := filepath.Glob(filepath.Join(storeRoot, "tasks", "*", "*", "index.md"))
	if err != nil {
		return nil, err
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
			t.Done = strings.Contains(v, "✅")
		}
	}
	return t, true
}

func frontMatter(content string) string {
	rest, ok := strings.CutPrefix(content, "---\n")
	if !ok {
		return ""
	}
	fm, _, _ := strings.Cut(rest, "\n---")
	return fm
}
