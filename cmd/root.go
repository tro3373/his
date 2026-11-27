package cmd

import (
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/tro3373/his/internal/config"
	"github.com/tro3373/his/internal/pick"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "his",
	Short: "Pick month logs in fzf and open them in $EDITOR",
	Long: `Pick month logs of the logs under gen.logs in fzf (several with tab) and open them in $EDITOR (default nvim).
The newest month comes first; the preview shows the end of the file.
When stdout is not a terminal, print the path of every month log instead.
To see the time summary, run "his latest" or "his tag".`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPick(); err != nil {
			log.Fatalf("==> Failed to pick a month log. Err:%+v\n", err)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error("Failed to execute root command", "error", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.his)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	setConfigSource()

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		slog.Debug("Using config", "file", viper.ConfigFileUsed())
	}
}

// setConfigSource points viper at the --config file, else at ".his" in the home directory.
func setConfigSource() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		return
	}
	home, err := homedir.Dir()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	viper.AddConfigPath(home)
	viper.SetConfigType("yaml")
	viper.SetConfigName(".his")
}

func runPick() error {
	logs, err := config.Logs()
	if err != nil {
		return err
	}
	return pick.Run(logs, isTerminal(os.Stdout), os.Stdout)
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
