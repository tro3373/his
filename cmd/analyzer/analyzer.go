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
	result := NewResult(logs)
	// fmt.Println("collectDateLogs:", dateLogs, err)
	return result, nil
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
		return logs[i].Date > logs[j].Date
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
	TimeLogs []*TimeLog
}

func NewResult(logs []*TimeLog) *Result {
	return &Result{logs}
}

// type Summaries struct {
// 	List []summary
// }
//
// type summary struct {
// 	key string
// 	sum int64
// }
//
// func NewSummary() *Summaries {
// 	return &Summaries{}
// }
//
// func (s *Summaries) Add(tag, title string, sum int64) {
//
// }
//
// func SummaryTimeString(sec int64) string {
// 	return fmt.Sprintf("%02dh%02dm(+%02ds)",
// 		sec/60/60,
// 		sec/60%60,
// 		sec%60,
// 	)
// }
//
// func summaryTimeLog(timeLogs []*TimeLog, titleSummary bool) []*TimeLog {
// 	summaryMap := make(map[string]*TimeLog)
// 	for _, tl := range timeLogs {
// 		key := tl.Tag
// 		if titleSummary {
// 			key = fmt.Sprintf("%s-%s", tl.Tag, tl.Title)
// 		}
// 		summaryTl := summaryMap[key]
// 		if summaryTl == nil {
// 			summaryTl = &TimeLog{Tag: tl.Tag, Start: tl.Start, Summary: 0}
// 			if titleSummary {
// 				summaryTl.Title = tl.Title
// 			}
// 			summaryMap[key] = summaryTl
// 		}
// 		summaryTl.Summary += tl.Summary
// 	}
// 	summaries := []*TimeLog{}
// 	for _, value := range summaryMap {
// 		summaries = append(summaries, value)
// 	}
// 	return summaries
// }
