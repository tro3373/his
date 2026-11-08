package gen

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/tro3373/his/internal/timelog"
)

// Link is a task to list under a segment of the day.
type Link struct {
	Path    string // relative to the store root, as the log links it
	Title   string
	Done    bool
	Segment int // index into Day.Segments
}

// Line is the markdown list item of the link.
func (l Link) Line() string {
	mark := " "
	if l.Done {
		mark = "x"
	}
	return fmt.Sprintf("- [%s] [%s](%s)", mark, l.Title, l.Path)
}

func (l Link) target() string { return target(l.Path) }

// target is how a list item links the task at path.
func target(path string) string { return "](" + path + ")" }

// RenderDay renders the time headings of the day with each link under its segment.
func RenderDay(d Day, links []Link) []string {
	var out []string
	for i, s := range d.Segments {
		out = append(out, heading(s))
		for _, l := range links {
			if l.Segment == i {
				out = append(out, l.Line())
			}
		}
	}
	return append(out, timelog.Heading{Start: d.End}.String())
}

func heading(s Segment) string {
	h := timelog.Heading{Start: s.Start, Text: s.Title}
	for _, th := range s.Hours {
		h.Hours = append(h.Hours, timelog.TagHours{Tag: th.Tag, Hours: float64(th.Quarters) / 4})
	}
	return h.String()
}

var weekdays = [...]string{"日", "月", "火", "水", "木", "金", "土"}

// DayHeader is the heading that opens the day: "## 2026-10-30 (金)".
func DayHeader(date time.Time) string {
	return fmt.Sprintf("## %s (%s)", dayKey(date), weekdays[date.Weekday()])
}

// dayKey is the date a day header carries, the key findSection and regDayHeader match on.
func dayKey(date time.Time) string { return date.Format(time.DateOnly) }

var regDayHeader = regexp.MustCompile(`^## (\d{4}-\d{2}-\d{2})`)

// SettledUntil is when the settled part of date's section ends: what a run may
// not write before. The zero time means the section has no time heading.
//   - An end heading settles up to its own time.
//   - A heading with hours settles what those hours cover.
//   - Any other heading is open: it settles up to the next quarter hour.
func SettledUntil(content string, date time.Time) time.Time {
	ls := strings.Split(content, "\n")
	head, end, found := findSection(ls, dayKey(date))
	if !found {
		return time.Time{}
	}
	i, h, ok := latestHeading(ls[head+1:end], date.Location())
	if !ok {
		return time.Time{}
	}
	if timelog.IsEnd(ls[head+1 : end][i]) {
		return h.Start
	}
	if len(h.Hours) == 0 {
		return h.Start.Truncate(quarter).Add(quarter)
	}
	var hours float64
	for _, th := range h.Hours {
		hours += th.Hours
	}
	return ceilQuarter(h.Start.Add(time.Duration(hours * float64(time.Hour))))
}

// latestHeading finds the time heading with the latest time, and its index in section.
func latestHeading(section []string, loc *time.Location) (int, timelog.Heading, bool) {
	at, latest, found := 0, timelog.Heading{}, false
	for i, l := range section {
		h, err := timelog.Parse(l, loc)
		if err != nil || found && !h.Start.After(latest.Start) {
			continue
		}
		at, latest, found = i, h, true
	}
	return at, latest, found
}

// MergeDay puts the day into the log content.
// block is what the run has for the day after SettledUntil; nil when it has nothing.
//   - No section for the day: one is created in date order with block.
//   - A section with no time heading: block goes right under the day header, and
//     lines that link the same tasks move into it.
//   - A section with time headings: block is appended and the lines before it stay.
//     An end heading at the time block starts is replaced by block's first heading.
//
// done lists the paths of finished tasks: their links are checked anywhere in the
// section, the one change made to settled lines.
// block must start at SettledUntil or later: it is written as given, so a rerun
// changes nothing only because the next plan is empty.
func MergeDay(content string, date time.Time, block []string, links []Link, done []string) string {
	ls := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	if content == "" {
		ls = nil
	}
	key := dayKey(date)
	head, end, found := findSection(ls, key)
	if !found && len(block) == 0 {
		return content
	}
	if !found {
		return join(insertSection(ls, key, append([]string{DayHeader(date)}, block...)))
	}
	out := append([]string{}, ls[:head+1]...)
	out = append(out, mergeSection(checkDone(ls[head+1:end], done), date.Location(), block, links)...)
	return join(append(out, ls[end:]...))
}

func mergeSection(section []string, loc *time.Location, block []string, links []Link) []string {
	if len(block) == 0 {
		return section
	}
	last, latest, found := latestHeading(section, loc)
	if !found {
		return placeBlock(section, block, links)
	}
	block = withoutPresentLinks(block, section, links)
	first, err := timelog.Parse(block[0], loc)
	out := slices.Clone(section)
	if err == nil && timelog.IsEnd(section[last]) && first.Start.Equal(latest.Start) {
		out[last], block = block[0], block[1:]
	}
	at := len(out)
	for at > 0 && strings.TrimSpace(out[at-1]) == "" {
		at--
	}
	return slices.Insert(out, at, block...)
}

func join(ls []string) string { return strings.Join(ls, "\n") + "\n" }

// findSection returns the index of the day header and of the line after the section.
// A section ends at the next day header only: a pasted "# ..." shell line is content.
func findSection(ls []string, key string) (int, int, bool) {
	head := -1
	for i, l := range ls {
		if head < 0 {
			if m := regDayHeader.FindStringSubmatch(l); m != nil && m[1] == key {
				head = i
			}
			continue
		}
		if regDayHeader.MatchString(l) {
			return head, i, true
		}
	}
	return head, len(ls), head >= 0
}

// insertSection places section before the first later day, or at the end.
func insertSection(ls []string, key string, section []string) []string {
	at := len(ls)
	for i, l := range ls {
		if m := regDayHeader.FindStringSubmatch(l); m != nil && m[1] > key {
			at = i
			break
		}
	}
	var out []string
	out = append(out, ls[:at]...)
	if at > 0 && strings.TrimSpace(ls[at-1]) != "" {
		out = append(out, "")
	}
	out = append(out, section...)
	if at < len(ls) {
		out = append(out, "")
	}
	return append(out, ls[at:]...)
}

func hasTimeHeading(section []string) bool {
	for _, l := range section {
		if timelog.IsHeading(l) {
			return true
		}
	}
	return false
}

// placeBlock writes block at the top of a section without time headings.
// A link the section already has keeps its line, hand-edited title and child
// items included, and moves to its place in block; only its checkbox is updated.
func placeBlock(section, block []string, links []Link) []string {
	rest := slices.Clone(section)
	moved := map[string][]string{}
	for _, k := range links {
		i := indexContaining(rest, k.target())
		if i < 0 {
			continue
		}
		item := takeItem(rest, i, k.Done)
		moved[k.Line()] = item
		rest = slices.Delete(rest, i, i+len(item))
	}
	var out []string
	for _, l := range block {
		if item, ok := moved[l]; ok {
			out = append(out, item...)
			continue
		}
		out = append(out, l)
	}
	return append(out, rest...)
}

// withoutPresentLinks drops from block the link lines the section already has.
func withoutPresentLinks(block, section []string, links []Link) []string {
	present := map[string]bool{}
	for _, k := range links {
		present[k.Line()] = indexContaining(section, k.target()) >= 0
	}
	return slices.DeleteFunc(slices.Clone(block), func(l string) bool { return present[l] })
}

// takeItem copies the list item at i with its child lines, checked when done.
func takeItem(ls []string, i int, done bool) []string {
	item := append([]string{}, ls[i:i+1+childLines(ls[i+1:])]...)
	if done {
		item[0] = checked(item[0])
	}
	return item
}

// childLines counts the indented lines that belong to the list item before ls.
func childLines(ls []string) int {
	n := 0
	for n < len(ls) && ls[n] != "" && (ls[n][0] == ' ' || ls[n][0] == '\t') {
		n++
	}
	return n
}

func checked(line string) string { return strings.Replace(line, "- [ ] ", "- [x] ", 1) }

// checkDone checks the links of the finished tasks at paths.
func checkDone(section, paths []string) []string {
	out := slices.Clone(section)
	for i, l := range out {
		if !strings.Contains(l, "- [ ] ") {
			continue
		}
		if slices.ContainsFunc(paths, func(p string) bool { return strings.Contains(l, target(p)) }) {
			out[i] = checked(l)
		}
	}
	return out
}

func indexContaining(ls []string, s string) int {
	for i, l := range ls {
		if strings.Contains(l, s) {
			return i
		}
	}
	return -1
}

// NewMonthContent is the start of a new month's log: everything in the previous
// month's log before its first day.
func NewMonthContent(prev string) string {
	ls := strings.Split(prev, "\n")
	for i, l := range ls {
		if regDayHeader.MatchString(l) {
			return join(ls[:i])
		}
	}
	return prev
}
