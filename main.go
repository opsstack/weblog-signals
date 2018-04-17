package main

// TODO
// If last run time, maybe store inode, etc. to know to restart counters
// Change output stats to all floats
// Attribute lex code

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	flag "github.com/ogier/pflag"
)

// constants
const (
	version   = "0.1"
	copyright = "Copyright 2018 by OpsStack"
)

// Global vars
var (
	argLogName        string
	argStatsMetric    string
	argExcludes       string
	argStatusFileName string
	flagBeginning     bool
	flagVerbose       bool
	flagVeryVerbose   bool
	flagHelp          bool
)

// init is called automatically at start
func init() {

	// Setup arguments, must do before calling Parse()
	flag.StringVarP(&argLogName, "logname", "f", "", "Log File Name")
	flag.StringVarP(&argStatsMetric, "metric", "m", "c", "Metric Type")
	flag.StringVarP(&argExcludes, "exclues", "e", "", "Exclude Pattern")
	flag.StringVarP(&argStatusFileName, "statusfile", "s", "", "Status File")
	flag.BoolVarP(&flagBeginning, "beginning", "b", false, "From Beginning")
	flag.BoolVarP(&flagVerbose, "verbose", "v", false, "Verbose Output")
	flag.BoolVarP(&flagVeryVerbose, "very-verbose", "w", false, "Very Verbose Output")
	flag.BoolVarP(&flagHelp, "help", "h", false, "Help")

	flag.Parse() // Process argurments
}

func main() {

	var (
		logFilename    string
		infoFileName   string
		err            error
		f              io.Reader
		lastRunInfo    [4]int64
		lastRunLine    int64
		currentRunline int64
		duration       int64
		rate           int64
		//		fstat          os.FileInfo
	)

	startTime := time.Now()

	if flagVerbose {
		fmt.Println("")
		fmt.Printf("WebLogMetrics Version %s - %s\n", version, copyright)
		fmt.Printf("Starting at: %s \n", startTime.Format(time.UnixDate))
		fmt.Printf("Arguments: %s \n\n", os.Args[1:]) // Skip program name
	}

	// Check our command-line arguments
	argsCheck(version, copyright)

	// Get our last run line and time from status file
	infoFileName = argStatusFileName
	lastRunLine, lastRunInfo, err = getLastRunInfo(infoFileName)

	logFilename = argLogName

	f, err = os.Open(logFilename)

	if err != nil {
		msg := fmt.Sprintf("Log file Error on file: %s", logFilename)
		log.Fatal(msg)
		panic("")
	}

	scanner := bufio.NewScanner(f)

	// line scanner
	var (
		l                 *Entry
		line              string
		counter           int64 = 0
		counter200Lines   int64 = 0
		counter300Lines   int64 = 0
		counter400Lines   int64 = 0
		counterErrorLines int64 = 0
		counterExcluded   int64 = 0
		sumResponseTime   float64
		avgResponseTime   float64
		firstLineTime     time.Time
		lastLineTime      time.Time
	)

	// Skip previous run lines
	// Not efficient but works for now
	if !flagBeginning {
		var i int64
		var ok bool
		startSkipTime := time.Now()
		for i = 0; i < lastRunLine; i++ {
			ok = scanner.Scan()
			// Check if we hit end of file; zero out counter; not sure what else to do herer
			if !ok {
				err = scanner.Err()
				check(err)
				if err == nil { // We ran off end of file
					fmt.Println("Reached end of file before we started reading for this run")
					lastRunLine = 0
				}
			}
		}
		skipTime := time.Now().Sub(startSkipTime) / time.Millisecond
		if flagVerbose {
			fmt.Printf("Skip count: %d \n", i)
			fmt.Printf("Skip time (ms): %d \n", skipTime)
		}
	} // Skip lines

	// Sacn Good Lines
	for scanner.Scan() {
		counter++                                // So we are processing line 1 on first line, etc.
		line = strings.TrimSpace(scanner.Text()) // Ensure trimmed for later splitting

		// Skip blank lines (causing errors at end) and Excluded Lines
		if len(string(line)) > 0 {

			if len(string(argExcludes)) == 0 ||
				(len(string(argExcludes)) > 0 && !strings.Contains(line, argExcludes)) {

				l, err = Combined(line)
				// This will abort on any error; may want to be more forgiving here and just go to next line
				//check(err)
				if err != nil {
					continue
				}

				// Get the last value
				fields := strings.Fields(line) // Split on spaces
				lastField := fields[len(fields)-1]
				l.ResponseTime, err = strconv.ParseFloat(lastField, 64)
				check(err)

				// Extract first timestamp
				if firstLineTime.IsZero() && !l.Time.IsZero() {
					firstLineTime = l.Time
				}

				switch {
				case l.Status == 200:
					counter200Lines++
					sumResponseTime += l.ResponseTime
				case l.Status >= 300 && l.Status < 399:
					counter300Lines++
				case l.Status >= 400 && l.Status < 499:
					counter400Lines++
				case l.Status >= 500 && l.Status < 599:
					counterErrorLines++
				}

			} else {
				counterExcluded++
			}
		} // Non-blank lines
	} // Good line scanner loop

	if l != nil { //Needed if we never read any lines due to empty file or skip
		lastLineTime = l.Time
	}

	if flagVerbose {
		fmt.Printf("Scanned count: %d \n", counter)
		fmt.Printf("Excluded count: %d \n", counterExcluded)
		fmt.Printf("Count Status 200: %d\n", counter200Lines)
		fmt.Printf("Count Status 3xx: %d\n", counter300Lines)
		fmt.Printf("Count Status 4xx: %d\n", counter400Lines)
		fmt.Printf("Count Errors (5xx): %d\n", counterErrorLines)
		fmt.Printf("First Scanned line time: %s\n", firstLineTime.Format(time.UnixDate))
		fmt.Printf("Last Scanned line time: %s\n\n", lastLineTime.Format(time.UnixDate))
	}

	// Update run status info to file
	//endTimeStamp := time.Now().Unix() // Epoch time
	if flagBeginning { // Don't use last run if starting over
		currentRunline = counter
	} else {
		currentRunline = lastRunLine + counter
	}

	err = saveLastRunInfo(infoFileName, currentRunline, lastRunInfo)
	check(err)

	// Statistics
	duration = int64(lastLineTime.Sub(firstLineTime).Seconds())

	if counter200Lines > 0 && duration > 0 {
		avgResponseTime = sumResponseTime / float64(counter200Lines)
		rate = counter200Lines / duration
	} else {
		avgResponseTime = 0
		rate = 0
	}

	// Output
	if flagVerbose {
		fmt.Printf("Duration: %d\n", duration)
	}

	switch argStatsMetric {
	case "c":
		if flagVerbose {
			fmt.Printf("Count Status 200: ")
		}
		fmt.Printf("%d\n", counter200Lines)

	case "r":
		if flagVerbose {
			fmt.Printf("Rate: ")
		}
		fmt.Printf("%d", rate)
		if flagVerbose {
			fmt.Printf("/sec\n")
		} else {
			fmt.Printf("\n")
		}

	case "e":
		if flagVerbose {
			fmt.Print("Errors 5xx: ")
		}
		fmt.Printf("%d\n", counterErrorLines)

	case "l":
		if flagVerbose {
			fmt.Printf("Avg Response Time: ")
		}
		fmt.Printf("%f", avgResponseTime)
		if flagVerbose {
			fmt.Printf(" ms\n")
		} else {
			fmt.Printf("\n")
		}
	default:
		panic("Invdalid Stats Metric in main")
	} // Switch on argStatsMetric

	runTime := time.Now().Sub(startTime) / time.Millisecond
	if flagVerbose {
		fmt.Printf("\n")
		fmt.Printf("Logged timestamp between runs: %d\n", duration)
		fmt.Printf("Total exec time (ms): %d \n\n", runTime)
	}

	os.Exit(1)
} // Main

// Process arguments
func argsCheck(version string, copyright string) {

	if flagHelp {
		fmt.Printf("GoldenWebReader Version %s - %s\n\n", version, copyright)
		fmt.Printf("Usage: %s [options]\n\n", os.Args[0])
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Require metric type
	if argStatsMetric == "" {
		log.Fatalln("Stats Metric missing - should be c, e, l")
		os.Exit(1)
	}
	if argStatsMetric != "c" && argStatsMetric != "r" && argStatsMetric != "e" && argStatsMetric != "l" {
		log.Fatalln("Stats Metric not valid - should be c, e, l")
		os.Exit(1)
	}

	if argLogName == "" {
		log.Fatalln("No log file name supplied on command line; use -f option.")
		os.Exit(1)
	}

	// Just in case we allow quoted arguments with trailing spaces
	if len(string(argExcludes)) > 0 {
		argExcludes = strings.TrimSpace(argExcludes)
	}
}

func check(e error) {
	if e != nil {
		log.Fatal(e)
		panic(e)
	}
}

type lastRunStruct struct {
	countLastRunTime   int64
	countLastRunLine   int64
	errorLastRunTime   int64
	errorLastRunLine   int64
	latencyLastRunTime int64
	latencyLastRunLine int64
}
