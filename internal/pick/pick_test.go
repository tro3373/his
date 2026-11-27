package pick

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tro3373/his/internal/gen"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# Log\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCandidates(t *testing.T) {
	tests := []struct {
		name  string
		files map[string][]string
		order []string
		want  []string
	}{
		{
			name:  "複数のlogの場合_新しい月から並ぶこと",
			files: map[string][]string{"job": {"2026/2026-10-01-log.md", "2026/2026-11-02-log.md"}, "prv": {"2026/2026-10-05-log.md"}},
			order: []string{"job", "prv"},
			want:  []string{"job 2026/2026-11-02-log.md", "prv 2026/2026-10-05-log.md", "job 2026/2026-10-01-log.md"},
		},
		{
			name:  "ファイル名が同じ場合_gen.logsの順になること",
			files: map[string][]string{"job": {"2026/2026-11-02-log.md"}, "prv": {"2026/2026-11-02-log.md"}},
			order: []string{"prv", "job"},
			want:  []string{"prv 2026/2026-11-02-log.md", "job 2026/2026-11-02-log.md"},
		},
		{
			name:  "年をまたぐ場合_年が新しい方が先になること",
			files: map[string][]string{"job": {"2025/2025-12-01-log.md", "2026/2026-01-05-log.md"}},
			order: []string{"job"},
			want:  []string{"job 2026/2026-01-05-log.md", "job 2025/2025-12-01-log.md"},
		},
		{
			name:  "月のログが無い場合_空になること",
			files: map[string][]string{"job": nil},
			order: []string{"job"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			var logs []gen.LogConfig
			for _, name := range tt.order {
				dir := filepath.Join(root, name)
				for _, f := range tt.files[name] {
					touch(t, filepath.Join(dir, f))
				}
				logs = append(logs, gen.LogConfig{Name: name, Dir: dir})
			}
			got, err := Candidates(logs)
			if err != nil {
				t.Fatal(err)
			}
			var gotLines []string
			for _, c := range got {
				if c.Path != filepath.Join(root, c.Log, c.Rel) {
					t.Errorf("Path = %q, want under %s/%s", c.Path, c.Log, c.Rel)
				}
				gotLines = append(gotLines, c.Log+" "+c.Rel)
			}
			if !slices.Equal(gotLines, tt.want) {
				t.Errorf("Candidates() = %v, want %v", gotLines, tt.want)
			}
		})
	}
}

func TestChosenPaths(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{name: "複数行の場合_先頭のフィールドだけを返すこと", in: "/a/x\tjob\t2026/x\n/b y/z\tprv\t2026/z\n", want: []string{"/a/x", "/b y/z"}},
		{name: "空の場合_何も返さないこと", in: ""},
		{name: "空行がある場合_飛ばすこと", in: "\n/a/x\tjob\tx\n", want: []string{"/a/x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chosenPaths(tt.in)
			if !slices.Equal(got, tt.want) {
				t.Errorf("chosenPaths() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFzf(t *testing.T) {
	cands := []Candidate{
		{Log: "job", Path: "/n/job/2026/2026-11-02-log.md", Rel: "2026/2026-11-02-log.md"},
		{Log: "prv", Path: "/n/prv/2026/2026-10-05-log.md", Rel: "2026/2026-10-05-log.md"},
	}
	tests := []struct {
		name    string
		out     string
		code    string
		want    []string
		wantErr string
	}{
		{name: "複数選んだ場合_選んだ行のpathを返すこと", out: "/n/job/2026/2026-11-02-log.md\tjob\t2026/2026-11-02-log.md\n/n/prv/2026/2026-10-05-log.md\tprv\t2026/2026-10-05-log.md\n", code: "0", want: []string{"/n/job/2026/2026-11-02-log.md", "/n/prv/2026/2026-10-05-log.md"}},
		{name: "何も選ばず終了コード1の場合_pathもエラーも無いこと", code: "1"},
		{name: "Escで終了コード130の場合_pathもエラーも無いこと", code: "130"},
		{name: "終了コード2の場合_エラーになること", code: "2", wantErr: "failed to run fzf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bin := t.TempDir()
			work := t.TempDir()
			writeScript(t, bin, "fzf", `printf '%s\n' "$@" > "$WORK/args"
cat > "$WORK/stdin"
printf '%s' "$FAKE_OUT"
exit "$FAKE_CODE"
`)
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("WORK", work)
			t.Setenv("FAKE_OUT", tt.out)
			t.Setenv("FAKE_CODE", tt.code)

			got, err := fzf(cands)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("fzf() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("fzf() = %v, want %v", got, tt.want)
			}
			stdin, _ := os.ReadFile(filepath.Join(work, "stdin"))
			wantIn := "/n/job/2026/2026-11-02-log.md\tjob\t2026/2026-11-02-log.md\n/n/prv/2026/2026-10-05-log.md\tprv\t2026/2026-10-05-log.md\n"
			if string(stdin) != wantIn {
				t.Errorf("fzf stdin = %q, want %q", stdin, wantIn)
			}
			args, _ := os.ReadFile(filepath.Join(work, "args"))
			for _, want := range []string{"--multi", "--with-nth\n2..", "--delimiter\n\t", "--preview\ntail -n 200 {1}"} {
				if !strings.Contains(string(args), want) {
					t.Errorf("fzf args = %q, want containing %q", args, want)
				}
			}
		})
	}
}

func TestFzf_notInstalled(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := fzf([]Candidate{{Path: "/a"}}); err == nil || !strings.Contains(err.Error(), "failed to run fzf") {
		t.Errorf("fzf() error = %v, want failed to run fzf", err)
	}
}
