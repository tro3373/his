package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tro3373/his/internal/activity"
)

var actCmd = &cobra.Command{
	Use:   "act",
	Short: "Append the current directory and branch to the Activity log",
	Long: `Append one line to the Activity log: the time, the working directory and its git branch.
The command being run is never recorded.
Shell hooks and editors call this so that work outside Claude Code is not read as a break
(see "his hook"). The log's place is gen.activity in the config.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runAct(time.Now()); err != nil {
			log.Fatalf("==> Failed to execute act. Err:%+v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(actCmd)
}

func runAct(now time.Time) error {
	cfg, err := loadActivityConfig()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get the working directory: %w", err)
	}
	return activity.Append(cfg.Dir, activity.Entry{At: now, Cwd: physicalPath(cwd), Branch: gitBranch(cwd)})
}

// physicalPath resolves symlinks in dir, or returns dir unchanged when it cannot.
// Claude Code records physical paths: a logical one would split a repository into two tags.
func physicalPath(dir string) string {
	if dir == "" {
		return dir
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return dir
	}
	return resolved
}

// gitBranch is the branch checked out in dir, or "" when dir is no repository or HEAD is detached.
func gitBranch(dir string) string {
	out, err := exec.Command("git", "-C", dir, "branch", "--show-current").Output() //nolint:gosec // dir is our own working directory and the arguments are fixed
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
