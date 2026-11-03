package gen

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRun drives two logs end to end: sessions, recordings and tasks in, the
// month's logs out.
func TestRun(t *testing.T) {
	root := t.TempDir()
	cwd := filepath.Join(root, "src/acme/api/backend-worktree/feature-x")
	write(t, filepath.Join(root, "src/acme/api/.envrc"), "export JOB_TAG=ALPHA\n")
	write(t, filepath.Join(root, "src/beta/web/.envrc"), "export JOB_TAG=GAMMA\n")
	gamma := filepath.Join(root, "src/beta/web/app")
	entry := func(utc, dir, branch string) string {
		return `{"type":"user","timestamp":"` + utc + `","cwd":"` + dir + `","gitBranch":"` + branch + `"}` + "\n"
	}
	// JST = UTC+9: 10:20, 13:10, 14:00 (GAMMA), 16:30, 17:20, 18:10
	write(t, filepath.Join(root, "work/projects/p/s.jsonl"),
		entry("2026-10-30T01:20:00Z", cwd, "feature/x")+
			entry("2026-10-30T04:10:00Z", cwd, "feature/x")+
			entry("2026-10-30T07:30:00Z", cwd, "feature/x")+
			entry("2026-10-30T08:20:00Z", cwd, "feature/x")+
			entry("2026-10-30T09:10:00Z", cwd, "feature/x"))
	// A session in the personal config dir still belongs to the work log: the
	// working directory decides, not the config dir.
	write(t, filepath.Join(root, "personal/projects/p/s.jsonl"), entry("2026-10-30T05:00:00Z", gamma, "develop"))
	write(t, filepath.Join(root, "rec/20261030-150000/meta.json"), `{"app":"slack"}`)
	write(t, filepath.Join(root, "rec/20261030-150000/transcript.md"), "- [01:00:00] 相手: x\n")
	write(t, filepath.Join(root, "rec/20261030-150000/minutes.md"), "# 議事録: 定例\n")
	write(t, filepath.Join(root, "rec/20261030-110000/meta.json"), `{"app":"chrome"}`)
	write(t, filepath.Join(root, "rec/20261030-110000/transcript.md"), "- [00:30:00] 相手: 私用\n")
	write(t, filepath.Join(root, "rec/20261030-110000/minutes.md"), "# 議事録: 私用\n")
	write(t, filepath.Join(root, "notes/tasks/backend/20261029-100000_x/index.md"), "---\nbranch: feature/x\nstatus: ✅️\n---\n")
	write(t, filepath.Join(root, "notes/tasks/app/20261030-140500_y/index.md"), "---\nbranch: feature/y\nstatus:\n---\n")
	write(t, filepath.Join(root, "notes/tasks/notes/20261030-090000_z/index.md"), "---\nbranch: feature/z\nstatus:\n---\n")
	write(t, filepath.Join(root, "notes/work/2026/2026-10-01-log.md"), "# Log\n\n## Head\n\n## 2026-10-29 (木)\n### 20261029_100000 ALPHA\n")

	cfg := Config{
		StoreRoot: filepath.Join(root, "notes"),
		RecDir:    filepath.Join(root, "rec"),
		Sessions:  []string{filepath.Join(root, "work/projects"), filepath.Join(root, "personal/projects")},
		Lunch:     noon,
		Logs: []LogConfig{
			{Name: "work", Dir: filepath.Join(root, "notes/work"), Roots: []string{filepath.Join(root, "src")}, Apps: []string{"slack", "zoom"}},
			{Name: "personal", Dir: filepath.Join(root, "notes/personal"), Apps: []string{"chrome"}, Default: true},
		},
	}
	now := time.Date(2026, 11, 2, 9, 0, 0, 0, jst)
	date := time.Date(2026, 10, 30, 0, 0, 0, 0, jst)
	if err := Run(cfg, now, date, io.Discard); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		want []string
	}{
		{
			name: "作業ログは月のログに追記し_昼休み以外の空白を休憩にし_会議とタスクを区間に置くこと",
			path: "notes/work/2026/2026-10-01-log.md",
			want: []string{
				"# Log", "", "## Head", "", "## 2026-10-29 (木)", "### 20261029_100000 ALPHA", "",
				"## 2026-10-30 (金)",
				// Only one activity in the morning, then nothing until 13:10: a break from 10:30.
				"### 20261030_100000 ALPHA 0.5h",
				"### 20261030_103000",
				"### 20261030_130000 ALPHA 1h GAMMA 1h",
				"- [ ] [20261030-140500_y](tasks/app/20261030-140500_y/index.md)",
				"### 20261030_150000 ALPHA 1h 定例",
				"### 20261030_160000 ALPHA 2h",
				"- [x] [20261029-100000_x](tasks/backend/20261029-100000_x/index.md)",
				"### 20261030_180000",
			},
		},
		{
			name: "どのセッションも作業していない通話とタスクは既定のログに書くこと",
			path: "notes/personal/2026/2026-10-30-log.md",
			want: []string{
				"# Log", "", "## 2026-10-30 (金)",
				"### 20261030_110000 MTG 0.5h 私用",
				"- [ ] [20261030-090000_z](tasks/notes/20261030-090000_z/index.md)",
				"### 20261030_113000",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := os.ReadFile(filepath.Join(root, tt.path))
			if err != nil {
				t.Fatal(err)
			}
			if want := lines(tt.want...); string(got) != want {
				t.Errorf("%s =\n%s\nwant\n%s", tt.path, got, want)
			}
		})
	}

	var again bytes.Buffer
	if err := Run(cfg, now, date, &again); err != nil {
		t.Fatal(err)
	}
	if again.Len() != 0 {
		t.Errorf("再実行でログを変えないこと: %s", again.String())
	}
}

func TestRun_target(t *testing.T) {
	now := time.Date(2026, 11, 2, 15, 0, 0, 0, jst)
	yesterday := "### 20261101_100000 notes 1h"
	today := "### 20261102_100000 notes 1h"
	tests := []struct {
		name    string
		target  time.Time
		want    []string
		notWant []string
	}{
		{name: "指定が無ければ昨日までを書き_今日は書かないこと", want: []string{yesterday}, notWant: []string{today}},
		{name: "today の指定で過去の足りない日と今日を書くこと", target: now, want: []string{yesterday, today}},
		{name: "日付の指定でその日だけを書くこと", target: time.Date(2026, 11, 1, 0, 0, 0, 0, jst), want: []string{yesterday}, notWant: []string{today}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			cwd := filepath.Join(root, "notes")
			write(t, filepath.Join(root, "work/projects/p/s.jsonl"),
				`{"type":"user","timestamp":"2026-11-01T01:20:00Z","cwd":"`+cwd+`"}`+"\n"+
					`{"type":"user","timestamp":"2026-11-02T01:20:00Z","cwd":"`+cwd+`"}`+"\n")
			cfg := Config{
				StoreRoot: cwd,
				Sessions:  []string{filepath.Join(root, "work/projects")},
				Lunch:     noon,
				Logs:      []LogConfig{{Name: "work", Dir: filepath.Join(root, "notes/work"), Default: true}},
			}
			if err := Run(cfg, now, tt.target, io.Discard); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(root, "notes/work/2026/2026-11-01-log.md"))
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tt.want {
				if !strings.Contains(string(got), w) {
					t.Errorf("%q is missing in\n%s", w, got)
				}
			}
			for _, w := range tt.notWant {
				if strings.Contains(string(got), w) {
					t.Errorf("%q should not be in\n%s", w, got)
				}
			}
		})
	}
}
