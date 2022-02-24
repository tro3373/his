/*
Copyright (c) 2022 tro3373 <tro3373@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func do() error {
	file, err := findLatestMd()
	if err != nil {
		return err
	}
	parseFile(file)
	return nil
}

func findLatestMd() (string, error) {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	pattern := fmt.Sprintf("%s/works/00_memos/*月.md", userHomeDir)
	files, err := filepath.Glob(pattern)
	sort.Slice(files, func(i, j int) bool {
		return files[i] > files[j]
	})
	for _, f := range files {
		// log.Printf("f:%s\n", f)
		return f, nil
	}
	return "", errors.New(fmt.Sprintf("Error: %s", "No such md exist."))
}

func parseFile(file string) error {
	fp, err := os.Open(file)
	if err != nil {
		return err
	}
	defer fp.Close()

	scanner := bufio.NewScanner(fp)
	for scanner.Scan() {
		log.Println(scanner.Text())
	}
	log.Printf(file)

	log.Printf("%d", time.Now().Unix())

	return nil
}

type TimeLog struct {
	date string
}
