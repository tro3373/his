package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestZshHook(t *testing.T) {
	tests := []struct {
		name string
		exe  string
		want []string
	}{
		{
			name: "通常のパスの場合_引用して関数を定義し3つのフックに登録すること",
			exe:  "/usr/bin/his",
			want: []string{
				"_his_act() { ( '/usr/bin/his' act >/dev/null 2>&1 &! ) }",
				"precmd_functions+=(_his_act)", "preexec_functions+=(_his_act)", "chpwd_functions+=(_his_act)",
			},
		},
		{
			name: "パスに引用符を含む場合_シェル用にエスケープすること",
			exe:  "/tmp/it's/his",
			want: []string{`'/tmp/it'\''s/his' act`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := zshHook(tt.exe)
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("zshHook() lacks %q:\n%s", w, got)
				}
			}
		})
	}
}

func TestZshHook_zsh(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls")
	exe := filepath.Join(dir, "it's his")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\necho \"$@\" >> "+logPath+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	hook := zshHook(exe)
	script := "eval " + shellQuote(hook) + "\neval " + shellQuote(hook) + "\n" +
		"for h in precmd preexec chpwd; do a=(${(M)${(P)${:-${h}_functions}}:#_his_act}); print -n -- \"$#a \"; done\n" +
		"_his_act\nwait\n"
	cmd := exec.Command(zsh, "-f", "-c", script)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("zsh failed: %v\n%s", err, out.String())
	}
	if got := strings.TrimSpace(out.String()); got != "1 1 1" {
		t.Errorf("registrations after sourcing twice = %q, want %q", got, "1 1 1")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		data, _ := os.ReadFile(logPath)
		if strings.TrimSpace(string(data)) == "act" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the hook did not run `act`: %q", data)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func TestHookCmd_shell(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "zshの場合_エラーにならないこと", args: []string{"zsh"}},
		{name: "bashの場合_エラーになること", args: []string{"bash"}, wantErr: true},
		{name: "引数が無い場合_エラーになること", wantErr: true},
		{name: "引数が2つの場合_エラーになること", args: []string{"zsh", "bash"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			hookCmd.SetOut(&out)
			t.Cleanup(func() { hookCmd.SetOut(nil) })
			err := hookCmd.Args(hookCmd, tt.args)
			if err == nil {
				err = hookCmd.RunE(hookCmd, tt.args)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
