package cmd

import (
	"log"
	"strconv"

	"github.com/spf13/cobra"
)

// latestCmd represents the latest command
var latestCmd = &cobra.Command{
	Use:   "latest",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := latest(args); err != nil {
			log.Fatalf("==> Failed to execute latest. Err:%+v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(latestCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// latestCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// latestCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func latest(args []string) error {

	outputCount := 1
	if len(args) != 0 {
		arg := args[0]
		tmp, err := strconv.Atoi(arg)
		if err == nil {
			outputCount = tmp
		}
	}

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
