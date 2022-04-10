package main

import (
	"fmt"
	"os"
	"time"
	"strings"
)

type DataRequest struct {
	Url  string
	From time.Time
	To   time.Time
}

// Get data in range: GetData <url> <from> <to>
// Get data all: GetData <url> <all>

func parseInput(tokens []string) (bool, DataRequest) {
	minLen := 3
	if len(tokens) < minLen {
		fmt.Printf("Get data in range: GetData <url> <from> <to>\nGet data all: GetData <url> <all>")
		return false, DataRequest {}
	}

	if tokens[0] != "GetData" {
		fmt.Printf("Unrecognized command: %v", tokens[0])
		return false, DataRequest {}
	} 

	var result DataRequest
	result.Url = tokens[1]

	if tokens[2] == "all" {
		from := time.Unix(0, 0)
		result.From = from

		to := time.Unix(2147483647, 0)
		result.To = to
	} else {
		var failed bool
		minLen = 4
		if len(tokens) < minLen {
			fmt.Printf("Specify the \"to\" part of the date range. Or use \"all\" to get all pages")
			return false, DataRequest {}
		}
		from, err := time.Parse(time.ANSIC, tokens[2])
		if err != nil {
			fmt.Printf("Failed to parse \"from\" date")
			failed = true
		}
		result.From = from

		to, err := time.Parse(time.ANSIC, tokens[3])
		if err != nil {
			fmt.Printf("Failed to parse \"to\" date")
			failed = true
		}
		result.To = to

		if failed {
			return false, DataRequest {}
		}


	}
	return true, result
}

func readInput(dbContext *DbContext) {
	for {
		var input string
		_, err := fmt.Scanln(&input)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}

		if input == "exit" || input == "kill" {
			os.Exit(0)
		}

		tokens := strings.Split(input, "")

		parsed, request := parseInput(tokens) 

		if parsed {
			fmt.Printf("parsed")
			pages, err := queryUrlDataInRange(dbContext, request.Url, request.From, request.To)
			if err != nil{
				fmt.Printf("Failed to query pages: %v")
				continue
			}

			zip, err := zipifyPages(pages)
			if err != nil {
				fmt.Printf("Page zipping failed: %v")
				continue	
			}

			f, err := os.OpenFile("output.zip", os.O_RDWR | os.O_CREATE | os.O_TRUNC, 0755)
			if err != nil {
				fmt.Printf("Could not create file")
				continue
			}

			_, err = f.Write(zip)
			if err != nil {
				fmt.Printf("Failed to write to the file")
				continue
			}

			f.Close()
			fmt.Printf("Successfully written %v bytes", len(zip))
		}
	}
}
