package cmd

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/tro3373/his/internal/config"
	"github.com/tro3373/his/internal/summary"
)

var latestCmd = &cobra.Command{
	Use:     "latest [TAG] [COUNT]",
	Aliases: []string{"show"},
	Short:   "Show the latest days of the time log summed by tag",
	Long: `Show the most recent days (default 1) of the time logs found by the pattern in the config, summed by tag.
A number is the count of days, anything else filters by tag.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := latest(args); err != nil {
			log.Fatalf("==> Failed to execute latest. Err:%+v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(latestCmd)
}

func latest(args []string) error {
	tag, outputCount := parseTagArgs(args, 1)

	pattern, err := config.Pattern()
	if err != nil {
		return err
	}
	result, err := summary.Analyze(pattern, 2) // always load 2 file
	if err != nil {
		return err
	}
	result.PrintTagResult(tag, outputCount)

	return nil
}
