package summary

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tro3373/his/internal/timelog"
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

// parseLines turns the time headings of a log into time logs.
// A heading that carries "TAG Nh" pairs counts those hours as they are.
// A heading without hours lasts until the next heading.
// A heading with neither tag nor title (lunch, end of day) only ends the previous one.
func parseLines(lines []string) []*TimeLog {
	timeLogs := []*TimeLog{}
	var open *TimeLog
	for _, line := range lines {
		// UTC keeps the time between two headings in wall-clock time: no DST jump.
		h, err := timelog.Parse(line, time.UTC)
		if errors.Is(err, timelog.ErrNotHeading) {
			continue
		}
		if err != nil {
			slog.Error("Failed to parse heading", "line", line, "err", err)
			continue
		}
		logs := newTimeLogs(h)
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

// newTimeLogs turns one heading into one log per "TAG Nh" pair, or into a single
// open-ended log when the heading has no hours.
func newTimeLogs(h timelog.Heading) []*TimeLog {
	base := TimeLog{Start: h.Start.Unix()}
	base.Date = h.Start.Format(time.DateOnly)
	if len(h.Hours) == 0 {
		base.Tag, base.Title, _ = strings.Cut(h.Text, " ")
		return []*TimeLog{&base}
	}
	logs := make([]*TimeLog, 0, len(h.Hours))
	for _, th := range h.Hours {
		l := base
		l.Tag = th.Tag
		l.Title = h.Text
		l.Sec = int64(th.Hours * 3600)
		l.End = l.Start + l.Sec
		logs = append(logs, &l)
	}
	return logs
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
