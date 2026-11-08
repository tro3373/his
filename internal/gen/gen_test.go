package gen

import (
	"bytes"
	"cmp"
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
		MinIdle:   time.Hour,
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
	older := "### 20261031_100000 notes 1h"
	handWritten := "### 20261101_090000 hand"
	endedDay := func(header, stamp string) string {
		return "# Log\n\n## " + header + "\n### " + stamp + "_090000 KEPT 1h\n### " + stamp + "_100000\n"
	}
	endedOct31 := endedDay("2026-10-31 (土)", "20261031")
	endedNov01 := endedDay("2026-11-01 (日)", "20261101")
	appendedNov01 := "### 20261101_100000 notes 1h\n### 20261101_110000"
	twoHoursNov01 := []string{"2026-10-31T01:20:00Z", "2026-11-01T01:20:00Z", "2026-11-01T02:10:00Z", "2026-11-02T01:20:00Z"}
	lateMorning := time.Date(2026, 11, 2, 11, 50, 0, 0, jst)
	dayStamps := []string{"2026-10-31T01:20:00Z", "2026-11-02T00:10:00Z", "2026-11-02T01:10:00Z", "2026-11-02T02:20:00Z"}
	tests := []struct {
		name    string
		target  time.Time
		logs    map[string]string // file name under the log dir's year => content
		now     time.Time         // zero means the run at the top of this test
		stamps  []string          // session messages; empty means one on each of the last 3 days
		lag     time.Duration
		days    int           // zero means 30
		idle    time.Duration // zero means an hour
		want    []string
		notWant []string
	}{
		{name: "指定も書いた日も無ければ_30日の窓を今日まで書くこと", want: []string{older, yesterday, today}},
		{
			name:    "手書きの日があれば_それより古い日は書かず_見出しより後を追記すること",
			logs:    map[string]string{"2026-11-01-log.md": "# Log\n\n## 2026-11-01 (日)\n" + handWritten + "\n"},
			want:    []string{handWritten + "\n" + yesterday, today},
			notWant: []string{older},
		},
		{
			name:    "終業見出しの時刻から作業があれば_その見出しを置き換えて追記すること",
			stamps:  twoHoursNov01,
			logs:    map[string]string{"2026-11-01-log.md": endedNov01},
			want:    []string{"### 20261101_090000 KEPT 1h\n" + appendedNov01 + "\n", today},
			notWant: []string{older},
		},
		{
			name:   "書いた日が2つあれば_古い日は触らないこと",
			stamps: twoHoursNov01,
			logs: map[string]string{
				"2026-10-01-log.md": endedOct31,
				"2026-11-01-log.md": endedNov01,
			},
			want:    []string{appendedNov01, today, "### 20261031_090000 KEPT 1h\n### 20261031_100000\n"},
			notWant: []string{older},
		},
		{
			name: "時刻の見出しが無い節は書いた日に数えず_その前の書いた日から書くこと",
			logs: map[string]string{
				"2026-10-01-log.md": endedOct31,
				"2026-11-01-log.md": "# Log\n\n## 2026-11-01 (日)\n- メモ\n",
			},
			want: []string{"### 20261031_090000 KEPT 1h", yesterday, today, "- メモ"},
		},
		{
			name:   "今日が書いてあっても_昨日の見出しより後を追記すること",
			stamps: twoHoursNov01,
			logs: map[string]string{
				"2026-11-01-log.md": endedNov01 + "\n## 2026-11-02 (月)\n### 20261102_090000 hand\n",
			},
			want:    []string{appendedNov01, "### 20261102_090000 hand"},
			notWant: []string{older},
		},
		{
			name:    "日付の指定でその日だけを書くこと",
			target:  time.Date(2026, 11, 1, 0, 0, 0, 0, jst),
			want:    []string{yesterday},
			notWant: []string{today},
		},
		{name: "lagの分だけ前で1日が終わること", now: lateMorning, stamps: dayStamps, lag: time.Hour, want: []string{"### 20261102_104500\n"}},
		{name: "catch_up_daysより前の日を書かないこと", now: lateMorning, stamps: dayStamps, days: 1, notWant: []string{"2026-10-31"}},
		{name: "catch_up_days以内の日を書くこと", now: lateMorning, stamps: dayStamps, days: 2, want: []string{"2026-10-31"}},
		{name: "min_idle以上の空白を休憩にすること", now: lateMorning, stamps: dayStamps, idle: 30 * time.Minute, want: []string{"### 20261102_093000\n"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			cwd := filepath.Join(root, "notes")
			stamps := tt.stamps
			if len(stamps) == 0 {
				stamps = []string{"2026-10-31T01:20:00Z", "2026-11-01T01:20:00Z", "2026-11-02T01:20:00Z"}
			}
			var session strings.Builder
			for _, ts := range stamps {
				session.WriteString(`{"type":"user","timestamp":"` + ts + `","cwd":"` + cwd + `"}` + "\n")
			}
			write(t, filepath.Join(root, "work/projects/p/s.jsonl"), session.String())
			for name, content := range tt.logs {
				write(t, filepath.Join(root, "notes/work/2026", name), content)
			}
			cfg := Config{
				StoreRoot:   cwd,
				Sessions:    []string{filepath.Join(root, "work/projects")},
				Lunch:       noon,
				Lag:         tt.lag,
				MinIdle:     cmp.Or(tt.idle, time.Hour),
				CatchUpDays: cmp.Or(tt.days, 30),
				Logs:        []LogConfig{{Name: "work", Dir: filepath.Join(root, "notes/work"), Default: true}},
			}
			if err := Run(cfg, cmp.Or(tt.now, now), tt.target, io.Discard); err != nil {
				t.Fatal(err)
			}
			var again bytes.Buffer
			if err := Run(cfg, cmp.Or(tt.now, now), tt.target, &again); err != nil {
				t.Fatal(err)
			}
			if again.Len() != 0 {
				t.Errorf("再実行でログを変えないこと: %s", again.String())
			}
			var all strings.Builder
			files, _ := filepath.Glob(filepath.Join(root, "notes/work/2026/*-log.md"))
			for _, f := range files {
				b, err := os.ReadFile(f)
				if err != nil {
					t.Fatal(err)
				}
				all.Write(b)
			}
			got := all.String()
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("%q is missing in\n%s", w, got)
				}
			}
			for _, w := range tt.notWant {
				if strings.Contains(got, w) {
					t.Errorf("%q should not be in\n%s", w, got)
				}
			}
		})
	}
}

// TestRun_appendTaskLinks drives a day that already has an end heading:
// only tasks of the appended part get a link, and finished tasks get checked.
func TestRun_appendTaskLinks(t *testing.T) {
	root := t.TempDir()
	cwd := filepath.Join(root, "notes")
	entry := func(utc, branch string) string {
		return `{"type":"user","timestamp":"` + utc + `","cwd":"` + cwd + `","gitBranch":"` + branch + `"}` + "\n"
	}
	// JST = UTC+9: 09:30 before the end heading, 10:20 and 11:10 after it.
	write(t, filepath.Join(root, "work/projects/p/s.jsonl"),
		entry("2026-11-01T00:30:00Z", "feature/old")+entry("2026-11-01T01:20:00Z", "feature/new")+entry("2026-11-01T02:10:00Z", "feature/new"))
	task := func(name, branch, status string) {
		write(t, filepath.Join(root, "notes/tasks/notes", name, "index.md"), "---\nbranch: "+branch+"\nstatus: "+status+"\n---\n")
	}
	task("20261030-090000_done", "feature/done", "✅️")
	task("20261030-090000_old", "feature/old", "")
	task("20261101-093000_split", "", "")
	task("20261030-090000_new", "feature/new", "")
	doneLink := "[20261030-090000_done](tasks/notes/20261030-090000_done/index.md)"
	log := filepath.Join(root, "notes/work/2026/2026-11-01-log.md")
	write(t, log, lines("# Log", "", "## 2026-11-01 (日)", "### 20261101_090000 KEPT 1h", "- [ ] "+doneLink, "### 20261101_100000"))
	cfg := Config{
		StoreRoot: cwd,
		Sessions:  []string{filepath.Join(root, "work/projects")},
		Lunch:     noon,
		MinIdle:   time.Hour,
		Logs:      []LogConfig{{Name: "work", Dir: filepath.Join(root, "notes/work"), Default: true}},
	}
	now := time.Date(2026, 11, 2, 15, 0, 0, 0, jst)
	if err := Run(cfg, now, time.Date(2026, 11, 1, 0, 0, 0, 0, jst), io.Discard); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	want := lines("# Log", "", "## 2026-11-01 (日)", "### 20261101_090000 KEPT 1h", "- [x] "+doneLink,
		"### 20261101_100000 notes 1h", "- [ ] [20261030-090000_new](tasks/notes/20261030-090000_new/index.md)", "### 20261101_110000")
	if string(got) != want {
		t.Errorf("log =\n%s\nwant\n%s", got, want)
	}
}
