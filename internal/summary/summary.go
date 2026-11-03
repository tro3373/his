package summary

import (
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type BaseLog struct {
	Date  string
	Sec   int64
	Tag   string
	Title string
}

type TimeLog struct {
	BaseLog
	Start int64 // unix sec
	End   int64 // unix sec
}

type TagSummaryLog struct {
	BaseLog
}

type TagTitleSummaryLog struct {
	BaseLog
}

type LogBaser interface {
	GetBaseLog() BaseLog
	IsTarget(t *TimeLog) bool
}

type Result struct {
	TagSummaryLogs      []*TagSummaryLog
	TagTitleSummaryLogs []*TagTitleSummaryLog
}

func Analyze(filePathPattern string, maxLoadFile int32) (*Result, error) {
	files, err := findRecentryFiles(filePathPattern, maxLoadFile)
	if err != nil {
		return nil, err
	}
	logs := parseFiles(files)
	return NewResult(logs), nil
}

func findRecentryFiles(filePathPattern string, maxLoadFile int32) ([]string, error) {
	files, err := filepath.Glob(filePathPattern)
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i] > files[j]
	})
	i := int64(math.Min(float64(len(files)), float64(maxLoadFile)))
	return files[:i], nil
}

func parseFiles(files []string) []*TimeLog {

	var logs = []*TimeLog{}
	for _, file := range files {
		timeLogs, err := parseFile(file)
		if err != nil {
			// return nil, err
			slog.Error("Failed to parseFile", "file", file, "err", err)
			continue
		}
		logs = append(logs, timeLogs...)
	}
	sort.Slice(logs, func(i, j int) bool {
		id := logs[i].Date
		jd := logs[j].Date
		if id != jd {
			return logs[i].Date > logs[j].Date
		}
		is := logs[i].Start
		js := logs[j].Start
		return is > js
	})
	return logs
}

func parseFile(file string) ([]*TimeLog, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return parseLines(strings.Split(string(data), "\n")), nil
}

// regHeading matches a time heading: "## 20250114_093000 ..." or "### 20250114_093000 ...".
var regHeading = regexp.MustCompile(`^#{2,3} \d{8}_\d{6}`)

// regHours matches an hours token such as "4.5h" placed right after a tag.
var regHours = regexp.MustCompile(`^\d+(\.\d+)?h$`)

// parseLines turns the time headings of a log into time logs.
// A heading that carries "TAG Nh" pairs counts those hours as they are.
// A heading without hours lasts until the next heading.
// A heading with neither tag nor title (lunch, end of day) only ends the previous one.
func parseLines(lines []string) []*TimeLog {
	timeLogs := []*TimeLog{}
	var open *TimeLog
	for _, line := range lines {
		if !regHeading.MatchString(line) {
			continue
		}
		logs, err := newTimeLogs(line)
		if err != nil {
			slog.Error("Failed to parse heading", "line", line, "err", err)
			continue
		}
		if open != nil {
			open.Fix(logs[0])
			open = nil
		}
		if logs[0].Sec == 0 {
			open = logs[0]
		}
		timeLogs = append(timeLogs, logs...)
	}
	validTimeLogs := []*TimeLog{}
	for _, l := range timeLogs {
		if !l.Valid() {
			continue
		}
		validTimeLogs = append(validTimeLogs, l)
	}
	return validTimeLogs
}

// newTimeLogs parses one heading. It returns one log per "TAG Nh" pair, or a single
// open-ended log when the heading has no hours.
func newTimeLogs(line string) ([]*TimeLog, error) {
	parts := strings.Fields(line)
	startStr := parts[1]
	startTime, err := time.Parse("20060102_150405", startStr)
	if err != nil {
		return nil, err
	}
	base := TimeLog{Start: startTime.Unix()}
	base.Date = fmt.Sprintf("%s-%s-%s", startStr[0:4], startStr[4:6], startStr[6:8])
	rest := parts[2:]
	var hourly []*TimeLog
	for len(rest) >= 2 && regHours.MatchString(rest[1]) {
		hours, err := strconv.ParseFloat(strings.TrimSuffix(rest[1], "h"), 64)
		if err != nil {
			return nil, err
		}
		l := base
		l.Tag = rest[0]
		l.Sec = int64(hours * 3600)
		l.End = l.Start + l.Sec
		hourly = append(hourly, &l)
		rest = rest[2:]
	}
	title := strings.Join(rest, " ")
	if len(hourly) > 0 {
		for _, l := range hourly {
			l.Title = title
		}
		return hourly, nil
	}
	if len(rest) > 0 {
		base.Tag = rest[0]
		base.Title = strings.Join(rest[1:], " ")
	}
	return []*TimeLog{&base}, nil
}

func (t *TimeLog) Fix(tl *TimeLog) {
	t.End = tl.Start
	t.Sec = t.End - t.Start
}

func (t *TimeLog) Valid() bool {
	if t.End == 0 {
		return false
	}
	if t.Sec == 0 {
		return false
	}
	if len(t.Date) == 0 {
		return false
	}
	if len(t.Tag) == 0 && len(t.Title) == 0 {
		return false
	}
	return true
}

func NewTagSummaryLog(tl *TimeLog) *TagSummaryLog {
	bl := BaseLog{
		Date:  tl.Date,
		Sec:   0,
		Tag:   tl.Tag,
		Title: tl.Title,
	}
	return &TagSummaryLog{
		BaseLog: bl,
	}
}

func NewTagTitleSummaryLog(tl *TimeLog) *TagTitleSummaryLog {
	bl := BaseLog{
		Date:  tl.Date,
		Sec:   0,
		Tag:   tl.Tag,
		Title: tl.Title,
	}
	return &TagTitleSummaryLog{
		BaseLog: bl,
	}
}

func NewResult(timeLogs []*TimeLog) *Result {
	var tagSummaries []*TagSummaryLog
	var tagTitleSummaries []*TagTitleSummaryLog
	findOrNewTagSummaryLog := func(tl *TimeLog) *TagSummaryLog {
		for _, sum := range tagSummaries {
			if sum.IsTarget(tl) {
				return sum
			}
		}
		sum := NewTagSummaryLog(tl)
		tagSummaries = append(tagSummaries, sum)
		return sum
	}
	findOrNewTagTitleSummaryLog := func(tl *TimeLog) *TagTitleSummaryLog {
		for _, sum := range tagTitleSummaries {
			if sum.IsTarget(tl) {
				return sum
			}
		}
		sum := NewTagTitleSummaryLog(tl)
		tagTitleSummaries = append(tagTitleSummaries, sum)
		return sum
	}
	for _, tl := range timeLogs {
		// fmt.Printf("Result: %+v\n", tl)
		tag := findOrNewTagSummaryLog(tl)
		tag.Append(tl)

		tagTitle := findOrNewTagTitleSummaryLog(tl)
		tagTitle.Append(tl)
	}
	return &Result{
		TagSummaryLogs:      tagSummaries,
		TagTitleSummaryLogs: tagTitleSummaries,
	}
}

// func findOrNew[T LogBaser](list []LogBaser, tl *TimeLog, newer func(tl *TimeLog) LogBaser) LogBaser {
// 	for _, lb := range list {
// 		if lb.IsTarget(tl) {
// 			return lb
// 		}
// 	}
// 	lb := newer(tl)
// 	list = append(list, lb)
// 	return lb
// }

func (r *Result) PrintTagResult(tag string, maxPrintDateCount int) {
	lbs := []LogBaser{}
	for _, l := range r.TagSummaryLogs {
		lbs = append(lbs, l)
	}
	r.printResultHandler(lbs, tag, maxPrintDateCount)
	// r.printResultHandler(r.TagSummaryLogs, tag, maxPrintDateCount)
}

func (r *Result) PrintTagTitleResult(tag string, maxPrintDateCount int) {
	lbs := []LogBaser{}
	for _, l := range r.TagTitleSummaryLogs {
		lbs = append(lbs, l)
	}
	r.printResultHandler(lbs, tag, maxPrintDateCount)
}

func (r *Result) printResultHandler(lbs []LogBaser, tag string, maxPrintDateCount int) {
	count := 0
	prevDate := ""
	for _, lb := range lbs {
		b := lb.GetBaseLog()
		if prevDate != b.Date {
			count++
			if count > maxPrintDateCount {
				return
			}
			prevDate = b.Date
		}
		if len(tag) != 0 && tag != b.Tag {
			continue
		}
		fmt.Printf("%s\n", lb)
	}
}

func (ts *TagSummaryLog) GetBaseLog() BaseLog {
	return ts.BaseLog
}

func (ts *TagSummaryLog) IsTarget(t *TimeLog) bool {
	return ts.Date == t.Date && ts.Tag == t.Tag
}

func (ts *TagSummaryLog) String() string {
	return fmt.Sprintf(
		"%s\t%s\t%s",
		ts.Date,
		ts.HumanTimeString(),
		ts.Tag,
	)
}

func (tts *TagTitleSummaryLog) GetBaseLog() BaseLog {
	return tts.BaseLog
}

func (tts *TagTitleSummaryLog) IsTarget(t *TimeLog) bool {
	return tts.Date == t.Date && tts.Tag == t.Tag && tts.Title == t.Title
}

func (tts *TagTitleSummaryLog) String() string {
	return fmt.Sprintf(
		"%s\t%s\t%s\t%s",
		tts.Date,
		tts.HumanTimeString(),
		tts.Tag,
		tts.Title,
	)
}

func (b *BaseLog) Append(t *TimeLog) {
	b.Sec = b.Sec + t.Sec
}

func (b *BaseLog) HumanTimeString() string {
	sec := b.Sec
	return fmt.Sprintf("%02dh%02dm(+%02ds)",
		sec/60/60,
		sec/60%60,
		sec%60,
	)
}
