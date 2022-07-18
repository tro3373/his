package analyzer

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
	return NewResult(logs)
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
			fmt.Println("Failed to parseFile file:", file, err)
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
	fp, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	var timeLogs = []*TimeLog{}
	regStartDate := regexp.MustCompile(`^- \d{8}_\d{6}`)

	scanner := bufio.NewScanner(fp)
	var prevTimeLog *TimeLog
	for scanner.Scan() {
		line := scanner.Text()
		if !regStartDate.MatchString(line) {
			continue
		}
		timeLog, err := NewTimeLog(line)
		if err != nil {
			fmt.Println("Failed to NewTimeLog line:", line, err)
			continue
			// return nil, err
		}
		// fmt.Printf("NEW: %+v\n", timeLog)
		if prevTimeLog != nil {
			prevTimeLog.Fix(timeLog)
			// fmt.Printf("Fixed: %+v\n", prevTimeLog)
		}
		prevTimeLog = timeLog
		timeLogs = append(timeLogs, timeLog)
	}
	var validTimeLogs = []*TimeLog{}
	for _, l := range timeLogs {
		if !l.Valid() {
			continue
		}
		validTimeLogs = append(validTimeLogs, l)
	}
	return validTimeLogs, nil
}

func NewTimeLog(line string) (*TimeLog, error) {
	// fmt.Printf("==> Parsing line: %s\n", line)

	tl := &TimeLog{}
	//12345678901234567890123456789
	//- 20220221_100000 COM hoge
	parts := strings.Split(line, " ")
	startStr := parts[1]
	timeFormat := "20060102_150405"
	startTime, err := time.Parse(timeFormat, startStr)
	if err != nil {
		return tl, err
	}
	tl.Date = fmt.Sprintf("%s-%s-%s", startStr[0:4], startStr[4:6], startStr[6:8])
	sec := startTime.Unix()
	tl.Start = sec

	if len(parts) > 2 {
		tl.Tag = parts[2]
		if len(parts) > 3 {
			tl.Title = strings.Join(parts[3:], " ")
		}
	}
	return tl, nil
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

func NewResult(timeLogs []*TimeLog) (*Result, error) {
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
	}, nil
}

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
