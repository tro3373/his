package main

import (
	"fmt"
	"os"

	"github.com/tro3373/his"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}
}
