// Package config reads the his config file through the global viper instance.
package config

import (
	"errors"
	"fmt"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	"github.com/tro3373/his/internal/activity"
	"github.com/tro3373/his/internal/gen"
)

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
	dir, err := ExpandTilde(a.Dir)
	if err != nil {
		return err
	}
	a.Dir = dir
	for i := range a.Ignore {
		ignored, err := ExpandTilde(a.Ignore[i])
		if err != nil {
			return err
		}
		a.Ignore[i] = activity.PhysicalPath(ignored)
	}
	return nil
}

// Activity reads only gen.activity over its defaults and does not validate,
// so a shell hook records activity whatever the rest of gen holds.
func Activity() (gen.ActivityConfig, error) {
	cfg := gen.DefaultConfig().Activity
	dropIgnoreDefault(&cfg)
	if err := viper.UnmarshalKey("gen.activity", &cfg, configDecodeHook()); err != nil {
		return cfg, fmt.Errorf("failed to read the gen.activity config: %w", err)
	}
	err := expandActivity(&cfg)
	return cfg, err
}

// Gen reads the gen section over its defaults, expands "~" in every path and validates it.
func Gen() (gen.Config, error) {
	cfg := gen.DefaultConfig()
	dropIgnoreDefault(&cfg.Activity)
	if err := viper.UnmarshalKey("gen", &cfg, configDecodeHook()); err != nil {
		return cfg, fmt.Errorf("failed to read the gen config: %w", err)
	}
	if err := expandActivity(&cfg.Activity); err != nil {
		return cfg, err
	}
	expand := func(p *string) error {
		v, err := ExpandTilde(*p)
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
