package cmd

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

func getDateLogs(maxLoadMdFiles float64) ([]*DateLog, error) {
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
	// list := []string{}
	// for _, f := range files {
	// 	list = append(list, f)
	// 	// log.Printf("f:%s\n", f)
	// 	// return f, nil
	// 	if len(list) == 2 {
	// 		break
	// 	}
	// }
	// return list, nil
	// return nil, errors.New(fmt.Sprintf("Error: %s", "No such md exist."))
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

	scanner := bufio.NewScanner(fp)
	var prevTimeLog *TimeLog
	for scanner.Scan() {
		line := scanner.Text()
		regStartDate := regexp.MustCompile(`^[#-] \d{4}`)
		if !regStartDate.MatchString(line) && !regStartDate.MatchString(line) {
			continue
		}
		// log.Println(line)
		if strings.HasPrefix(line, "# ") {
			dateLog = NewDateLog(line)
			dateLogs = append(dateLogs, dateLog)
			prevTimeLog = nil
			continue
		}
		timeLog, err := NewTimeLog(line, prevTimeLog)
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
		tl.Summary = tl.Start - prev.Start
	}
	return &tl, nil
}

func (t *TimeLog) parse(line string) error {
	//12345678901234567890123456789
	//- 20220221_100000 COM hoge
	parts := strings.Split(line, " ")
	startStr := parts[1]
	timeFormat := "20060102_150405"
	startTime, err := time.Parse(timeFormat, startStr)
	if err != nil {
		return err
	}
	t.Start = startTime.Unix()
	t.Tag = parts[2]
	t.Title = strings.Join(parts[3:], " ")
	return nil
}

func (t *TimeLog) SummaryTimeString() string {
	sec := t.Summary
	return fmt.Sprintf("%3.0fm\t%02dh%02dm(+%02ds)",
		float64(sec/60),
		sec/60/60,
		sec/60%60,
		sec%60,
	)
}

// type TitleSummary struct {
// 	Tag     string
// 	Title   string
// 	Summary int64
// }
//
// func NewTimeLog(line string) (*TimeLog, error) {
// 	// timeLogPrefix := "- "
// 	// timeFormat := "20060102_150405"
// 	// posTimeS := len(timeLogPrefix)
// 	// posTimeE := posTimeS + len(timeFormat)
// 	// startStr := line[posTimeS:posTimeE] // YYYYMMDD_hhmmss
// 	// startTime, _ := time.Parse(timeFormat, startStr)
// 	startTime, err := paseTimeLog2Unix(line)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	var tag string
// 	var title string
// 	posTagS := posTimeE + 1
// 	if len(line) > posTagS {
// 		// log.Printf("> line:%s\n", line)
// 		// log.Printf("> line.len:%d\n", len(line))
// 		tag = strings.Split(line[posTagS:], " ")[0]
// 		// count := len(startStr) + len(tag) + 4
// 		posTitleS := posTagS + len(tag) + 1
// 		if len(line) > posTitleS {
// 			title = line[posTitleS:]
// 		}
// 	}
// 	return &TimeLog{
// 		tag,
// 		title,
// 		startTime,
// 	}
// }

// func paseTimeLog2Unix(line string) (int64, error) {
// 	//12345678901234567890123456789
// 	//- 20220221_100000 COM hoge
// 	timeLogPrefix := "- "
// 	timeFormat := "20060102_150405"
// 	posTimeS := len(timeLogPrefix)
// 	posTimeE := posTimeS + len(timeFormat)
// 	startStr := line[posTimeS:posTimeE] // YYYYMMDD_hhmmss
// 	startTime, err := time.Parse(timeFormat, startStr)
// 	if err != nil {
// 		return -1, err
// 	}
// 	return startTime.Unix(), nil
// }

func summaryTag(timeLogs []*TimeLog) map[string]int64 {
	m := make(map[string]int64)
	// factories = make(map[string]Factory)
	tag := ""
	prev := int64(0)
	for _, tl := range timeLogs {
		if prev != 0 && tag != "" {
			summary := m[tag]
			add := tl.Start - prev
			summary += add
			m[tag] = summary
			// log.Printf("  Tag:%4s Add:%5d Sum:%6d Sta:%d End:%d Title:%s",
			// 	tag, add, summary, prev, tl.Start, tl.Title)
		}
		prev = tl.Start
		tag = tl.Tag
	}
	return m
}
func dumpSummaryTag(d string, m map[string]int64) {
	for tag, sec := range m {
		fmt.Printf("%s\t%s\t%3.0fm\t%02dh%02dm(+%02ds)\n",
			d,
			tag,
			float64(sec/60),
			sec/60/60,
			sec/60%60,
			sec%60,
		)
	}
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
			summaryTl = &TimeLog{Tag: tl.Tag, Start: tl.Start, Summary: 0}
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
	// findSummary := func(summaries []*TitleSummary, tag, title string) *TitleSummary {
	// 	for _, summary := range summaries {
	// 		if summary.Tag == tag && summary.Title == title {
	// 			return summary
	// 		}
	// 	}
	// 	return nil
	// }
	// for _, timeLog := range timeLogs {
	// 	summary := findSummary(summaries, timeLog.Tag, timeLog.Title)
	// 	if summary == nil {
	// 		summary = &TitleSummary{}
	// 	}
	// 	summary.Tag = timeLog.Tag
	// 	summary.Title = timeLog.Title
	// 	summary.Summary += timeLog.Title
	// }
	// // sum := findSummary(summaries, "tag", "title")
	// return summaries
	// m := make(map[string]int64)
	// tag := ""
	// prev := int64(0)
	// for _, tl := range timeLogs {
	// 	if prev != 0 && tag != "" {
	// 		summary := m[tag]
	// 		add := tl.Start - prev
	// 		summary += add
	// 		m[tag] = summary
	// 		// log.Printf("  Tag:%4s Add:%5d Sum:%6d Sta:%d End:%d Title:%s",
	// 		// 	tag, add, summary, prev, tl.Start, tl.Title)
	// 	}
	// 	prev = tl.Start
	// 	tag = tl.Tag
	// }
	// return m
}
