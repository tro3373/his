package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

func collectDateLogs(maxLoadMdFiles float64) ([]*DateLog, error) {
	files, err := findRecentryMds(maxLoadMdFiles)
	if err != nil {
		return nil, err
	}
	return collectFromFiles(files)
}

func findRecentryMds(maxLoadMdFiles float64) ([]string, error) {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	pattern := fmt.Sprintf("%s/works/00_memos/*月.md", userHomeDir)
	files, err := filepath.Glob(pattern)
	sort.Slice(files, func(i, j int) bool {
		return files[i] > files[j]
	})
	i := int64(math.Min(float64(len(files)), maxLoadMdFiles))
	return files[:i], nil
}

func collectFromFiles(files []string) ([]*DateLog, error) {

	var allLogs = []*DateLog{}
	for _, file := range files {
		dateLogs, err := collectFromFile(file)
		if err != nil {
			return nil, err
		}
		allLogs = append(allLogs, dateLogs...)
	}
	sort.Slice(allLogs, func(i, j int) bool {
		return allLogs[i].Date > allLogs[j].Date
	})
	return allLogs, nil
}

func collectFromFile(file string) ([]*DateLog, error) {
	fp, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	// log.Printf("> %s\n", file)
	// log.Printf("%d", time.Now().Unix())

	var dateLogs = []*DateLog{}
	var dateLog *DateLog
	regStartDate := regexp.MustCompile(`^[#-] \d{4}`)

	scanner := bufio.NewScanner(fp)
	var prevTimeLog *TimeLog
	for scanner.Scan() {
		line := scanner.Text()
		if !regStartDate.MatchString(line) {
			continue
		}
		// log.Println("####", line)
		if strings.HasPrefix(line, "# ") {
			dateLog = NewDateLog(line)
			dateLogs = append(dateLogs, dateLog)
			prevTimeLog = nil
			continue
		}
		timeLog, err := NewTimeLog(line, prevTimeLog)
		// log.Println("######", timeLog.SummaryTimeString())
		if err != nil {
			return nil, err
		}
		prevTimeLog = timeLog
		dateLog.TimeLogs = append(dateLog.TimeLogs, timeLog)
	}
	return dateLogs, nil
}

type DateLog struct {
	Date     string
	TimeLogs []*TimeLog
}

func NewDateLog(line string) *DateLog {
	//# 2022-02-21
	date := line[2:]
	return &DateLog{
		date,
		[]*TimeLog{},
	}
}

type TimeLog struct {
	Date    string
	Tag     string
	Title   string
	Start   int64 // unix sec
	Summary int64
}

func NewTimeLog(line string, prev *TimeLog) (*TimeLog, error) {
	tl := TimeLog{}
	err := tl.parse(line)
	if err != nil {
		return &tl, err
	}
	if prev != nil {
		prev.Summary = tl.Start - prev.Start
	}
	return &tl, nil
}

func (t *TimeLog) parse(line string) error {
	// fmt.Printf("==> Parsing line: %s\n", line)
	//12345678901234567890123456789
	//- 20220221_100000 COM hoge
	parts := strings.Split(line, " ")
	startStr := parts[1]
	timeFormat := "20060102_150405"
	startTime, err := time.Parse(timeFormat, startStr)
	if err != nil {
		return err
	}
	sec := startTime.Unix()
	t.Start = sec
	t.Date = time.Unix(sec, 0).Format("2006-01-02")
	if len(parts) > 2 {
		t.Tag = parts[2]
		if len(parts) > 3 {
			t.Title = strings.Join(parts[3:], " ")
		}
	}
	return nil
}

func SummaryTimeString(sec int64) string {
	return fmt.Sprintf("%02dh%02dm(+%02ds)",
		sec/60/60,
		sec/60%60,
		sec%60,
	)
}

func summaryTimeLog(timeLogs []*TimeLog, titleSummary bool) []*TimeLog {
	summaryMap := make(map[string]*TimeLog)
	for _, tl := range timeLogs {
		key := tl.Tag
		if titleSummary {
			key = fmt.Sprintf("%s-%s", tl.Tag, tl.Title)
		}
		summaryTl := summaryMap[key]
		if summaryTl == nil {
			summaryTl = &TimeLog{Date: tl.Date, Tag: tl.Tag, Start: tl.Start, Summary: 0}
			if titleSummary {
				summaryTl.Title = tl.Title
			}
			summaryMap[key] = summaryTl
		}
		summaryTl.Summary += tl.Summary
	}
	summaries := []*TimeLog{}
	for _, value := range summaryMap {
		summaries = append(summaries, value)
	}
	return summaries
}

func deepcopy(src interface{}, dst interface{}) (err error) {
	b, err := json.Marshal(src)
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, dst)
	if err != nil {
		return err
	}
	return nil
}
