package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var hookCmd = &cobra.Command{
	Use:   "hook zsh",
	Short: "Print the shell code that records activity with \"his act\"",
	Long: `Print shell code that runs "his act" in the background on every prompt,
command and directory change. Only zsh is supported.

  eval "$(his hook zsh)"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if args[0] != "zsh" {
			return fmt.Errorf("unsupported shell %q: only zsh is supported", args[0])
		}
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to find the his executable: %w", err)
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), zshHook(exe))
		return err
	},
}

func init() {
	rootCmd.AddCommand(hookCmd)
}

// zshHook is the zsh code that runs "<exe> act" in the background from precmd, preexec and chpwd.
// Sourcing it again registers nothing twice.
// The subshell keeps the background job from overwriting $! in the user's shell.
// no_monitor is required: with job control on, the subshell makes the next fzf
// started inside $(...) fail with "input/output error".
func zshHook(exe string) string {
	quoted := "'" + strings.ReplaceAll(exe, "'", `'\''`) + "'"
	var b strings.Builder
	fmt.Fprintf(&b, "_his_act() { setopt local_options no_monitor; ( %s act >/dev/null 2>&1 &! ) }\n", quoted)
	for _, hook := range []string{"precmd", "preexec", "chpwd"} {
		fmt.Fprintf(&b, "(( ${%[1]s_functions[(I)_his_act]} )) || %[1]s_functions+=(_his_act)\n", hook)
	}
	return b.String()
}
