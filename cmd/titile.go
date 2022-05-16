package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

// titileCmd represents the titile command
var titileCmd = &cobra.Command{
	Use:   "titile",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := title(args); err != nil {
			log.Fatalf("==> Failed to execute subcommand tag. Err:%+v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(titileCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// titileCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// titileCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func title(args []string) error {

	// tag, outputCount, err := parseArgs(args)
	// fmt.Printf("%#+v\n", tag)
	//
	// dateLogs, err := getDateLogs(2) // always load 2 file
	// if err != nil {
	// 	return err
	// }
	//
	// // TODO implement
	// count := 0
	// for _, dl := range dateLogs {
	// 	count++
	// 	if count > outputCount {
	// 		break
	// 	}
	// 	m := summaryTag(dl.TimeLogs)
	// 	dumpSummaryTag(dl.Date, m)
	// }
	return nil
}

// func parseArgs(args []string) (string, int, error) {
// 	errSpecifyTag := errors.Errorf("Error: %s", "Specify tag name")
// 	if len(args) == 0 {
// 		return "", 0, errSpecifyTag
// 	}
//
// 	tag := ""
// 	count := 10
// 	for _, arg := range args {
// 		tmp, err := strconv.Atoi(arg)
// 		if err == nil {
// 			count = tmp
// 			continue
// 		}
// 		tag = arg
// 	}
// 	if len(tag) == 0 {
// 		return tag, count, errSpecifyTag
// 	}
//
// 	return tag, count, nil
// }
