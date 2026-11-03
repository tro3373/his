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
	StoreRoot string      `mapstructure:"store_root"` // holds tasks/; links are relative to it
	RecDir    string      `mapstructure:"rec_dir"`    // rec's output directory
	Sessions  []string    `mapstructure:"sessions"`   // <CLAUDE_CONFIG_DIR>/projects of every config dir
	Lunch     Lunch       `mapstructure:"lunch"`
	Logs      []LogConfig `mapstructure:"logs"`
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

// catchUpDays is how far back a run without a date looks.
// Claude Code deletes session logs after 30 days (cleanupPeriodDays).
const catchUpDays = 30

// meetingTagWindow is how far around a call the session activity decides its project.
const meetingTagWindow = 30 * time.Minute

// Run writes the days of every log; datesOf tells which.
// A day's headings are written once, so today written half way keeps its
// headings: the rest of the day only adds task links.
func Run(cfg Config, now, target time.Time, w io.Writer) error {
	loc := now.Location()
	dates := datesOf(midnight(now), target)
	since := dates[0]

	tasks, err := ReadTasks(cfg.StoreRoot, loc)
	if err != nil {
		return err
	}
	recs, err := ReadRecordings(cfg.RecDir, since)
	if err != nil {
		return err
	}
	resolver := NewTagResolver()
	acts := make([][]Activity, len(cfg.Logs))
	for _, dir := range cfg.Sessions {
		got, err := ReadActivities(dir, since, resolver)
		if err != nil {
			return err
		}
		for _, a := range got {
			if i := logOf(cfg.Logs, a.Cwd); i >= 0 {
				acts[i] = append(acts[i], a)
			}
		}
	}
	owner := taskOwners(cfg.Logs, acts)

	for _, d := range dates {
		for i, l := range cfg.Logs {
			dayActs := between(acts[i], d, d.AddDate(0, 0, 1))
			meets := meetingsOf(recs, l.Apps, d, dayActs)
			day, ok := PlanDay(d, cfg.Lunch, dayActs, meets)
			if !ok {
				continue
			}
			links := linksOf(tasks, day, dayActs, d, func(t Task) bool { return owner(t.Repo) == i })
			block := RenderDay(day, links)
			if err := writeDay(l.Dir, d, block, links, w); err != nil {
				return err
			}
		}
	}
	return nil
}

// datesOf is the days a run writes.
// A zero target is the last 30 days up to yesterday, as cron runs it.
// Today is those days and today, so a run at the end of the day leaves no gap.
// Any other target is that day only.
func datesOf(today, target time.Time) []time.Time {
	if !target.IsZero() && !midnight(target).Equal(today) {
		return []time.Time{midnight(target)}
	}
	var dates []time.Time
	for i := catchUpDays; i >= 1; i-- {
		dates = append(dates, today.AddDate(0, 0, -i))
	}
	if target.IsZero() {
		return dates
	}
	return append(dates, today)
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

// linksOf lists the day's tasks: the ones whose branch had session activity that
// day, and the ones split that day that belong to this log.
// Each goes under the segment where its branch was most active, else under the
// segment it was split in, else under the last segment that is not lunch.
func linksOf(tasks []Task, day Day, dayActs []Activity, date time.Time, ownTask func(Task) bool) []Link {
	next := date.AddDate(0, 0, 1)
	var picked []Task
	for _, t := range tasks {
		worked := t.Branch != "" && slices.ContainsFunc(dayActs, func(a Activity) bool {
			return a.Repo == t.Repo && a.Branch == t.Branch
		})
		created := within(t.Created, date, next) && ownTask(t)
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
		if a.Repo != t.Repo || a.Branch != t.Branch {
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

func writeDay(dir string, date time.Time, block []string, links []Link, w io.Writer) error {
	path, content, err := monthFile(dir, date)
	if err != nil {
		return err
	}
	merged := MergeDay(content, date, block, links)
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
