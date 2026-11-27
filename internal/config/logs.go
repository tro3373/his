package config

import (
	"fmt"

	"github.com/spf13/viper"
	"github.com/tro3373/his/internal/gen"
)

// Logs reads and validates only gen.logs, "~" expanded, and does not validate the rest of gen.
func Logs() ([]gen.LogConfig, error) {
	var logs []gen.LogConfig
	if err := viper.UnmarshalKey("gen.logs", &logs, configDecodeHook()); err != nil {
		return nil, fmt.Errorf("failed to read the gen.logs config: %w", err)
	}
	if err := gen.ValidateLogs(logs); err != nil {
		return nil, fmt.Errorf("invalid gen.logs config (see `his gen --help`): %w", err)
	}
	if err := expandLogs(logs); err != nil {
		return nil, err
	}
	return logs, nil
}
