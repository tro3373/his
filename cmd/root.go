package cmd

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "his",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		if err := runInner(cmd, args); err != nil {
			log.Fatalf("==> Err:%+v\n", err)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.his.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	rootCmd.Flags().StringP("tag", "t", "", "tag filter")
	rootCmd.Flags().BoolP("detail", "d", false, "ditail(tag+titile) summary mode (default off)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".his" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".his")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
func runInner(cmd *cobra.Command, args []string) error {
	tag, err := cmd.Flags().GetString("tag")
	if err != nil {
		return err
	}
	detail, err := cmd.Flags().GetBool("detail")
	if err != nil {
		return err
	}
	fmt.Println("tag, detail, args==>", tag, detail, args)

	// if len(args) == 0 {
	// 	latestCmd.Run(cmd, args)
	// }
	tag, outputCount, err := parseArgs(args)
	if err != nil {
		return err
	}
	fmt.Printf("tag==>%#+v\n", tag)
	if true {
		// TODO
		return err
	}

	dateLogs, err := getDateLogs(2) // always load 2 file
	if err != nil {
		return err
	}

	count := 0
	for _, dateLog := range dateLogs {
		count++
		if count > outputCount {
			break
		}
		if detail {
			dumpDetail(dateLog)
			continue
		}
		dumpSummary(dateLog)
	}
	return nil
}

func parseArgs(args []string) (string, int, error) {
	// errSpecifyTag := errors.Errorf("Error: %s", "Specify tag name")
	var errSpecifyTag error
	if len(args) == 0 {
		return "", 0, errSpecifyTag
	}

	tag := ""
	count := 1
	for idx, arg := range args {
		log.Printf("==> parseArgs: %d:%s\n", idx, arg)
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

func dumpSummary(dateLog *DateLog) {
	m := summaryTag(dateLog.TimeLogs)
	dumpSummaryTag(dateLog.Date, m)
}

func dumpDetail(dateLog *DateLog) {
	m := summaryTag(dateLog.TimeLogs)
	dumpSummaryTag(dateLog.Date, m)
}
