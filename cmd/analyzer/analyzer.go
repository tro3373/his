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

func sampleCaller() error {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	pattern := fmt.Sprintf("%s/works/00_memos/*月.md", userHomeDir)
	result, err := Analyze(pattern, 2)
	fmt.Printf("==> %#+v", result)
	return err
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
		if prevTimeLog != nil {
			prevTimeLog.Fix(timeLog)
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

type TimeLog struct {
	Date    string
	Start   int64 // unix sec
	End     int64 // unix sec
	Summary int64
	Tag     string
	Title   string
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
	sec := startTime.Unix()
	tl.Start = sec
	tl.Date = time.Unix(sec, 0).Format("2006-01-02")

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
	t.Summary = t.End - t.Start
}

func (t *TimeLog) Valid() bool {
	if t.End == 0 {
		return false
	}
	if t.Summary == 0 {
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

type Result struct {
	SummaryLogs []*SummaryLog
}

func NewResult(timeLogs []*TimeLog) (*Result, error) {
	var sls []*SummaryLog
	findOrNewSummaryLog := func(tl *TimeLog) *SummaryLog {
		for _, sl := range sls {
			if sl.isTarget(tl) {
				return sl
			}
		}
		sl := &SummaryLog{}
		sls = append(sls, sl)
		return sl
	}
	for _, tl := range timeLogs {
		sl := findOrNewSummaryLog(tl)
		sl.Append(tl)
	}
	return &Result{sls}, nil
}

func (r *Result) PrintTagResult(maxPrintDateCount int) {
	r.printResultHandler(maxPrintDateCount, func(s *SummaryLog) {
		s.PrintTagFormatSummary()
	})
}

func (r *Result) PrintTagTitleResult(tag string, maxPrintDateCount int) {
	r.printResultHandler(maxPrintDateCount, func(s *SummaryLog) {
		if len(tag) != 0 && tag != s.Tag {
			return
		}
		s.PrintTagTitleFormatSummary()
	})
}

func (r *Result) printResultHandler(maxPrintDateCount int, fn func(s *SummaryLog)) {
	count := 0
	prevDate := ""
	for _, s := range r.SummaryLogs {
		if prevDate != s.Date {
			count++
			if count > maxPrintDateCount {
				return
			}
			prevDate = s.Date
		}
		fn(s)
	}
}

type SummaryLog struct {
	Date  string
	Sec   int64
	Tag   string
	Title string
}

func (s *SummaryLog) isTarget(t *TimeLog) bool {
	return s.Date == t.Date && s.Tag == t.Tag && s.Title == t.Title
}

func (s *SummaryLog) Append(t *TimeLog) {
	s.Sec = s.Sec + t.Summary
}

func (s *SummaryLog) HumanTimeString() string {
	sec := s.Sec
	return fmt.Sprintf("%02dh%02dm(+%02ds)",
		sec/60/60,
		sec/60%60,
		sec%60,
	)
}

func (s *SummaryLog) PrintTagTitleFormatSummary() {
	fmt.Printf(
		"%s\t%s\t%s\t%s\n",
		s.Date,
		s.HumanTimeString(),
		s.Tag,
		s.Title,
	)
}

func (s *SummaryLog) PrintTagFormatSummary() {
	fmt.Printf(
		"%s\t%s\t%s\n",
		s.Date,
		s.HumanTimeString(),
		s.Tag,
	)
}
