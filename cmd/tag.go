package cmd

import (
	"fmt"
	"log"
	"strconv"

	"github.com/spf13/cobra"
)

// tagCmd represents the tag command
var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := tag(args); err != nil {
			log.Fatalf("==> Failed to execute subcommand tag. Err:%+v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(tagCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// tagCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// tagCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func tag(args []string) error {

	tag, outputCount := parseTagArgs(args)

	dateLogs, err := collectDateLogs(2) // always load 2 file
	if err != nil {
		return err
	}

	var timeLogs []*TimeLog
	count := 0
	for _, dateLog := range dateLogs {
		count++
		if count > outputCount {
			break
		}
		timeLogs = append(timeLogs, dateLog.TimeLogs...)
	}

	summaryTimeLogs := summaryTimeLog(timeLogs, true)
	for _, timeLog := range summaryTimeLogs {
		if len(timeLog.Tag) == 0 {
			continue
		}
		if len(tag) != 0 && tag != timeLog.Tag {
			continue
		}
		fmt.Printf(
			"%s\t%s\t%s\t%s\n",
			timeLog.Date,
			SummaryTimeString(timeLog.Summary),
			timeLog.Tag,
			timeLog.Title,
		)
	}
	return nil
}

func parseTagArgs(args []string) (string, int) {
	// default
	tag := ""
	count := 14

	if len(args) == 0 {
		return tag, count
	}

	for _, arg := range args {
		tmp, err := strconv.Atoi(arg)
		if err == nil {
			count = tmp
			continue
		}
		tag = arg
	}
	return tag, count
}
