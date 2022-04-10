package main

import (
	"fmt"
	"os"
	"time"
	"strings"
)

type DataRequest struct {
	url  string
	from time.Time
	to   time.Time
}

// Get data in range: GetData <url> <from> <to>
// Get data all: GetData <url> <all>

func parseGetDataInRange(tokens []string) (bool, DataRequest) {
	requiredLength := 3
	if len(tokens) < requiredLength {
		return false, DataRequest {}
	}

	format := time.ANSIC

	from, err := time.Parse(format, tokens[1])
	if err != nil {
		return false, DataRequest {}
	}

	to, err := time.Parse(format, tokens[2])
	if err != nil {
		return false, DataRequest {}
	}

	return true, DataRequest { url: tokens[0], from: from, to: to }	
}

func parseGetDataAll(tokens []string) (bool, DataRequest) {
	requiredLength := 2
	if len(tokens) < requiredLength {
		return false, DataRequest {}
	}

	if tokens[1] == "all" || tokens[1] == "All" {
		return true, DataRequest { url: tokens[0], from: time.Unix(0, 0), to: time.Unix(2147483647, 0)}
	}

	return false, DataRequest {}
}

func parseGetData(tokens []string) (bool, DataRequest) {
	requiredLength := 1
	if len(tokens) < requiredLength {
		return false, DataRequest{}
	}

	if tokens[0] == "GetData" {
		parsed, request := parseGetDataAll(tokens[1:])
		if parsed {
			return true, request
		}

		parsed, request = parseGetDataInRange(tokens[1:])
		if parsed {
			return true, request
		}
	}

	return false, DataRequest {}
}

func readInput() {
	for {
		var input string
		_, err := fmt.Scanln(&input)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}

		if input == "exit" || input == "kill" {
			return
		}

		tokens := strings.Split(input, "")

		parsed, request := parseGetData(tokens) 
		if !parsed {
			fmt.Printf("Unrecognized command\n")
		}
	}
}
