package cmd

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/tro3373/his/internal/config"
	"github.com/tro3373/his/internal/summary"
)

var tagCmd = &cobra.Command{
	Use:   "tag [TAG] [COUNT]",
	Short: "Show the latest days of the time log summed by tag and title",
	Long: `Show the most recent days (default 14) of the time logs found by the pattern in the config, summed by tag and title.
A number is the count of days, anything else filters by tag.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := tag(args); err != nil {
			log.Fatalf("==> Failed to execute subcommand tag. Err:%+v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(tagCmd)
}

func tag(args []string) error {

	tag, outputCount := parseTagArgs(args, 14)

	pattern, err := config.Pattern()
	if err != nil {
		return err
	}
	result, err := summary.Analyze(pattern, 2) // always load 2 file
	if err != nil {
		return err
	}
	result.PrintTagTitleResult(tag, outputCount)

	return nil
}
