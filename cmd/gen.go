package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	cfg, err := loadGenConfig()
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

func configDecodeHook() viper.DecoderConfigOption {
	return viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.TextUnmarshallerHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
	))
}

// dropIgnoreDefault empties the default ignore list when the config sets the key.
// mapstructure decodes into an existing non-nil slice element by element,
// so a configured list shorter than the default would keep the default's tail.
func dropIgnoreDefault(a *gen.ActivityConfig) {
	if viper.IsSet("gen.activity.ignore") {
		a.Ignore = nil
	}
}

// expandActivity expands "~" in the directory and every ignored directory.
// Ignored directories are resolved like the working directories "his act" records,
// or a symlinked home would never match.
func expandActivity(a *gen.ActivityConfig) error {
	dir, err := expandTilde(a.Dir)
	if err != nil {
		return err
	}
	a.Dir = dir
	for i := range a.Ignore {
		ignored, err := expandTilde(a.Ignore[i])
		if err != nil {
			return err
		}
		a.Ignore[i] = physicalPath(ignored)
	}
	return nil
}

// loadActivityConfig reads only gen.activity over its defaults and does not validate,
// so a shell hook records activity whatever the rest of gen holds.
func loadActivityConfig() (gen.ActivityConfig, error) {
	cfg := gen.DefaultConfig().Activity
	dropIgnoreDefault(&cfg)
	if err := viper.UnmarshalKey("gen.activity", &cfg, configDecodeHook()); err != nil {
		return cfg, fmt.Errorf("failed to read the gen.activity config: %w", err)
	}
	err := expandActivity(&cfg)
	return cfg, err
}

func loadGenConfig() (gen.Config, error) {
	cfg := gen.DefaultConfig()
	dropIgnoreDefault(&cfg.Activity)
	if err := viper.UnmarshalKey("gen", &cfg, configDecodeHook()); err != nil {
		return cfg, fmt.Errorf("failed to read the gen config: %w", err)
	}
	if err := expandActivity(&cfg.Activity); err != nil {
		return cfg, err
	}
	expand := func(p *string) error {
		v, err := expandTilde(*p)
		*p = v
		return err
	}
	if err := errors.Join(expand(&cfg.StoreRoot), expand(&cfg.RecDir)); err != nil {
		return cfg, err
	}
	for i := range cfg.Sessions {
		if err := expand(&cfg.Sessions[i]); err != nil {
			return cfg, err
		}
	}
	for i := range cfg.Logs {
		if err := expand(&cfg.Logs[i].Dir); err != nil {
			return cfg, err
		}
		for j := range cfg.Logs[i].Roots {
			if err := expand(&cfg.Logs[i].Roots[j]); err != nil {
				return cfg, err
			}
		}
	}
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("invalid gen config (see `his gen --help`): %w", err)
	}
	return cfg, nil
}
