package cmd

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/tro3373/his/cmd/analyzer"
	"github.com/tro3373/his/cmd/util"
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

	tag, outputCount := util.ParseTagArgs(args, 14)

	pattern, err := getDefaultFindFilePattern()
	if err != nil {
		return err
	}
	result, err := analyzer.Analyze(pattern, 2) // always load 2 file
	result.PrintTagTitleResult(tag, outputCount)

	return nil
}
