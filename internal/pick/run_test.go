package pick

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tro3373/his/internal/gen"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name        string
		interactive bool
		fzfOut      string
		fzfCode     string
		wantList    string
		wantEdited  bool
		wantErr     string
		noLogs      bool
	}{
		{name: "端末でない場合_pathを1行ずつ出しfzfを起動しないこと", wantList: "{path}\n"},
		{name: "端末で選んだ場合_editorで開くこと", interactive: true, fzfOut: "{path}\tjob\t2026/2026-11-02-log.md\n", fzfCode: "0", wantEdited: true},
		{name: "端末で何も選ばなかった場合_editorを起動しないこと", interactive: true, fzfCode: "130"},
		{name: "端末で月のログが無い場合_エラーになること", interactive: true, noLogs: true, wantErr: "no month log found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "job")
			path := filepath.Join(dir, "2026/2026-11-02-log.md")
			if !tt.noLogs {
				touch(t, path)
			}
			bin := t.TempDir()
			record := filepath.Join(t.TempDir(), "edited")
			writeScript(t, bin, "fzf", `cat > /dev/null
printf '%s' "$FAKE_OUT"
exit "${FAKE_CODE:-0}"
`)
			writeScript(t, bin, "nvim", `printf '%s\n' "$@" > "$RECORD"`+"\n")
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("EDITOR", "")
			t.Setenv("RECORD", record)
			t.Setenv("FAKE_OUT", strings.ReplaceAll(tt.fzfOut, "{path}", path))
			t.Setenv("FAKE_CODE", tt.fzfCode)

			var list bytes.Buffer
			err := Run([]gen.LogConfig{{Name: "job", Dir: dir}}, tt.interactive, &list)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Run() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if want := strings.ReplaceAll(tt.wantList, "{path}", path); list.String() != want {
				t.Errorf("list = %q, want %q", list.String(), want)
			}
			data, readErr := os.ReadFile(record)
			if edited := readErr == nil; edited != tt.wantEdited {
				t.Errorf("editor started = %v, want %v (args %q)", edited, tt.wantEdited, data)
			}
			if tt.wantEdited && strings.TrimSpace(string(data)) != path {
				t.Errorf("editor args = %q, want %q", data, path)
			}
		})
	}
}
