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

// mapstructure decodes into an existing non-nil slice element by element,
// so a configured list shorter than the default would keep the default's tail.
func dropIgnoreDefault(a *gen.ActivityConfig) {
	if viper.IsSet("gen.activity.ignore") {
		a.Ignore = nil
	}
}

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

// Activity reads only gen.activity over its defaults and does not validate the rest of gen.
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
	if err := errors.Join(expandPath(&cfg.StoreRoot), expandPath(&cfg.RecDir), expandPaths(cfg.Sessions)); err != nil {
		return cfg, err
	}
	if err := expandLogs(cfg.Logs); err != nil {
		return cfg, err
	}
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("invalid gen config (see `his gen --help`): %w", err)
	}
	return cfg, nil
}

func expandLogs(logs []gen.LogConfig) error {
	for i := range logs {
		if err := errors.Join(expandPath(&logs[i].Dir), expandPaths(logs[i].Roots)); err != nil {
			return err
		}
	}
	return nil
}

func expandPaths(paths []string) error {
	for i := range paths {
		if err := expandPath(&paths[i]); err != nil {
			return err
		}
	}
	return nil
}

func expandPath(p *string) error {
	v, err := ExpandTilde(*p)
	*p = v
	return err
}
