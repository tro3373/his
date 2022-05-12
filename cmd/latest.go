package cmd

import (
	"fmt"
	"log"

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
		if err := latest(); err != nil {
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

func latest() error {
	dateLogs, err := getDateLogs(1)
	if err != nil {
		return err
	}
	dl := dateLogs[0]

	m := summaryTag(dl.TimeLogs)
	dumpSummaryTag(dl.Date, m)
	return nil
}

func summaryTag(timeLogs []*TimeLog) map[string]int64 {
	m := make(map[string]int64)
	// factories = make(map[string]Factory)
	tag := ""
	prev := int64(0)
	for _, tl := range timeLogs {
		if prev != 0 && tag != "" {
			summary := m[tag]
			add := tl.Start - prev
			summary += add
			m[tag] = summary
			// log.Printf("  Tag:%4s Add:%5d Sum:%6d Sta:%d End:%d Title:%s",
			// 	tag, add, summary, prev, tl.Start, tl.Title)
		}
		prev = tl.Start
		tag = tl.Tag
	}
	return m
}
func dumpSummaryTag(d string, m map[string]int64) {
	for tag, sec := range m {
		fmt.Printf("%s\t%s\t%3.0fm\t%02dh%02dm(+%02ds)\n",
			d,
			tag,
			float64(sec/60),
			sec/60/60,
			sec/60%60,
			sec%60,
		)
	}
}
