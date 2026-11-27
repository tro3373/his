package cmd

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/tro3373/his/internal/config"
	"github.com/tro3373/his/internal/gen"
)

var genCmd = &cobra.Command{
	Use:   "gen [YYYY-MM-DD]",
	Short: "Write the daily time headings from Claude Code sessions and recorded calls",
	Long: `Write the daily time headings of each log under the "gen" section of the config.

Without an argument, writes each log from yesterday, or from an older newest
day that already has a time heading, through today.
It starts catch_up_days back when no day has one.
A day ends no later than lag before now.
Days before the start are not touched.
A day is only added to: what follows its latest time heading is appended,
at most one append an hour, and earlier lines stay except that the
links of finished tasks get checked.
With a date, does that day only.

Config (~/.his):
  gen:
    store_root: ~/notes               # holds tasks/; links are relative to it
    rec_dir: ~/rec/out                # rec's output directory (REC_OUT_DIR)
    sessions:                         # every Claude Code config dir
      - ~/.claude/projects
      - ~/work/.claude/projects
    lunch: {start: "12:00", end: "13:00"}
    lag: 30m                          # optional; 0s still stops at the current quarter hour
    catch_up_days: 30                 # optional
    min_idle: 1h                      # optional; a shorter gap is not a break
    logs:
      - name: work
        dir: ~/notes/work
        roots: [~/work]               # sessions working under these
        apps: [slack, zoom]
      - name: personal
        dir: ~/notes/personal
        apps: [chrome]
        default: true                 # every other session and task

    activity:                         # optional; what "his act" appends to (see "his hook")
      dir: ~/.local/state/his/activity
      retention_days: 35              # at least catch_up_days; older day files are deleted
      ignore: ["~"]                   # working directories left out, matched exactly

A project is the JOB_TAG exported by the nearest .envrc above the session's
working directory ("export JOB_TAG=ACME"), else the repository name.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGen(args); err != nil {
			log.Fatalf("==> Failed to execute gen. Err:%+v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(genCmd)
}

func runGen(args []string) error {
	cfg, err := config.Gen()
	if err != nil {
		return err
	}
	now := time.Now()
	target, err := parseTarget(args, now)
	if err != nil {
		return err
	}
	return gen.Run(cfg, now, target, os.Stdout)
}

func parseTarget(args []string, now time.Time) (time.Time, error) {
	if len(args) == 0 {
		return time.Time{}, nil
	}
	t, err := time.ParseInLocation(time.DateOnly, args[0], now.Location())
	if err != nil {
		return time.Time{}, fmt.Errorf("date must be YYYY-MM-DD: %w", err)
	}
	return t, nil
}
