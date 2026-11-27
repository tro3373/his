package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Pattern is the glob of the time log files, "~" expanded.
func Pattern() (string, error) {
	pattern := viper.GetString("pattern")
	if pattern == "" {
		pattern = "~/*.md"
	}
	return ExpandTilde(pattern)
}

// ExpandTilde replaces a leading "~" or "~/" with the home directory.
// "~user" is left as is.
func ExpandTilde(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, path[1:]), nil
}
