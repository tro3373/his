package util

import (
	"strconv"
)

func ParseTagArgs(args []string, defCount int) (string, int) {
	// default
	tag := ""
	count := defCount

	if len(args) == 0 {
		return tag, count
	}

	for _, arg := range args {
		tmp, err := strconv.Atoi(arg)
		if err == nil {
			count = tmp
			continue
		}
		tag = arg
	}
	return tag, count
}
