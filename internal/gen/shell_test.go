package gen

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tro3373/his/internal/activity"
)

func TestRun_shellActivity(t *testing.T) {
	date := time.Date(2026, 10, 30, 0, 0, 0, 0, jst)
	run := time.Date(2026, 11, 2, 9, 0, 0, 0, jst)
	tests := []struct {
		name    string
		cwd     func(root string) string
		ignore  func(root string) []string
		wantLog bool
	}{
		{
			name:    "shellだけの作業の場合_時間見出しになること",
			cwd:     func(root string) string { return filepath.Join(root, "notes") },
			wantLog: true,
		},
		{
			name:   "ignoreにcwdが一致する場合_時間見出しにならないこと",
			cwd:    func(root string) string { return filepath.Join(root, "notes") },
			ignore: func(root string) []string { return []string{filepath.Join(root, "notes")} },
		},
		{
			name:    "ignoreがcwdの親の場合_時間見出しになること",
			cwd:     func(root string) string { return filepath.Join(root, "notes") },
			ignore:  func(root string) []string { return []string{root} },
			wantLog: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			actDir := filepath.Join(root, "act")
			cwd := tt.cwd(root)
			for _, at := range []time.Time{date.Add(10*time.Hour + 20*time.Minute), date.Add(11*time.Hour + 10*time.Minute)} {
				if err := activity.Append(actDir, activity.Entry{At: at, Cwd: cwd}); err != nil {
					t.Fatal(err)
				}
			}
			cfg := Config{
				StoreRoot: filepath.Join(root, "store"),
				Sessions:  []string{filepath.Join(root, "none")},
				Lunch:     noon,
				MinIdle:   time.Hour,
				Logs:      []LogConfig{{Name: "work", Dir: filepath.Join(root, "work"), Default: true}},
				Activity:  ActivityConfig{Dir: actDir, RetentionDays: 35},
			}
			if tt.ignore != nil {
				cfg.Activity.Ignore = tt.ignore(root)
			}
			if err := Run(cfg, run, date, io.Discard); err != nil {
				t.Fatal(err)
			}
			files, _ := filepath.Glob(filepath.Join(root, "work/2026/*-log.md"))
			var got strings.Builder
			for _, f := range files {
				b, err := os.ReadFile(f)
				if err != nil {
					t.Fatal(err)
				}
				got.Write(b)
			}
			const heading = "### 20261030_100000 notes"
			if has := strings.Contains(got.String(), heading); has != tt.wantLog {
				t.Errorf("log has %q = %v, want %v:\n%s", heading, has, tt.wantLog, got.String())
			}
		})
	}
}

func TestRun_prunesActivity(t *testing.T) {
	root := t.TempDir()
	actDir := filepath.Join(root, "act")
	old := time.Date(2026, 9, 1, 10, 0, 0, 0, jst)
	if err := activity.Append(actDir, activity.Entry{At: old, Cwd: "/w"}); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		StoreRoot: filepath.Join(root, "store"), Sessions: []string{filepath.Join(root, "none")},
		Lunch: noon, MinIdle: time.Hour, CatchUpDays: 30,
		Logs:     []LogConfig{{Name: "work", Dir: filepath.Join(root, "work"), Default: true}},
		Activity: ActivityConfig{Dir: actDir, RetentionDays: 35},
	}
	if err := Run(cfg, time.Date(2026, 11, 2, 9, 0, 0, 0, jst), time.Time{}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(actDir, "20260901.tsv")); !os.IsNotExist(err) {
		t.Errorf("保持日数より古い日のファイルを消すこと: %v", err)
	}
}

func TestConfigValidate_activity(t *testing.T) {
	valid := func() Config {
		c := DefaultConfig()
		c.StoreRoot = "/s"
		c.Sessions = []string{"/x"}
		c.Lunch = noon
		c.Logs = []LogConfig{{Name: "w", Dir: "/w", Default: true}}
		return c
	}
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{name: "既定値の場合_エラーにならないこと", mutate: func(*Config) {}},
		{name: "保持日数がcatch_up_daysより短い場合_エラーになること", mutate: func(c *Config) { c.Activity.RetentionDays = 29 }, wantErr: "gen.activity.retention_days"},
		{name: "保持日数がcatch_up_daysと同じ場合_エラーにならないこと", mutate: func(c *Config) { c.Activity.RetentionDays = 30 }},
		{name: "dirが空の場合_エラーになること", mutate: func(c *Config) { c.Activity.Dir = "" }, wantErr: "gen.activity.dir"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := valid()
			tt.mutate(&c)
			err := c.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
