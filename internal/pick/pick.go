// Package pick lets the user choose month logs of every log in fzf and open them in the editor.
package pick

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/tro3373/his/internal/gen"
	"github.com/tro3373/his/internal/monthlog"
)

// Candidate is one month log to pick.
type Candidate struct {
	Log  string // name of the log it belongs to
	Path string
	Rel  string // Path relative to the log's dir
}

// Candidates is every month log of every log, the newest month first.
// Month logs of the same name keep the order of logs.
func Candidates(logs []gen.LogConfig) ([]Candidate, error) {
	var all []Candidate
	for _, l := range logs {
		paths, err := monthlog.List(l.Dir)
		if err != nil {
			return nil, fmt.Errorf("failed to list the month logs of %s: %w", l.Name, err)
		}
		for _, p := range paths {
			rel, err := filepath.Rel(l.Dir, p)
			if err != nil {
				return nil, err
			}
			all = append(all, Candidate{Log: l.Name, Path: p, Rel: rel})
		}
	}
	slices.SortStableFunc(all, func(a, b Candidate) int {
		return cmp.Compare(filepath.Base(b.Path), filepath.Base(a.Path))
	})
	return all, nil
}

// Run picks month logs of logs and opens them in the editor.
// Without an interactive terminal it writes each candidate's path to list instead.
func Run(logs []gen.LogConfig, interactive bool, list io.Writer) error {
	cands, err := Candidates(logs)
	if err != nil {
		return err
	}
	if !interactive {
		return writePaths(list, cands)
	}
	if len(cands) == 0 {
		return errors.New("no month log found under the gen.logs dirs")
	}
	chosen, err := Fzf(cands)
	if err != nil || len(chosen) == 0 {
		return err
	}
	return Edit(os.Getenv("EDITOR"), chosen)
}

func writePaths(w io.Writer, cands []Candidate) error {
	for _, c := range cands {
		if _, err := fmt.Fprintln(w, c.Path); err != nil {
			return err
		}
	}
	return nil
}
