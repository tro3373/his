package pick

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestEdit(t *testing.T) {
	tests := []struct {
		name     string
		editor   string
		onPath   string
		paths    []string
		wantArgs []string
		wantRun  bool
	}{
		{name: "引数付きのeditorの場合_引数とpathを渡すこと", editor: "{bin} -p", paths: []string{"/a/x.md", "/b/y.md"}, wantArgs: []string{"-p", "/a/x.md", "/b/y.md"}, wantRun: true},
		{name: "空白を含むpathの場合_1つの引数で渡すこと", editor: "{bin}", paths: []string{"/a b/x.md"}, wantArgs: []string{"/a b/x.md"}, wantRun: true},
		{name: "editorが空の場合_nvimを起動すること", onPath: "nvim", paths: []string{"/a/x.md"}, wantArgs: []string{"/a/x.md"}, wantRun: true},
		{name: "pathが無い場合_editorを起動しないこと", editor: "{bin}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bin := t.TempDir()
			record := filepath.Join(t.TempDir(), "args")
			name := "editor"
			if tt.onPath != "" {
				name = tt.onPath
			}
			script := writeScript(t, bin, name, `printf '%s\n' "$@" > "$RECORD"`+"\n")
			t.Setenv("RECORD", record)
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

			if err := Edit(strings.ReplaceAll(tt.editor, "{bin}", script), tt.paths); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(record)
			if os.IsNotExist(err) {
				if tt.wantRun {
					t.Fatal("editor was not started")
				}
				return
			}
			if !tt.wantRun {
				t.Fatalf("editor was started with %q", data)
			}
			got := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
			if !slices.Equal(got, tt.wantArgs) {
				t.Errorf("editor args = %q, want %q", got, tt.wantArgs)
			}
		})
	}
}

func TestEdit_failure(t *testing.T) {
	bin := t.TempDir()
	script := writeScript(t, bin, "editor", "exit 3\n")
	if err := Edit(script, []string{"/a"}); err == nil || !strings.Contains(err.Error(), "failed to run the editor") {
		t.Errorf("Edit() error = %v, want failed to run the editor", err)
	}
}
