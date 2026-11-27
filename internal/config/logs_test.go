package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestLogs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tests := []struct {
		name     string
		yaml     string
		wantDirs map[string]string
		wantErr  string
	}{
		{
			name:     "gen全体が不完全な場合_logsだけ読めること",
			yaml:     "gen:\n  logs:\n    - name: job\n      dir: ~/notes/job\n    - name: prv\n      dir: /abs/prv\n",
			wantDirs: map[string]string{"job": filepath.Join(home, "notes/job"), "prv": "/abs/prv"},
		},
		{name: "logsが空の場合_エラーになること", yaml: "gen:\n  lag: 1h\n", wantErr: "gen.logs is empty"},
		{name: "dirが無い場合_エラーになること", yaml: "gen:\n  logs:\n    - name: job\n", wantErr: `gen.logs "job" has no dir`},
		{name: "defaultが2つある場合_エラーになること", yaml: "gen:\n  logs:\n    - {name: job, dir: /j, default: true}\n    - {name: prv, dir: /p, default: true}\n", wantErr: "more than one gen.logs is default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			viper.SetConfigType("yaml")
			if err := viper.ReadConfig(strings.NewReader(tt.yaml)); err != nil {
				t.Fatal(err)
			}
			got, err := Logs()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Logs() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tt.wantDirs) {
				t.Fatalf("Logs() = %+v, want %d logs", got, len(tt.wantDirs))
			}
			for _, l := range got {
				if l.Dir != tt.wantDirs[l.Name] {
					t.Errorf("log %q dir = %q, want %q", l.Name, l.Dir, tt.wantDirs[l.Name])
				}
			}
		})
	}
}
