package cmd

import (
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			want := gen.Config{Lag: tt.wantLag, CatchUpDays: tt.wantDays, MinIdle: tt.wantIdle}
			if got.Lag != want.Lag || got.CatchUpDays != want.CatchUpDays || got.MinIdle != want.MinIdle {
				t.Errorf("loadGenConfig() = %+v, want %+v", got, want)
			}
		})
	}
}
