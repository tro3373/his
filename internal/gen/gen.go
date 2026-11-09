package gen

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Config is the "gen" section of the his config file.
type Config struct {
	StoreRoot string         `mapstructure:"store_root"` // holds tasks/; links are relative to it
	RecDir    string         `mapstructure:"rec_dir"`    // rec's output directory
	Sessions  []string       `mapstructure:"sessions"`   // <CLAUDE_CONFIG_DIR>/projects of every config dir
	Lunch     Lunch          `mapstructure:"lunch"`
	Logs      []LogConfig    `mapstructure:"logs"`
	Activity  ActivityConfig `mapstructure:"activity"`

	Lag         time.Duration `mapstructure:"lag"`           // a day ends no later than this long before now
	CatchUpDays int           `mapstructure:"catch_up_days"` // how far back a run without a date looks
	MinIdle     time.Duration `mapstructure:"min_idle"`      // how long a stretch without activity must last to be a break
}

// ActivityConfig is the "gen.activity" section: where shells and nvim append
// the Activity log, and which of its working directories to leave out.
type ActivityConfig struct {
	Dir           string   `mapstructure:"dir"`
	RetentionDays int      `mapstructure:"retention_days"` // older day files are deleted by gen
	Ignore        []string `mapstructure:"ignore"`         // working directories excluded by exact match, not by prefix
}

// DefaultConfig is the Config a config file overrides key by key.
// CatchUpDays matches Claude Code deleting session logs after 30 days (cleanupPeriodDays).
func DefaultConfig() Config {
	return Config{
		Lag: 30 * time.Minute, CatchUpDays: 30, MinIdle: time.Hour,
		Activity: ActivityConfig{Dir: "~/.local/state/his/activity", RetentionDays: 35, Ignore: []string{"~"}},
	}
}

// LogConfig is one log to write.
// A session belongs to a log by its working directory, not by its config dir:
// one config dir may hold sessions of several logs.
type LogConfig struct {
	Name  string   `mapstructure:"name"`
	Dir   string   `mapstructure:"dir"`   // <Dir>/<YYYY>/<YYYY-MM-DD>-log.md
	Roots []string `mapstructure:"roots"` // working directories under these belong to this log
	Apps  []string `mapstructure:"apps"`  // rec apps whose calls belong to this log
	// Default takes the sessions under no log's roots, and the tasks whose
	// repository had no session.
	Default bool `mapstructure:"default"`
}

// Validate reports the config errors that would make a run read or write the wrong place.
func (c Config) Validate() error {
	var errs []error
	if c.StoreRoot == "" {
		errs = append(errs, errors.New("gen.store_root is empty"))
	}
	if len(c.Sessions) == 0 || slices.Contains(c.Sessions, "") {
		errs = append(errs, errors.New("gen.sessions needs at least one non-empty directory"))
	}
	if c.Lunch.End <= c.Lunch.Start {
		errs = append(errs, errors.New("gen.lunch needs a start before its end"))
	}
	if len(c.Logs) == 0 {
		errs = append(errs, errors.New("gen.logs is empty"))
	}
	if c.Lag < 0 {
		errs = append(errs, errors.New("gen.lag is negative"))
	}
	if c.CatchUpDays < 0 {
		errs = append(errs, errors.New("gen.catch_up_days is negative"))
	}
	if c.MinIdle <= 0 {
		errs = append(errs, errors.New("gen.min_idle must be positive"))
	}
	if c.Activity.Dir == "" {
		errs = append(errs, errors.New("gen.activity.dir is empty"))
	}
	if slices.Contains(c.Activity.Ignore, "") {
		errs = append(errs, errors.New(`gen.activity.ignore has an empty entry: a bare ~ is YAML null, write "~"`))
	}
	if c.Activity.RetentionDays < c.CatchUpDays {
		errs = append(errs, fmt.Errorf("gen.activity.retention_days (%d) is shorter than gen.catch_up_days (%d)", c.Activity.RetentionDays, c.CatchUpDays))
	}
	defaults := 0
	for _, l := range c.Logs {
		if l.Dir == "" {
			errs = append(errs, fmt.Errorf("gen.logs %q has no dir", l.Name))
		}
		if l.Default {
			defaults++
		}
	}
	if defaults > 1 {
		errs = append(errs, errors.New("more than one gen.logs is default"))
	}
	return errors.Join(errs...)
}

// meetingTagWindow is how far around a call the session activity decides its project.
const meetingTagWindow = 30 * time.Minute

// Run writes the days of every log; logDates tells which.
// A day is only added to: what follows its latest time heading is appended.
// A zero target writes each log from yesterday, or from an older newest written day, through today.
// Any other target is that day only.
// Every day ends no later than cfg.Lag before now.
func Run(cfg Config, now, target time.Time, w io.Writer) error {
	loc := now.Location()
	today := midnight(now)
	rules := DayRules{Lunch: cfg.Lunch, MinIdle: cfg.MinIdle, Limit: now.Add(-cfg.Lag)}
	since := today.AddDate(0, 0, -cfg.CatchUpDays)
	if !target.IsZero() {
		since = midnight(target)
	}

	tasks, err := ReadTasks(cfg.StoreRoot, loc)
	if err != nil {
		return err
	}
	recs, err := ReadRecordings(cfg.RecDir, since)
	if err != nil {
		return err
	}
	resolver := NewTagResolver()
	var all []Activity
	for _, dir := range cfg.Sessions {
		got, err := ReadActivities(dir, since, resolver)
		if err != nil {
			return err
		}
		all = append(all, got...)
	}
	shell, err := ReadShellActivities(cfg.Activity, today, since, resolver)
	if err != nil {
		return err
	}
	all = append(all, shell...)
	acts := make([][]Activity, len(cfg.Logs))
	for _, a := range all {
		if i := logOf(cfg.Logs, a.Cwd); i >= 0 {
			acts[i] = append(acts[i], a)
		}
	}
	owner := taskOwners(cfg.Logs, acts)
	done := finishedPaths(tasks)

	for i, l := range cfg.Logs {
		dates := []time.Time{since}
		if target.IsZero() {
			dates, err = logDates(l.Dir, today, cfg.CatchUpDays)
			if err != nil {
				return err
			}
		}
		for _, d := range dates {
			dayActs := between(acts[i], d, d.AddDate(0, 0, 1))
			meets := meetingsOf(recs, l.Apps, d, dayActs)
			path, content, err := monthFile(l.Dir, d)
			if err != nil {
				return err
			}
			dayRules := rules
			dayRules.From = SettledUntil(content, d)
			var block []string
			var links []Link
			if day, ok := PlanDay(d, dayRules, dayActs, meets); ok {
				from := maxTime(d, dayRules.From)
				links = linksOf(tasks, day, between(dayActs, from, d.AddDate(0, 0, 1)), from, func(t Task) bool { return owner(t.Repo) == i })
				block = RenderDay(day, links)
			}
			if err := writeDay(path, content, d, block, links, done, w); err != nil {
				return err
			}
		}
	}
	return nil
}

// logDates is the days a run without a date writes to the log in dir:
// from yesterday, or from the newest day whose section already has a time heading
// when that is older, through today.
// With no such day within catchUpDays, it starts catchUpDays back.
// Yesterday is always included because the daily run at noon finishes it:
// a day written by hand before that must not freeze it at half a day.
// Days older than the start are never touched: their session logs may be gone already.
func logDates(dir string, today time.Time, catchUpDays int) ([]time.Time, error) {
	yesterday := today.AddDate(0, 0, -1)
	start := today.AddDate(0, 0, -catchUpDays)
	for i := 0; i <= catchUpDays; i++ {
		d := today.AddDate(0, 0, -i)
		written, err := dayWritten(dir, d)
		if err != nil {
			return nil, err
		}
		if written {
			start = d
			break
		}
	}
	if start.After(yesterday) {
		start = yesterday
	}
	var dates []time.Time
	for d := start; !d.After(today); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d)
	}
	return dates, nil
}

// dayWritten reports whether the log in dir has a time heading in date's section.
func dayWritten(dir string, date time.Time) (bool, error) {
	_, content, err := monthFile(dir, date)
	if err != nil {
		return false, err
	}
	ls := strings.Split(content, "\n")
	head, end, found := findSection(ls, dayKey(date))
	return found && hasTimeHeading(ls[head+1:end]), nil
}

// logOf is the log whose roots hold cwd, else the default log, else -1.
func logOf(logs []LogConfig, cwd string) int {
	for i, l := range logs {
		for _, root := range l.Roots {
			if cwd == root || strings.HasPrefix(cwd, strings.TrimSuffix(root, "/")+"/") {
				return i
			}
		}
	}
	return defaultLog(logs)
}

// defaultLog is the index of the default log, or -1.
func defaultLog(logs []LogConfig) int {
	return slices.IndexFunc(logs, func(l LogConfig) bool { return l.Default })
}

func midnight(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func between(acts []Activity, from, to time.Time) []Activity {
	var out []Activity
	for _, a := range acts {
		if !within(a.At, from, to) {
			continue
		}
		out = append(out, a)
	}
	return out
}

// taskOwners decides which log a repository's tasks go to: the first log whose
// sessions worked in it, else the default log. -1 means no log.
func taskOwners(logs []LogConfig, acts [][]Activity) func(repo string) int {
	fallback := defaultLog(logs)
	return func(repo string) int {
		for i := range logs {
			if slices.ContainsFunc(acts[i], func(a Activity) bool { return a.Repo == repo }) {
				return i
			}
		}
		return fallback
	}
}

// meetingsOf turns the day's recordings made by apps into meetings.
// A meeting's project is the one most active around it, then the day's, then MTG.
func meetingsOf(recs []Recording, apps []string, date time.Time, dayActs []Activity) []Meeting {
	var out []Meeting
	next := date.AddDate(0, 0, 1)
	for _, r := range recs {
		if !slices.Contains(apps, r.App) || !within(r.Start, date, next) {
			continue
		}
		tag := dominantTag(between(dayActs, r.Start.Add(-meetingTagWindow), r.End.Add(meetingTagWindow)))
		tag = cmp.Or(tag, dominantTag(dayActs), "MTG")
		out = append(out, Meeting{Start: r.Start, End: r.End, Tag: tag, Title: r.Title})
	}
	return out
}

func dominantTag(acts []Activity) string {
	count := map[string]int{}
	for _, a := range acts {
		count[a.Tag]++
	}
	best := ""
	for tag, n := range count {
		if n > count[best] || n == count[best] && tag < best {
			best = tag
		}
	}
	return best
}

// finishedPaths is where the finished tasks are, as the logs link them.
func finishedPaths(tasks []Task) []string {
	var out []string
	for _, t := range tasks {
		if t.Done {
			out = append(out, t.Path)
		}
	}
	return out
}

// linksOf lists the tasks of the day from from on: the ones worked on that day (see Task.workedIn),
// and the ones split that day that belong to this log, unless not started yet.
// Each goes under the segment where its branch was most active, else under the
// segment it was split in, else under the last segment that is not lunch.
func linksOf(tasks []Task, day Day, dayActs []Activity, from time.Time, ownTask func(Task) bool) []Link {
	next := midnight(from).AddDate(0, 0, 1)
	var picked []Task
	for _, t := range tasks {
		worked := slices.ContainsFunc(dayActs, t.workedIn)
		created := within(t.Created, from, next) && ownTask(t) && !t.NotStarted
		if worked || created {
			picked = append(picked, t)
		}
	}
	slices.SortFunc(picked, func(a, b Task) int { return cmp.Or(a.Created.Compare(b.Created), cmp.Compare(a.Path, b.Path)) })
	links := make([]Link, 0, len(picked))
	for _, t := range picked {
		links = append(links, Link{Path: t.Path, Title: t.Title, Done: t.Done, Segment: segmentOf(t, day, dayActs)})
	}
	return links
}

func segmentOf(t Task, day Day, dayActs []Activity) int {
	count := make([]int, len(day.Segments))
	for _, a := range dayActs {
		if !t.workedIn(a) {
			continue
		}
		if i := segmentIndex(day, a.At); i >= 0 {
			count[i]++
		}
	}
	best := -1
	for i, n := range count {
		if n > 0 && (best < 0 || n > count[best]) {
			best = i
		}
	}
	if best >= 0 {
		return best
	}
	if i := segmentIndex(day, t.Created); i >= 0 {
		return i
	}
	last := 0
	for i, s := range day.Segments {
		if s.Kind != KindBreak {
			last = i
		}
	}
	return last
}

// segmentIndex is the work or meeting segment that holds at, or -1.
func segmentIndex(day Day, at time.Time) int {
	for i, s := range day.Segments {
		end := day.End
		if i+1 < len(day.Segments) {
			end = day.Segments[i+1].Start
		}
		if s.Kind != KindBreak && within(at, s.Start, end) {
			return i
		}
	}
	return -1
}

func writeDay(path, content string, date time.Time, block []string, links []Link, done []string, w io.Writer) error {
	merged := MergeDay(content, date, block, links, done)
	if merged == content {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(merged), 0o600); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	_, _ = fmt.Fprintf(w, "==> Updated %s (%s)\n", path, date.Format("2006-01-02"))
	return nil
}

// monthFile returns the log of the date's month and its content.
// The month's log is the one named <YYYY-MM>-*-log.md, whatever its day: the
// first business day names it. When there is none, it is named after date and
// starts with the previous log's head.
func monthFile(dir string, date time.Time) (string, string, error) {
	year := filepath.Join(dir, date.Format("2006"))
	found, err := filepath.Glob(filepath.Join(year, date.Format("2006-01")+"-??-log.md"))
	if err != nil {
		return "", "", err
	}
	if len(found) > 0 {
		slices.Sort(found)
		data, err := os.ReadFile(found[0])
		return found[0], string(data), err
	}
	path := filepath.Join(year, date.Format("2006-01-02")+"-log.md")
	prev, err := previousLog(dir, filepath.Base(path))
	if err != nil {
		return "", "", err
	}
	if prev == "" {
		return path, "# Log\n\n", nil
	}
	data, err := os.ReadFile(prev)
	if err != nil {
		return "", "", err
	}
	return path, NewMonthContent(string(data)), nil
}

func previousLog(dir, name string) (string, error) {
	all, err := filepath.Glob(filepath.Join(dir, "*", "*-log.md"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	prev := ""
	for _, p := range all {
		b := filepath.Base(p)
		if b < name && b > filepath.Base(prev) {
			prev = p
		}
	}
	return prev, nil
}
