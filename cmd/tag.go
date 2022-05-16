package cmd

import (
	"fmt"
	"log"
	"strconv"

	"github.com/pkg/errors"
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

	tag, outputCount, err := parseTagArgs(args)
	if err != nil {
		return err
	}
	fmt.Printf("%#+v\n", tag)

	dateLogs, err := getDateLogs(2) // always load 2 file
	if err != nil {
		return err
	}

	count := 0
	for _, dl := range dateLogs {
		count++
		if count > outputCount {
			break
		}
		m := summaryTag(dl.TimeLogs)
		dumpSummaryTag(dl.Date, m)
	}
	return nil
}

func parseTagArgs(args []string) (string, int, error) {
	errSpecifyTag := errors.Errorf("Error: %s", "Specify tag name")
	if len(args) == 0 {
		return "", 0, errSpecifyTag
	}

	tag := ""
	count := 1
	for _, arg := range args {
		tmp, err := strconv.Atoi(arg)
		if err == nil {
			count = tmp
			continue
		}
		tag = arg
	}
	if len(tag) == 0 {
		return tag, count, errSpecifyTag
	}

	return tag, count, nil
}
