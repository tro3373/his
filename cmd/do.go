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

const MAX_LOAD_MD_FILES = 2
const MAX_SHOW_SUMMARY = 15

func do() error {
	files, err := findRecentryMds()
	if err != nil {
		return err
	}
	dateLogs, err := collectFromFiles(files)
	if err != nil {
		return err
	}

	count := -1
	for _, dl := range dateLogs {
		count++
		if count > MAX_SHOW_SUMMARY {
			break
		}
		// log.Printf("Date: %s\n", dl.Date)
		m := make(map[string]int64)
		// factories = make(map[string]Factory)
		tag := ""
		prev := int64(0)
		for _, tl := range dl.TimeLogs {
			// log.Printf("Start:%#+v", tl.Start)
			// log.Printf("Tag:%#+v", tl.Tag)
			// log.Printf("Title:%#+v", tl.Title)
			if prev != 0 && tag != "" {
				summary := m[tag]
				summary += tl.Start - prev
				m[tag] = summary
			}
			prev = tl.Start
			tag = tl.Tag
		}
		// log.Printf("Summary:%#+v", m)
		for tag, sec := range m {
			// log.Printf("%s\t%s\t%.1fh\n", dl.Date, tag, float64((sec / 60 / 60)))
			fmt.Printf("%s\t%s\t%.2fh\n", dl.Date, tag, float64((sec / 60 / 60)))
		}
	}
	return nil
}

func findRecentryMds() ([]string, error) {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	pattern := fmt.Sprintf("%s/works/00_memos/*月.md", userHomeDir)
	files, err := filepath.Glob(pattern)
	sort.Slice(files, func(i, j int) bool {
		return files[i] > files[j]
	})
	i := int64(math.Min(float64(len(files)), MAX_LOAD_MD_FILES))
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
			continue
		}
		timeLog := NewTimeLog(line)
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
	Start int64
	Tag   string
	Title string
}

func NewTimeLog(line string) *TimeLog {
	//12345678901234567890123456789
	//- 20220221_100000 COM hoge
	start := line[2:17]
	startTime, _ := time.Parse("20060102_150405", start)
	var tag string
	var title string
	if len(line) > 18 {
		tag = strings.Split(line[18:], " ")[0]
		count := len(start) + len(tag) + 4
		title = line[count:]
	}
	return &TimeLog{
		startTime.Unix(),
		tag,
		title,
	}
}
