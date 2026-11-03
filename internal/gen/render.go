package gen

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
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

func (l Link) target() string { return "](" + l.Path + ")" }

const headingTime = "20060102_150405"

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
	return append(out, "### "+d.End.Format(headingTime))
}

func heading(s Segment) string {
	parts := []string{"###", s.Start.Format(headingTime)}
	for _, h := range s.Hours {
		parts = append(parts, h.Tag, strconv.FormatFloat(float64(h.Quarters)/4, 'f', -1, 64)+"h")
	}
	if s.Title != "" {
		parts = append(parts, s.Title)
	}
	return strings.Join(parts, " ")
}

var weekdays = [...]string{"日", "月", "火", "水", "木", "金", "土"}

// DayHeader is the heading that opens the day: "## 2026-10-30 (金)".
func DayHeader(date time.Time) string {
	return fmt.Sprintf("## %s (%s)", date.Format("2006-01-02"), weekdays[date.Weekday()])
}

var (
	regDayHeader   = regexp.MustCompile(`^## (\d{4}-\d{2}-\d{2})`)
	regTimeHeading = regexp.MustCompile(`^#{2,3} \d{8}_\d{6}`)
	// regEndHeading is a time heading with nothing after it: lunch or the end of the day.
	regEndHeading = regexp.MustCompile(`^#{2,3} \d{8}_\d{6}\s*$`)
)

// MergeDay puts the day into the log content.
//   - No section for the day: one is created in date order with block.
//   - A section without time headings: block goes right under the day header.
//     Lines that link the same tasks are moved into block; other lines stay after it.
//   - A section with time headings was written already (by hand or by gen):
//     the headings stay, missing links are appended and finished tasks are checked.
//
// Running it again with the same input changes nothing.
func MergeDay(content string, date time.Time, block []string, links []Link) string {
	ls := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	if content == "" {
		ls = nil
	}
	key := date.Format("2006-01-02")
	head, end, found := findSection(ls, key)
	if !found {
		return join(insertSection(ls, key, append([]string{DayHeader(date)}, block...)))
	}
	out := append([]string{}, ls[:head+1]...)
	out = append(out, mergeSection(ls[head+1:end], block, links)...)
	return join(append(out, ls[end:]...))
}

func mergeSection(section, block []string, links []Link) []string {
	if hasTimeHeading(section) {
		return syncLinks(section, links)
	}
	return mergeBlock(section, block, links)
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
		if regTimeHeading.MatchString(l) {
			return true
		}
	}
	return false
}

// mergeBlock puts block first and keeps the rest of the section after it.
// A link the section already has keeps its line, hand-edited title and child
// items included, and moves to its place in block; only its checkbox is updated.
func mergeBlock(section, block []string, links []Link) []string {
	rest := append([]string{}, section...)
	moved := map[string][]string{}
	for _, k := range links {
		i := indexContaining(rest, k.target())
		if i < 0 {
			continue
		}
		n := 1 + childLines(rest[i+1:])
		item := append([]string{}, rest[i:i+n]...)
		if k.Done {
			item[0] = checked(item[0])
		}
		moved[k.Line()] = item
		rest = append(rest[:i], rest[i+n:]...)
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

// childLines counts the indented lines that belong to the list item before ls.
func childLines(ls []string) int {
	n := 0
	for n < len(ls) && ls[n] != "" && (ls[n][0] == ' ' || ls[n][0] == '\t') {
		n++
	}
	return n
}

func checked(line string) string { return strings.Replace(line, "- [ ] ", "- [x] ", 1) }

// syncLinks checks finished tasks and appends the links the section lacks.
// They go after the last content of the day: before the end-of-day heading when
// the section closes with one, else before the blank lines that close it.
func syncLinks(section []string, links []Link) []string {
	out := append([]string{}, section...)
	var missing []string
	for _, k := range links {
		i := indexContaining(out, k.target())
		if i < 0 {
			missing = append(missing, k.Line())
			continue
		}
		if k.Done {
			out[i] = checked(out[i])
		}
	}
	at := len(out)
	for at > 0 && strings.TrimSpace(out[at-1]) == "" {
		at--
	}
	if at > 0 && regEndHeading.MatchString(out[at-1]) {
		at--
	}
	for at > 0 && strings.TrimSpace(out[at-1]) == "" {
		at--
	}
	res := append([]string{}, out[:at]...)
	res = append(res, missing...)
	return append(res, out[at:]...)
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
