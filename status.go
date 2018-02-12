package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Fileformat is 4 lines of last log line read, one for each mode:
//	Count mode
//  Rate mode
//  Error mode
//  Latency mode

// Read last run info from saved file
func getLastRunInfo(statusFileName string) (int64, [4]int64, error) {

	var (
		f           io.Reader
		scanner     *bufio.Scanner
		err         error
		lastRunLine int64
		r           int
		last        [4]int64
	)

	if statusFileName == "" {
		statusFileName = "status.file"
	}

	f, err = os.Open(statusFileName)

	// If not exist or error, just ignore, hope we can create when done
	if err == nil {
		scanner = bufio.NewScanner(f)

		// Read values in loop
		for i := 0; i < 4; i++ {
			scanner.Scan()
			r, err = strconv.Atoi(strings.TrimSpace(scanner.Text()))
			check(err)
			last[i] = int64(r)
		}

		// Choose our values
		switch argStatsMetric {
		case "c":
			lastRunLine = last[0]
		case "r":
			lastRunLine = last[1]
		case "e":
			lastRunLine = last[2]
		case "l":
			lastRunLine = last[3]
		default:
			panic("Invdalid Stats Metric")
		}
	} else {
		if flagVerbose {
			fmt.Printf("Error opening status file: %s, ignoring.\n\n", statusFileName)
		}
		lastRunLine = 0 // Set zero if no file or error
	}
	// Returns last line for real use and last array so we can write unchanged data back out when done
	return lastRunLine, last, nil
}

// Save last run info to saved file
func saveLastRunInfo(statusFile string, lastRunLine int64, last [4]int64) (error) {
	var (
		fw  io.Writer
		err error
		s   string
	)

	if statusFile == "" {
		statusFile = "status.file"
	}
	fw, err = os.Create(statusFile)
	check(err)

	// Update data to change, leave rest unchanged so we can write back out to file
	switch argStatsMetric {
	case "c":
		last[0] = lastRunLine
	case "r":
		last[1] = lastRunLine
	case "e":
		last[2] = lastRunLine
	case "l":
		last[3] = lastRunLine
	default:
		panic("Invdalid Stats Metric in saveLastRunInfo")
	}

	// Loop writing
	for i := 0; i < 4; i++ {
		s = fmt.Sprintf("%d\n", last[i])
		_, err = io.WriteString(fw, s)
		check(err)
	}

	return nil
}
