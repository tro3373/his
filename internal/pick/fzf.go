package pick

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	exitNoMatch    = 1
	exitInterrupt  = 130
	previewCommand = "tail -n 200 {1}"
)

// fzf lets the user choose candidates in fzf and returns the chosen paths.
// Choosing nothing, or leaving with Esc, returns no paths and no error.
func fzf(cands []Candidate) ([]string, error) {
	var in strings.Builder
	for _, c := range cands {
		fmt.Fprintf(&in, "%s\t%s\t%s\n", c.Path, c.Log, c.Rel)
	}
	var out bytes.Buffer
	cmd := exec.Command("fzf",
		"--delimiter", "\t", "--with-nth", "2..", "--multi", "--preview", previewCommand)
	cmd.Stdin = strings.NewReader(in.String())
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) && (exit.ExitCode() == exitNoMatch || exit.ExitCode() == exitInterrupt) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to run fzf: %w", err)
	}
	return chosenPaths(out.String()), nil
}

func chosenPaths(output string) []string {
	var paths []string
	for line := range strings.Lines(output) {
		path, _, _ := strings.Cut(strings.TrimRight(line, "\r\n"), "\t")
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}
