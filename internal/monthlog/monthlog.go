// Package monthlog is the only code that knows where month logs lie:
// <dir>/<YYYY>/<YYYY-MM-DD>-log.md, named after the first day of the month the file holds.
package monthlog

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	yearLayout  = "2006"
	monthLayout = "2006-01"
	dayLayout   = time.DateOnly
	nameSuffix  = "-log.md"
)

// PathFor is the path a new month log gets when date is the first day it holds.
func PathFor(dir string, date time.Time) string {
	return filepath.Join(dir, date.Format(yearLayout), date.Format(dayLayout)+nameSuffix)
}

// Find is the month log of month's month, whatever day of that month names it.
// When several exist, the one with the lowest name wins.
func Find(dir string, month time.Time) (string, bool, error) {
	pattern := filepath.Join(dir, month.Format(yearLayout), month.Format(monthLayout)+"-??"+nameSuffix)
	found, err := filepath.Glob(pattern)
	if err != nil {
		return "", false, err
	}
	found = slices.DeleteFunc(found, func(p string) bool { return !isMonthLog(p) })
	if len(found) == 0 {
		return "", false, nil
	}
	sortByName(found)
	return found[0], true, nil
}

// List is every month log under dir, sorted by file name ascending, oldest month first.
// A dir that does not exist holds none.
func List(dir string) ([]string, error) {
	all, err := filepath.Glob(filepath.Join(dir, "*", "*"+nameSuffix))
	if err != nil {
		return nil, err
	}
	all = slices.DeleteFunc(all, func(p string) bool { return !isMonthLog(p) })
	sortByName(all)
	return all, nil
}

func isMonthLog(path string) bool {
	date, ok := strings.CutSuffix(filepath.Base(path), nameSuffix)
	if !ok {
		return false
	}
	_, err := time.Parse(dayLayout, date)
	return err == nil
}

func sortByName(paths []string) {
	slices.SortFunc(paths, func(a, b string) int {
		return cmp.Or(cmp.Compare(filepath.Base(a), filepath.Base(b)), cmp.Compare(a, b))
	})
}
