// Copyright 2013 Frederik Zipp. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Gocyclo calculates the cyclomatic complexities of functions and
// methods in Go source code.
//
// Usage:
//
//	gocyclo [<flag> ...] <Go file or directory> ...
//
// Flags:
//
//	-over N               show functions with complexity > N only and
//	                      return exit code 1 if the output is non-empty
//	-top N                show the top N most complex functions only
//	-avg, -avg-short      show the average complexity;
//	                      the short option prints the value without a label
//	-ignore REGEX         exclude files matching the given regular expression
//
// The output fields for each line are:
// <complexity> <package> <function> <file:line:column>
//
// The exit code is 2 if a path could not be analyzed (it does not exist, a
// directory cannot be read, a file cannot be parsed) or the flags are
// invalid, 1 if -over found functions, and 0 otherwise. The functions that
// could be analyzed are reported even when the exit code is 2.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/fzipp/gocyclo"
)

// Exit codes. exitError is distinct from exitOver so that a script can tell
// "the code is too complex" apart from "the code could not be analyzed".
const (
	exitOver  = 1
	exitError = 2
)

const usageDoc = `Calculate cyclomatic complexities of Go functions.
Usage:
    gocyclo [flags] <Go file or directory> ...

Flags:
    -over N               show functions with complexity > N only and
                          return exit code 1 if the set is non-empty
    -top N                show the top N most complex functions only
    -avg, -avg-short      show the average complexity over all functions;
                          the short option prints the value without a label
    -ignore REGEX         exclude files matching the given regular expression

The output fields for each line are:
<complexity> <package> <function> <file:line:column>

The exit code is 2 if a path could not be analyzed or the flags are invalid,
1 if -over found functions, and 0 otherwise.
`

func main() {
	over := flag.Int("over", 0, "show functions with complexity > N only")
	top := flag.Int("top", -1, "show the top N most complex functions only")
	avg := flag.Bool("avg", false, "show the average complexity")
	avgShort := flag.Bool("avg-short", false, "show the average complexity without a label")
	ignore := flag.String("ignore", "", "exclude files matching the given regular expression")

	log.SetFlags(0)
	log.SetPrefix("gocyclo: ")
	flag.Usage = usage
	flag.Parse()
	paths := flag.Args()
	if len(paths) == 0 {
		usage()
	}

	allStats, err := gocyclo.AnalyzePaths(paths, regex(*ignore))
	shownStats := allStats.SortAndFilter(*top, *over)

	printStats(shownStats)
	if *avg || *avgShort {
		printAverage(allStats, *avgShort)
	}

	if err != nil {
		// errors.Join separates the problems with newlines; give each one
		// its own prefixed line.
		for _, line := range strings.Split(err.Error(), "\n") {
			log.Print(line)
		}
		os.Exit(exitError)
	}
	if *over > 0 && len(shownStats) > 0 {
		os.Exit(exitOver)
	}
}

func regex(expr string) *regexp.Regexp {
	if expr == "" {
		return nil
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		log.Print(err)
		os.Exit(exitError)
	}
	return re
}

func printStats(s gocyclo.Stats) {
	stdout := bufio.NewWriter(os.Stdout)
	defer stdout.Flush()

	for _, stat := range s {
		fmt.Fprintln(stdout, stat)
	}
}

func printAverage(s gocyclo.Stats, short bool) {
	if !short {
		fmt.Print("Average: ")
	}
	fmt.Printf("%.3g\n", s.AverageComplexity())
}

func usage() {
	_, _ = fmt.Fprint(os.Stderr, usageDoc)
	os.Exit(2)
}
