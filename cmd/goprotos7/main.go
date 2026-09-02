package main

import (
	"fmt"
	"github.com/brunolkatz/goprotos7"
	"log"
	"os"
	"strconv"
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
		case "h", "help":
			log.Println("Usage: goprotos7 [options]")
			log.Println("Options:")
			for flag, description := range ArgsFlagsHelper {
				log.Println(fmt.Sprintf("  %s - %s", flag, description))
			}
			return
		case "b", "bin-folder":
			opts = append(opts, goprotos7.WithBinFilesFolder(value))
		case "p", "port":
			port := 102 // Default port
			p, err := strconv.Atoi(value)
			if err != nil {
				opts = append(opts, goprotos7.WithPort(port))
			}
			if p < 1 || p > 65535 {
				log.Printf("[ERROR] Invalid port number: %s. Using default port %d.", value, port)
				opts = append(opts, goprotos7.WithPort(port))
			} else {
				opts = append(opts, goprotos7.WithPort(p))
			}
		case "start-local":
			opts = append(opts, goprotos7.WithTransport())
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
		"--help":        "Show this help message",
		"--version":     "Show the version",
		"-b":            "Bin files folder",
		"--bin-folder":  "Bin files folder",
		"-p":            "Port to listen on (default is 102)",
		"--port":        "Port to listen on (default is 102)",
		"--start-local": "Start the server on localhost (127.0.0.1), not visible in the local network",
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
