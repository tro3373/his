package pick

import (
	"fmt"
	"os"
	"os/exec"
)

const defaultEditor = "nvim"

// Edit opens paths in editor with the terminal attached.
// editor is a shell command that may carry arguments ("nvim -p"); empty means nvim.
func Edit(editor string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	if editor == "" {
		editor = defaultEditor
	}
	args := append([]string{"-c", editor + ` "$@"`, "sh"}, paths...)
	cmd := exec.Command("sh", args...) //nolint:gosec // the editor command is the user's own $EDITOR
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run the editor %q: %w", editor, err)
	}
	return nil
}
