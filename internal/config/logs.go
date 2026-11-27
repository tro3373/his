package config

import (
	"errors"
	"fmt"

	"github.com/spf13/viper"
	"github.com/tro3373/his/internal/gen"
)

// Logs reads only gen.logs, each Dir with "~" expanded, and does not validate the rest of gen.
func Logs() ([]gen.LogConfig, error) {
	var logs []gen.LogConfig
	if err := viper.UnmarshalKey("gen.logs", &logs, configDecodeHook()); err != nil {
		return nil, fmt.Errorf("failed to read the gen.logs config: %w", err)
	}
	if len(logs) == 0 {
		return nil, errors.New("gen.logs is empty (see `his gen --help`)")
	}
	for i := range logs {
		if logs[i].Dir == "" {
			return nil, fmt.Errorf("gen.logs %q has no dir", logs[i].Name)
		}
		dir, err := ExpandTilde(logs[i].Dir)
		if err != nil {
			return nil, err
		}
		logs[i].Dir = dir
	}
	return logs, nil
}
