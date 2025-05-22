package main

import (
	"github.com/brunolkatz/goprotos7"
	"log"
	"os"
)

func init() {
	// set the log default values to show the time and the log level
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {

	args := parseArgs(os.Args[1:])

	var opts []goprotos7.ServerOption
	for arg, value := range args {
		switch arg {
		case "b", "bin-folder":
			opts = append(opts, goprotos7.WithBinFilesFolder(value))
		}
	}

	s, _ := goprotos7.New(opts...)
	if err := s.Start(); err != nil {
		panic(err)
	}
	return
}

var (
	ArgsFlagsHelper = map[string]string{
		"--help":       "Show this help message",
		"--version":    "Show the version",
		"-b":           "Bin files folder",
		"--bin-folder": "Bin files folder",
	}
)

func parseArgs(args []string) map[string]string {
	result := make(map[string]string)

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Check if it starts with "-" or "--"
		if len(arg) > 1 && arg[0] == '-' {
			// Remove leading "-" or "--"
			key := arg
			for key[0] == '-' {
				key = key[1:]
			}

			// Check if next argument exists and is not another flag
			if i+1 < len(args) && (len(args[i+1]) == 0 || args[i+1][0] != '-') {
				result[key] = args[i+1]
				i++ // Skip the value
			} else {
				// Boolean flag (just present, no value)
				result[key] = "true"
			}
		}
	}

	return result
}
