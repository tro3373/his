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

// TestRun drives one log end to end: sessions, a recording and tasks in, the
// month's log out.
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
	var out bytes.Buffer
	if err := Run(cfg, now, time.Date(2026, 10, 30, 0, 0, 0, 0, jst), &out); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "notes/work/2026/2026-10-01-log.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
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
	}, "\n") + "\n"
	if string(got) != want {
		t.Errorf("work log =\n%s\nwant\n%s", got, want)
	}

	// The Meet call and the notes task, which no session worked in, go to the default personal log.
	personal, err := os.ReadFile(filepath.Join(root, "notes/personal/2026/2026-10-30-log.md"))
	if err != nil {
		t.Fatal(err)
	}
	wantPersonal := strings.Join([]string{
		"# Log", "", "## 2026-10-30 (金)",
		"### 20261030_110000 MTG 0.5h 私用",
		"- [ ] [20261030-090000_z](tasks/notes/20261030-090000_z/index.md)",
		"### 20261030_113000",
	}, "\n") + "\n"
	if string(personal) != wantPersonal {
		t.Errorf("personal log =\n%s\nwant\n%s", personal, wantPersonal)
	}

	var again bytes.Buffer
	if err := Run(cfg, now, time.Date(2026, 10, 30, 0, 0, 0, 0, jst), &again); err != nil {
		t.Fatal(err)
	}
	if again.Len() != 0 {
		t.Errorf("second run changed the log: %s", again.String())
	}
}

func TestRunToday(t *testing.T) {
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
	now := time.Date(2026, 11, 2, 15, 0, 0, 0, jst)
	if err := Run(cfg, now, now, io.Discard); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "notes/work/2026/2026-11-01-log.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"### 20261101_100000 notes 1h", "### 20261102_100000 notes 1h"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("today の指定で過去の足りない日と今日を書くこと: %q is missing in\n%s", want, got)
		}
	}
}
