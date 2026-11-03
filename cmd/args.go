package cmd

import "strconv"

// parseTagArgs reads the positional args of latest and tag:
// a number is the count, anything else the tag. Later args win.
func parseTagArgs(args []string, defCount int) (string, int) {
	tag := ""
	count := defCount
	for _, arg := range args {
		if n, err := strconv.Atoi(arg); err == nil {
			count = n
			continue
		}
		tag = arg
	}
	return tag, count
}
