package cmd

import (
	"path/filepath"
	"testing"
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
			got, err := expandTilde(tt.in)
			if err != nil || got != tt.want {
				t.Errorf("expandTilde(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
			}
		})
	}
}
