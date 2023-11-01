package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	// Call the importUrl function to read URLs from a file
	urls, err := importUrl("urls.txt")
	if err != nil {
		log.Fatalf("Error importing URLs: %s", err)
	}

	// URLs are now stored in the 'urls' variable
	fmt.Println("Number of URLs read from the file: ", len(urls))
	
	// url channels
	urlsChan := make(chan string, len(urls))
	resultsChan := make(chan bool, len(urls))
	startTime := time.Now()

	for i := 0; i < 3; i++ {
		go worker(urlsChan, resultsChan, time.Now())
	}

	for _, url := range urls {
		urlsChan <- url
	}
	close(urlsChan)

	successCount := 0
	for range urls {
		if success := <-resultsChan; success {
			successCount++
		}
		
	}
	successPercentage := float64(successCount) / float64(len(urls)) * 100
	fmt.Printf("%.2f%% of the URLs were successfully pinged.\n", successPercentage)

	totalTime := time.Since(startTime)
	fmt.Printf("Total time taken to ping URLs: %s\n", totalTime)
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

// pingUrl tries to retrieve the URL using a HEAD request and returns the HTTP status code and error if any.
func pingUrl(url string) (int, error) {
	
	client := http.Client{
		Timeout: 10 * time.Second,
	}
	// We use a HEAD request to minimize data transfer.
	resp, err := client.Head(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}

// worker pings URLs received on the urlsChan and sends the results on the resultsChan.
func worker(urlsChan <-chan string, resultsChan chan<- bool, startTime time.Time) {
	for url := range urlsChan {
		// Ping the URL and send the result on the resultsChan.
		status, err := pingUrl(url)
		
		success := err == nil && status == http.StatusOK
		
		resultsChan <- success

		// Log the % of URLs that have been pinged.
		elapsed := time.Since(startTime)
		fmt.Printf("Pinged %s | Success: %t | Status Code: %d | Elapsed Time: %s\n", url, success, status, elapsed)
	}
}