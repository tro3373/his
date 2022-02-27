package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

func do() error {
	file, err := findLatestMd()
	if err != nil {
		return err
	}
	parseFile(file)
	return nil
}

func findLatestMd() (string, error) {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	pattern := fmt.Sprintf("%s/works/00_memos/*月.md", userHomeDir)
	files, err := filepath.Glob(pattern)
	sort.Slice(files, func(i, j int) bool {
		return files[i] > files[j]
	})
	for _, f := range files {
		// log.Printf("f:%s\n", f)
		return f, nil
	}
	return "", errors.New(fmt.Sprintf("Error: %s", "No such md exist."))
}

func parseFile(file string) error {
	fp, err := os.Open(file)
	if err != nil {
		return err
	}
	defer fp.Close()

	log.Printf("> %s\n", file)
	// log.Printf("%d", time.Now().Unix())

	var dateLogs = []DateLog{}
	var dateLog DateLog

	scanner := bufio.NewScanner(fp)
	for scanner.Scan() {
		line := scanner.Text()
		regStartDate := regexp.MustCompile(`^[#-] \d{4}`)
		if !regStartDate.MatchString(line) && !regStartDate.MatchString(line) {
			continue
		}
		log.Println(line)
		if strings.HasPrefix(line, "# ") {
			dateLog = NewDateLog(line)
			dateLogs = append(dateLogs, dateLog)
			continue
		}
		timeLog := NewTimeLog(line)
		dateLog.TimeLogs = append(dateLog.TimeLogs, timeLog)
	}
	log.Printf("%#+v", dateLogs)
	log.Printf("%#+v", dateLog)

	return nil
}

type DateLog struct {
	Date     string
	TimeLogs []TimeLog
}

func NewDateLog(line string) DateLog {
	//# 2022-02-21
	date := line[2:]
	return DateLog{
		date,
		[]TimeLog{},
	}
}

type TimeLog struct {
	Start int64
	Tag   string
	Title string
}

func NewTimeLog(line string) TimeLog {
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
	return TimeLog{
		startTime.Unix(),
		tag,
		title,
	}
}
