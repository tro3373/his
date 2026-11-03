// Package timelog is the format of the time headings in a work log, shared by
// the code that writes them and the code that reads them:
//
//	### 20261030_130000 ALPHA 1h GAMMA 0.5h Title
package timelog

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Layout is the time of a heading.
const Layout = "20060102_150405"

var (
	regHeading = regexp.MustCompile(`^#{2,3} \d{8}_\d{6}`)
	// regEnd is a heading with nothing after its time: lunch or the end of the day.
	regEnd   = regexp.MustCompile(`^#{2,3} \d{8}_\d{6}\s*$`)
	regHours = regexp.MustCompile(`^\d+(\.\d+)?h$`)
)

// ErrNotHeading is returned by Parse for a line that is not a time heading.
var ErrNotHeading = errors.New("not a time heading")

// TagHours is the time spent on one tag, written "ALPHA 1.5h".
type TagHours struct {
	Tag   string
	Hours float64
}

// Heading is one time heading.
type Heading struct {
	Start time.Time
	Hours []TagHours
	// Text is what follows the hours: the title.
	// A heading without hours is the older "TAG Title" form, kept whole here.
	Text string
}

// IsHeading reports whether line starts like a time heading ("## " or "### ").
func IsHeading(line string) bool { return regHeading.MatchString(line) }

// IsEnd reports whether line is a time heading with nothing after its time.
func IsEnd(line string) bool { return regEnd.MatchString(line) }

// Parse reads a time heading, its time in loc.
// Words are split on any run of spaces and Text joins them with one.
func Parse(line string, loc *time.Location) (Heading, error) {
	if !IsHeading(line) {
		return Heading{}, ErrNotHeading
	}
	fields := strings.Fields(line)
	start, err := time.ParseInLocation(Layout, fields[1], loc)
	if err != nil {
		return Heading{}, err
	}
	h := Heading{Start: start}
	rest := fields[2:]
	for len(rest) >= 2 && regHours.MatchString(rest[1]) {
		hours, err := strconv.ParseFloat(strings.TrimSuffix(rest[1], "h"), 64)
		if err != nil {
			return Heading{}, err
		}
		h.Hours = append(h.Hours, TagHours{Tag: rest[0], Hours: hours})
		rest = rest[2:]
	}
	h.Text = strings.Join(rest, " ")
	return h, nil
}

// String writes the heading at level 3.
func (h Heading) String() string {
	parts := []string{"###", h.Start.Format(Layout)}
	for _, th := range h.Hours {
		parts = append(parts, th.Tag, strconv.FormatFloat(th.Hours, 'f', -1, 64)+"h")
	}
	if h.Text != "" {
		parts = append(parts, h.Text)
	}
	return strings.Join(parts, " ")
}
