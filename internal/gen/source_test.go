package gen

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestTagResolver(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "work/acme/api/.envrc"), "source_up_if_exists\nexport JOB_TAG=\"ALPHA\"\n")
	write(t, filepath.Join(root, "work/acme/api/backend/.envrc"), "use flake\n")
	write(t, filepath.Join(root, "repos/lib/.git/HEAD"), "ref: refs/heads/main\n")
	write(t, filepath.Join(root, "repos/app-worktree/feature-y/.git"), "gitdir: x\n")
	tests := []struct {
		name     string
		cwd      string
		wantTag  string
		wantRepo string
	}{
		{name: "親へ辿って最初の JOB_TAG を使うこと", cwd: "work/acme/api/backend", wantTag: "ALPHA", wantRepo: "backend"},
		{name: "消えた worktree でも親の JOB_TAG を使い_リポジトリ名は worktree の元にすること", cwd: "work/acme/api/backend-worktree/feature-x", wantTag: "ALPHA", wantRepo: "backend"},
		{name: "JOB_TAG が無ければリポジトリ名を案件にすること", cwd: "repos/notes", wantTag: "notes", wantRepo: "notes"},
		{name: "サブディレクトリで起動しても .git のあるディレクトリをリポジトリにすること", cwd: "repos/lib/repro/build-tags", wantTag: "lib", wantRepo: "lib"},
		{name: "残っている worktree のサブディレクトリは worktree の元をリポジトリにすること", cwd: "repos/app-worktree/feature-y/pkg", wantTag: "app", wantRepo: "app"},
	}
	r := NewTagResolver()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag, repo := r.Resolve(filepath.Join(root, tt.cwd))
			if tag != tt.wantTag || repo != tt.wantRepo {
				t.Errorf("Resolve() = (%q, %q), want (%q, %q)", tag, repo, tt.wantTag, tt.wantRepo)
			}
		})
	}
}

func TestReadActivities(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "work/acme/api/.envrc"), "export JOB_TAG=ALPHA\n")
	cwd := filepath.Join(root, "work/acme/api/backend")
	sessions := filepath.Join(root, "projects")
	write(t, filepath.Join(sessions, "p/s1.jsonl"),
		`{"type":"permission-mode"}`+"\n"+
			`{"type":"user","timestamp":"2026-10-30T01:15:00.000Z","cwd":"`+cwd+`","gitBranch":"feature/x","message":{"content":"hi"}}`+"\n"+
			`{"type":"system","timestamp":"2026-10-30T01:16:00.000Z","cwd":"`+cwd+`"}`+"\n"+
			`{"type":"assistant","timestamp":"2026-10-29T01:00:00.000Z","cwd":"`+cwd+`"}`+"\n")
	// claude -p run by cron or by another tool is not someone working.
	write(t, filepath.Join(sessions, "p/s2.jsonl"),
		`{"type":"user","timestamp":"2026-10-30T03:00:00.000Z","cwd":"`+cwd+`","entrypoint":"sdk-cli"}`+"\n")
	write(t, filepath.Join(sessions, "p/s1/subagents/a.jsonl"),
		`{"type":"assistant","timestamp":"2026-10-30T02:00:00.000Z","cwd":"`+cwd+`","gitBranch":"feature/x"}`+"\n")
	got, err := ReadActivities(sessions, at(0, 0), NewTagResolver())
	if err != nil {
		t.Fatal(err)
	}
	want := []Activity{
		{At: time.Date(2026, 10, 30, 1, 15, 0, 0, time.UTC), Tag: "ALPHA", Repo: "backend", Branch: "feature/x"},
		{At: time.Date(2026, 10, 30, 2, 0, 0, 0, time.UTC), Tag: "ALPHA", Repo: "backend", Branch: "feature/x"},
	}
	slices.SortFunc(got, func(a, b Activity) int { return a.At.Compare(b.At) })
	if len(got) != len(want) {
		t.Fatalf("ReadActivities() = %+v, want %+v", got, want)
	}
	for i := range want {
		if !got[i].At.Equal(want[i].At) || got[i].Tag != want[i].Tag || got[i].Repo != want[i].Repo || got[i].Branch != want[i].Branch {
			t.Errorf("ReadActivities()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseRecording(t *testing.T) {
	tests := []struct {
		name       string
		dir        string
		meta       string
		transcript string
		minutes    string
		want       Recording
		ok         bool
	}{
		{
			name:       "開始はディレクトリ名_長さは最後の発言_題名は議事録の 1 行目から取ること",
			dir:        "20261028-150210",
			meta:       `{"app":"slack"}`,
			transcript: "- [00:00:05] 自分: お疲れ様です\n- [01:05:59] 相手: 失礼します\n",
			minutes:    "# 議事録: 定例ミーティング\n\n## 概要\n",
			want: Recording{
				Start: time.Date(2026, 10, 28, 15, 2, 10, 0, jst),
				End:   time.Date(2026, 10, 28, 16, 8, 9, 0, jst),
				App:   "slack",
				Title: "定例ミーティング",
			},
			ok: true,
		},
		{
			name:       "議事録が見出しで始まらなければ題名を付けないこと",
			dir:        "20261028-150210",
			meta:       `{"app":"zoom"}`,
			transcript: "- [00:30:00] 相手: x\n",
			minutes:    "文字起こしの本文が届いていないため、議事録を作れません。\n",
			want: Recording{
				Start: time.Date(2026, 10, 28, 15, 2, 10, 0, jst),
				End:   time.Date(2026, 10, 28, 15, 32, 10, 0, jst),
				App:   "zoom",
			},
			ok: true,
		},
		{
			name: "アプリの記録が無い録音は使わないこと",
			dir:  "20261028-150210", transcript: "- [01:05:59] 相手: x\n", minutes: "# 議事録: x\n",
			ok: false,
		},
		{
			name: "5 分未満の録音は使わないこと",
			dir:  "20261028-150210", meta: `{"app":"slack"}`, transcript: "- [00:04:00] 相手: x\n", minutes: "# 議事録: x\n",
			ok: false,
		},
		{
			name: "発言の無い録音は使わないこと",
			dir:  "20261028-150210", meta: `{"app":"slack"}`, transcript: "", minutes: "文字起こしが空\n",
			ok: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseRecording(tt.dir, []byte(tt.meta), []byte(tt.transcript), []byte(tt.minutes), jst)
			if ok != tt.ok {
				t.Fatalf("parseRecording() ok = %v, want %v", ok, tt.ok)
			}
			if ok && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseRecording() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseTask(t *testing.T) {
	tests := []struct {
		name    string
		rel     string
		content string
		want    Task
		ok      bool
	}{
		{
			name:    "front matter のブランチと完了を読み_作成日はディレクトリ名から取ること",
			rel:     "tasks/backend/20261030-144224_console-文言修正/index.md",
			content: "---\ntitle: x\nbranch: feature/console\nstatus: ✅️🚀\n---\n\n- [x] a\n",
			want: Task{
				Path: "tasks/backend/20261030-144224_console-文言修正/index.md", Title: "20261030-144224_console-文言修正",
				Repo: "backend", Branch: "feature/console", Status: taskDone,
				Created: time.Date(2026, 10, 30, 14, 42, 24, 0, jst),
			},
			ok: true,
		},
		{
			name:    "未完了なら Done は false にすること",
			rel:     "tasks/backend/20261030-144224_x/index.md",
			content: "---\nbranch: feature/x\nstatus:\n---\n",
			want: Task{
				Path: "tasks/backend/20261030-144224_x/index.md", Title: "20261030-144224_x",
				Repo: "backend", Branch: "feature/x", Created: time.Date(2026, 10, 30, 14, 42, 24, 0, jst),
			},
			ok: true,
		},
		{
			name:    "task.md の status が done なら完了として読むこと",
			rel:     "tasks/backend/20261030-144224_x/task.md",
			content: "---\nid: 20261030-144224\nstatus: done\nbranch: feature/x\n---\n",
			want: Task{
				Path: "tasks/backend/20261030-144224_x/task.md", Title: "20261030-144224_x",
				Repo: "backend", Branch: "feature/x", Status: taskDone, Created: time.Date(2026, 10, 30, 14, 42, 24, 0, jst),
			},
			ok: true,
		},
		{
			name:    "task.md の status が done 以外なら未完了として読むこと",
			rel:     "tasks/backend/20261030-144224_x/task.md",
			content: "---\nstatus: doing\nbranch: feature/x\n---\n",
			want: Task{
				Path: "tasks/backend/20261030-144224_x/task.md", Title: "20261030-144224_x",
				Repo: "backend", Branch: "feature/x", Created: time.Date(2026, 10, 30, 14, 42, 24, 0, jst),
			},
			ok: true,
		},
		{
			name:    "ended を読むこと",
			rel:     "tasks/backend/20261030-144224_x/task.md",
			content: "---\nstatus: done\nbranch: main\nended: 2026-10-30T18:00:00+09:00\n---\n",
			want: Task{
				Path: "tasks/backend/20261030-144224_x/task.md", Title: "20261030-144224_x",
				Repo: "backend", Branch: "main", Status: taskDone, Created: time.Date(2026, 10, 30, 14, 42, 24, 0, jst),
				Ended: time.Date(2026, 10, 30, 18, 0, 0, 0, jst),
			},
			ok: true,
		},
		{
			name:    "status が todo なら NotStarted にすること",
			rel:     "tasks/backend/20261030-144224_x/task.md",
			content: "---\nstatus: todo\nbranch: feature/x\n---\n",
			want: Task{
				Path: "tasks/backend/20261030-144224_x/task.md", Title: "20261030-144224_x",
				Repo: "backend", Branch: "feature/x", Status: taskNotStarted, Created: time.Date(2026, 10, 30, 14, 42, 24, 0, jst),
			},
			ok: true,
		},
		{
			name:    "status が ready なら NotStarted にして started を読むこと",
			rel:     "tasks/backend/20261030-144224_x/task.md",
			content: "---\nstatus: ready\nbranch: feature/x\nstarted: 2026-10-30T15:00:00+09:00\n---\n",
			want: Task{
				Path: "tasks/backend/20261030-144224_x/task.md", Title: "20261030-144224_x",
				Repo: "backend", Branch: "feature/x", Status: taskNotStarted, Created: time.Date(2026, 10, 30, 14, 42, 24, 0, jst),
				Started: time.Date(2026, 10, 30, 15, 0, 0, 0, jst),
			},
			ok: true,
		},
		{
			name: "日時で始まらないディレクトリはタスクとして扱わないこと",
			rel:  "tasks/backend/index.md", content: "- [ ] a\n",
			ok: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseTask(tt.rel, []byte(tt.content), jst)
			if ok != tt.ok {
				t.Fatalf("parseTask() ok = %v, want %v", ok, tt.ok)
			}
			if ok && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTask() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestReadTasks(t *testing.T) {
	tests := []struct {
		name string
		file string
	}{
		{name: "index.md のタスクを読むこと", file: "index.md"},
		{name: "task.md のタスクを読むこと", file: "task.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			rel := filepath.Join("tasks", "backend", "20261030-144224_x", tt.file)
			write(t, filepath.Join(root, rel), "---\nbranch: feature/x\n---\n")

			got, err := ReadTasks(root, jst)
			if err != nil {
				t.Fatalf("ReadTasks() error = %v", err)
			}
			if len(got) != 1 || got[0].Path != rel {
				t.Errorf("ReadTasks() = %+v, want one task at %s", got, rel)
			}
		})
	}
}

func TestTaskWorkedIn(t *testing.T) {
	created := time.Date(2026, 11, 9, 10, 0, 0, 0, jst)
	ended := time.Date(2026, 11, 9, 18, 0, 0, 0, jst)
	open := Task{Repo: "r", Branch: "main", Created: created}
	closed := Task{Repo: "r", Branch: "main", Created: created, Ended: ended}
	tests := []struct {
		name string
		task Task
		act  Activity
		want bool
	}{
		{name: "作成の時刻の Activity は作業と数えること", task: closed, act: Activity{At: created, Repo: "r", Branch: "main"}, want: true},
		{name: "終了の時刻の Activity は作業と数えること", task: closed, act: Activity{At: ended, Repo: "r", Branch: "main"}, want: true},
		{name: "作成より前の Activity は作業と数えないこと", task: closed, act: Activity{At: created.Add(-time.Second), Repo: "r", Branch: "main"}},
		{name: "終了より後の Activity は作業と数えないこと", task: closed, act: Activity{At: ended.Add(time.Second), Repo: "r", Branch: "main"}},
		{name: "終了していなければ後の日の Activity も作業と数えること", task: open, act: Activity{At: ended.AddDate(0, 0, 3), Repo: "r", Branch: "main"}, want: true},
		{name: "別の branch の Activity は作業と数えないこと", task: closed, act: Activity{At: created, Repo: "r", Branch: "x"}},
		{name: "別の repo の Activity は作業と数えないこと", task: closed, act: Activity{At: created, Repo: "z", Branch: "main"}},
		{name: "started より前の Activity は作業と数えないこと", task: Task{Repo: "r", Branch: "main", Created: created, Started: ended}, act: Activity{At: created, Repo: "r", Branch: "main"}},
		{name: "started の Activity は作業と数えること", task: Task{Repo: "r", Branch: "main", Created: created, Started: ended}, act: Activity{At: ended, Repo: "r", Branch: "main"}, want: true},
		{name: "branch の無いタスクは作業と数えないこと", task: Task{Repo: "r", Created: created}, act: Activity{At: created, Repo: "r"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.task.workedIn(tt.act); got != tt.want {
				t.Errorf("workedIn() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLinksOfTaskWindow(t *testing.T) {
	d := time.Date(2026, 11, 9, 0, 0, 0, 0, jst)
	day := Day{End: d.Add(18 * time.Hour), Segments: []Segment{{Start: d.Add(9 * time.Hour)}, {Start: d.Add(13 * time.Hour)}}}
	act := func(h int) Activity {
		return Activity{At: d.Add(time.Duration(h) * time.Hour), Repo: "r", Branch: "main"}
	}
	tests := []struct {
		name string
		task Task
		own  bool
		acts []Activity
		want []int
	}{
		{
			name: "終了後の日の Activity だけならタスクを載せないこと",
			task: Task{Repo: "r", Branch: "main", Created: d.AddDate(0, 0, -2), Ended: d.AddDate(0, 0, -1)},
			acts: []Activity{act(10)},
		},
		{
			name: "作成前の日の Activity だけならタスクを載せないこと",
			task: Task{Repo: "r", Branch: "main", Created: d.AddDate(0, 0, 1)},
			acts: []Activity{act(10)},
		},
		{
			name: "終了後の Activity が多くても終了前の区間に置くこと",
			task: Task{Repo: "r", Branch: "main", Created: d, Ended: d.Add(12 * time.Hour)},
			acts: []Activity{act(10), act(14), act(15)},
			want: []int{0},
		},
		{
			name: "着手前のタスクは作成日でも載せないこと",
			task: Task{Repo: "r", Branch: "feature/x", Created: d.Add(10 * time.Hour), Status: taskNotStarted},
			own:  true,
		},
		{
			name: "着手前のタスクは branch の Activity があっても載せないこと",
			task: Task{Repo: "r", Branch: "main", Created: d, Status: taskNotStarted},
			acts: []Activity{act(10)},
		},
		{
			name: "着手済みのタスクは作成日に載せること",
			task: Task{Repo: "r", Branch: "feature/x", Created: d.Add(10 * time.Hour)},
			own:  true,
			want: []int{0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []int
			for _, l := range linksOf([]Task{tt.task}, day, tt.acts, d, func(Task) bool { return tt.own }) {
				got = append(got, l.Segment)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("segments = %v, want %v", got, tt.want)
			}
		})
	}
}
