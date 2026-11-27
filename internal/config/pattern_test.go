package config

import (
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestExpandTilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "~だけの場合_ホームになること", in: "~", want: home},
		{name: "~/配下の場合_ホーム配下になること", in: "~/a/b", want: filepath.Join(home, "a/b")},
		{name: "~で始まる別名の場合_展開しないこと", in: "~user/a", want: "~user/a"},
		{name: "絶対パスの場合_そのままであること", in: "/a/~", want: "/a/~"},
		{name: "空の場合_空のままであること", in: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandTilde(tt.in)
			if err != nil || got != tt.want {
				t.Errorf("ExpandTilde(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
			}
		})
	}
}

func TestPattern(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tests := []struct {
		name string
		set  string
		want string
	}{
		{name: "未設定の場合_ホーム直下のmdになること", want: filepath.Join(home, "*.md")},
		{name: "~を使った場合_ホームに展開すること", set: "~/logs/*.md", want: filepath.Join(home, "logs/*.md")},
		{name: "絶対パスの場合_そのままであること", set: "/var/logs/*.md", want: "/var/logs/*.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			if tt.set != "" {
				viper.Set("pattern", tt.set)
			}
			got, err := Pattern()
			if err != nil || got != tt.want {
				t.Errorf("Pattern() = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}
