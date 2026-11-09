// Package activity owns the Activity log on disk: one file per day,
// one line per shell or nvim operation, never the command itself.
package activity

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	fileExt    = ".tsv"
	dateLayout = "20060102"
)

// Entry is one moment someone was working in Cwd.
type Entry struct {
	At     time.Time
	Cwd    string
	Branch string
}

// Append adds e to <dir>/YYYYMMDD.tsv, the date taken from e.At in its location.
// An Entry whose Cwd holds a tab or a newline is dropped without error:
// its line could not be parsed back.
func Append(dir string, e Entry) error {
	if strings.ContainsAny(e.Cwd, "\t\n") {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create %s: %w", dir, err)
	}
	path := filepath.Join(dir, e.At.Format(dateLayout)+fileExt)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", path, err)
	}
	line := fmt.Sprintf("%d\t%s\t%s\n", e.At.Unix(), e.Cwd, e.Branch)
	_, werr := f.WriteString(line)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return fmt.Errorf("failed to write %s: %w", path, werr)
	}
	return nil
}

// Read returns the entries at or after since, in since's location, oldest file first.
// A missing dir is no entries. Malformed lines are skipped.
func Read(dir string, since time.Time) ([]Entry, error) {
	files, err := dayFiles(dir, since.Location())
	if err != nil || files == nil {
		return nil, err
	}
	var out []Entry
	for _, f := range files {
		if f.date.Before(midnight(since)) {
			continue
		}
		data, err := os.ReadFile(f.path)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", f.path, err)
		}
		for line := range strings.SplitSeq(string(data), "\n") {
			e, ok := parseLine(line, since.Location())
			if ok && !e.At.Before(since) {
				out = append(out, e)
			}
		}
	}
	return out, nil
}

// Prune removes the day files dated before before's date.
// Files not named YYYYMMDD.tsv stay. A missing dir is not an error.
func Prune(dir string, before time.Time) error {
	files, err := dayFiles(dir, before.Location())
	if err != nil {
		return err
	}
	for _, f := range files {
		if !f.date.Before(midnight(before)) {
			continue
		}
		if err := os.Remove(f.path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s: %w", f.path, err)
		}
	}
	return nil
}

type dayFile struct {
	path string
	date time.Time
}

// dayFiles lists the YYYYMMDD.tsv files of dir sorted by name, dated in loc.
// It returns nil for a missing dir.
func dayFiles(dir string, loc *time.Location) ([]dayFile, error) {
	ents, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", dir, err)
	}
	files := []dayFile{}
	for _, ent := range ents {
		name, ok := strings.CutSuffix(ent.Name(), fileExt)
		if !ok || ent.IsDir() {
			continue
		}
		d, err := time.ParseInLocation(dateLayout, name, loc)
		if err != nil || d.Format(dateLayout) != name {
			continue
		}
		files = append(files, dayFile{path: filepath.Join(dir, ent.Name()), date: d})
	}
	return files, nil
}

func parseLine(line string, loc *time.Location) (Entry, bool) {
	parts := strings.Split(line, "\t")
	if len(parts) != 3 || parts[1] == "" {
		return Entry{}, false
	}
	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return Entry{}, false
	}
	return Entry{At: time.Unix(sec, 0).In(loc), Cwd: parts[1], Branch: parts[2]}, true
}

func midnight(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
