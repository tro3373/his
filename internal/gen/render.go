package gen

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
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

func (l Link) target() string { return "](" + l.Path + ")" }

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
	return fmt.Sprintf("## %s (%s)", date.Format("2006-01-02"), weekdays[date.Weekday()])
}

var regDayHeader = regexp.MustCompile(`^## (\d{4}-\d{2}-\d{2})`)

var (
	regBlockBegin = regexp.MustCompile(`^<!-- his gen ([0-9a-f]{16}) -->$`)
	blockEnd      = "<!-- his gen end -->"
)

func blockHash(lines []string) string {
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])[:16]
}

// markBlock wraps block in the marker lines that tell a later run what gen wrote.
func markBlock(block []string) []string {
	out := []string{"<!-- his gen " + blockHash(block) + " -->"}
	out = append(out, block...)
	return append(out, blockEnd)
}

// ownedRegion finds the gen block of the section that nobody edited:
// its begin marker, a later end marker, and lines between that still hash to the begin marker's hash.
// It returns the indexes of the two markers.
func ownedRegion(section []string) (begin, end int, ok bool) {
	for i, l := range section {
		m := regBlockBegin.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		j := indexOf(section[i+1:], blockEnd)
		if j < 0 {
			return 0, 0, false
		}
		if blockHash(section[i+1:i+1+j]) == m[1] {
			return i, i + 1 + j, true
		}
	}
	return 0, 0, false
}

func indexOf(ls []string, s string) int {
	for i, l := range ls {
		if l == s {
			return i
		}
	}
	return -1
}

// MergeDay puts the day into the log content.
// block is written between marker lines that record its hash.
//   - No section for the day: one is created in date order with the marked block.
//   - A section holding a gen block nobody edited (its lines still match the hash
//     in its begin marker): the block is replaced in place with the new one.
//   - A section with time headings but no such block (written by hand, or edited
//     after gen): the headings stay, missing links are appended and finished tasks are checked.
//   - A section without time headings: the marked block goes right under the day header.
//
// Whenever a block is written, lines in the rest of the section that link the same
// tasks are moved into it; other lines stay outside it.
// Running it again with the same input changes nothing.
func MergeDay(content string, date time.Time, block []string, links []Link) string {
	ls := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	if content == "" {
		ls = nil
	}
	key := date.Format("2006-01-02")
	marked := markBlock(block)
	head, end, found := findSection(ls, key)
	if !found {
		return join(insertSection(ls, key, append([]string{DayHeader(date)}, marked...)))
	}
	out := append([]string{}, ls[:head+1]...)
	out = append(out, mergeSection(ls[head+1:end], marked, links)...)
	return join(append(out, ls[end:]...))
}

func mergeSection(section, marked []string, links []Link) []string {
	if begin, end, ok := ownedRegion(section); ok {
		rest := append(append([]string{}, section[:begin]...), section[end+1:]...)
		return mergeBlock(rest, begin, marked, links)
	}
	if hasTimeHeading(section) {
		return syncLinks(section, links)
	}
	return mergeBlock(section, 0, marked, links)
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

// mergeBlock inserts block into the section at index at and keeps the rest around it.
// A link the section already has keeps its line, hand-edited title and child
// items included, and moves to its place in block; only its checkbox is updated.
func mergeBlock(section []string, at int, block []string, links []Link) []string {
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
		if i < at {
			at -= min(n, at-i)
		}
	}
	var out []string
	for _, l := range block {
		if item, ok := moved[l]; ok {
			out = append(out, item...)
			continue
		}
		out = append(out, l)
	}
	res := append([]string{}, rest[:at]...)
	res = append(res, out...)
	return append(res, rest[at:]...)
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
// A closing gen end marker is skipped like those blank lines.
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
	if at > 0 && out[at-1] == blockEnd {
		at--
	}
	for at > 0 && strings.TrimSpace(out[at-1]) == "" {
		at--
	}
	if at > 0 && timelog.IsEnd(out[at-1]) {
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
