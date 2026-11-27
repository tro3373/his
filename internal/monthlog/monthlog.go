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
	found, err := glob(filepath.Join(dir, month.Format(yearLayout), month.Format(monthLayout)+"-??"+nameSuffix))
	if err != nil || len(found) == 0 {
		return "", false, err
	}
	return found[0], true, nil
}

// Before is the newest month log under dir named before date's day, or "" when there is none.
func Before(dir string, date time.Time) (string, error) {
	all, err := List(dir)
	if err != nil {
		return "", err
	}
	limit := PathFor(dir, date)
	prev := ""
	for _, p := range all {
		if Compare(p, limit) < 0 {
			prev = p
		}
	}
	return prev, nil
}

// List is every month log under dir, oldest month first.
// A dir that does not exist holds none.
func List(dir string) ([]string, error) {
	return glob(filepath.Join(dir, "*", "*"+nameSuffix))
}

// Compare orders month logs by the month they hold, oldest first, whichever dir they lie under.
// Month logs of the same file name compare equal.
func Compare(a, b string) int {
	return cmp.Compare(filepath.Base(a), filepath.Base(b))
}

func glob(pattern string) ([]string, error) {
	found, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	found = slices.DeleteFunc(found, func(p string) bool { return !isMonthLog(p) })
	slices.SortStableFunc(found, Compare)
	return found, nil
}

func isMonthLog(path string) bool {
	date, ok := strings.CutSuffix(filepath.Base(path), nameSuffix)
	if !ok {
		return false
	}
	_, err := time.Parse(dayLayout, date)
	return err == nil
}
