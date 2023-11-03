package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	// Call the importUrl function to read URLs from a file
	urls, err := importUrl("urls.txt")
	if err != nil {
		log.Fatalf("Error importing URLs: %s", err)
	}

	// URLs are now stored in the 'urls' variable
		fmt.Println("Number of URLs read from the file: ", len(urls))

		for _, url := range urls {
			resp, err := http.Get(url)
	        if err != nil {
	            log.Fatalf("Error fetching URL %q: %v\n", url, err)
	        }
			defer resp.Body.Close()

	        statusCode := resp.StatusCode
			fmt.Println(url, " - ", statusCode)
		}
}



func importUrl(filename string) ([]string, error) {
	// Open the file containing the URLs
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)

	// Create a slice to hold the URLs
	var urls []string

	// Read each line from the file and append to the slice
	for scanner.Scan() {
		url := scanner.Text()
		urls = append(urls, url)
	}

	// Check for errors during reading
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error while reading file: %w", err)
	}

	return urls, nil
}

