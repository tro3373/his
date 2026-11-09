package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/tro3373/his/internal/gen"
)

func TestLoadGenConfig(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]any
		wantLag   time.Duration
		wantDays  int
		wantIdle  time.Duration
		wantErr   bool
		wantDir   string   // under home; empty means the default
		wantSkip  []string // "~" stands for home; nil with noSkip unset means the default ~
		noSkip    bool
	}{
		{name: "未設定の場合_既定値になること", wantLag: 30 * time.Minute, wantDays: 30, wantIdle: time.Hour},
		{
			name:      "設定した場合_設定値になること",
			overrides: map[string]any{"lag": "2h", "catch_up_days": 7, "min_idle": "45m"},
			wantLag:   2 * time.Hour, wantDays: 7, wantIdle: 45 * time.Minute,
		},
		{
			name:      "lagに0を指定した場合_既定値に戻らないこと",
			overrides: map[string]any{"lag": "0s"},
			wantLag:   0, wantDays: 30, wantIdle: time.Hour,
		},
		{name: "lagが負の場合_エラーになること", overrides: map[string]any{"lag": "-1m"}, wantErr: true},
		{name: "min_idleが0の場合_エラーになること", overrides: map[string]any{"min_idle": "0s"}, wantErr: true},
		{name: "catch_up_daysが負の場合_エラーになること", overrides: map[string]any{"catch_up_days": -1}, wantErr: true},
		{name: "ignoreが未設定の場合_既定の~をホームに展開すること", wantLag: 30 * time.Minute, wantDays: 30, wantIdle: time.Hour},
		{name: "ignoreが空の場合_空になること", overrides: map[string]any{"activity": map[string]any{"ignore": []string{}}}, noSkip: true, wantLag: 30 * time.Minute, wantDays: 30, wantIdle: time.Hour},
		{name: "ignoreを指定した場合_指定だけになること", overrides: map[string]any{"activity": map[string]any{"ignore": []string{"/tmp"}}}, wantSkip: []string{"/tmp"}, wantLag: 30 * time.Minute, wantDays: 30, wantIdle: time.Hour},
		{name: "activityのdirに~を使った場合_ホームに展開すること", overrides: map[string]any{"activity": map[string]any{"dir": "~/act"}}, wantDir: "act", wantLag: 30 * time.Minute, wantDays: 30, wantIdle: time.Hour},
		{name: "retention_daysがcatch_up_daysより短い場合_エラーになること", overrides: map[string]any{"activity": map[string]any{"retention_days": 29}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			viper.Reset()
			t.Cleanup(viper.Reset)
			section := map[string]any{
				"store_root": "/notes",
				"sessions":   []string{"/sessions"},
				"lunch":      map[string]any{"start": "12:00", "end": "13:00"},
				"logs":       []map[string]any{{"name": "work", "dir": "/notes/work", "default": true}},
			}
			for k, v := range tt.overrides {
				section[k] = v
			}
			viper.Set("gen", section)

			got, err := loadGenConfig()
			if (err != nil) != tt.wantErr {
				t.Fatalf("loadGenConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			wantSkip := make([]string, 0, len(tt.wantSkip))
			if tt.wantSkip == nil && !tt.noSkip {
				tt.wantSkip = []string{"~"}
			}
			for _, p := range tt.wantSkip {
				wantSkip = append(wantSkip, strings.Replace(p, "~", home, 1))
			}
			if !slices.Equal(got.Activity.Ignore, wantSkip) {
				t.Errorf("Activity.Ignore = %v, want %v", got.Activity.Ignore, wantSkip)
			}
			wantDir := filepath.Join(home, ".local/state/his/activity")
			if tt.wantDir != "" {
				wantDir = filepath.Join(home, tt.wantDir)
			}
			if got.Activity.Dir != wantDir {
				t.Errorf("Activity.Dir = %q, want %q", got.Activity.Dir, wantDir)
			}
			want := gen.Config{Lag: tt.wantLag, CatchUpDays: tt.wantDays, MinIdle: tt.wantIdle}
			if got.Lag != want.Lag || got.CatchUpDays != want.CatchUpDays || got.MinIdle != want.MinIdle {
				t.Errorf("loadGenConfig() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestLoadActivityConfig_yaml(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		yaml    string
		wantDir string
		want    []string
	}{
		{
			name:    "READMEの例の場合_ホームを除外しdirを展開すること",
			yaml:    "gen:\n  activity:\n    dir: ~/act\n    ignore: [\"~\"]\n",
			wantDir: filepath.Join(home, "act"),
			want:    []string{home},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			viper.SetConfigType("yaml")
			if err := viper.ReadConfig(strings.NewReader(tt.yaml)); err != nil {
				t.Fatal(err)
			}
			got, err := loadActivityConfig()
			if err != nil {
				t.Fatal(err)
			}
			if got.Dir != tt.wantDir || !slices.Equal(got.Ignore, tt.want) {
				t.Errorf("loadActivityConfig() = %+v, want dir %q ignore %v", got, tt.wantDir, tt.want)
			}
		})
	}
}
