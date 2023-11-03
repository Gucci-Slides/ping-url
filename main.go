package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// LogMessage represents a log entry
type LogMessage struct {
	Message string
}



func main() {
	logChan := make(chan LogMessage, 100) 

	// Start the logging goroutine
	go asyncLogger(logChan)

	// Call the importUrl function to read URLs from a file
	urls, err := importUrl("urls.txt")
	if err != nil {
		logChan <- LogMessage{Message: fmt.Sprintf("Error importing URLs: %s", err)}
		return
	}

	
	logChan <- LogMessage{Message: fmt.Sprintf("Number of URLs read from the file: %d", len(urls))}	

	// url channels
	urlsChan := make(chan string, len(urls))
	resultsChan := make(chan bool, len(urls))
	doneChan := make(chan struct{})
	startTime := time.Now()

	for i := 0; i < 5; i++ {
		go worker(urlsChan, resultsChan, doneChan, i, logChan)
	}

	for _, url := range urls {
		urlsChan <- url
	}
	close(urlsChan)

	for i := 0; i < 5; i++ {
		<-doneChan
	}

	successCount := 0
	for range urls {
		if success := <-resultsChan; success {
			successCount++
		}
		
	}
	successPercentage := float64(successCount) / float64(len(urls)) * 100
	logChan <- LogMessage{Message: fmt.Sprintf("%.2f%% of the URLs were successfully pinged.", successPercentage)}

	totalTime := time.Since(startTime).Seconds()
	logChan <- LogMessage{Message: fmt.Sprintf("Total time taken to ping URLs: %f", totalTime)}

	
	close(logChan)
	time.Sleep(1 * time.Second) // Give some time for logs to flush
}

func asyncLogger(logChan <-chan LogMessage) {
	logFile, err := os.OpenFile("myapp6.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err) // Replace with more sophisticated error handling
	}
	defer logFile.Close()

	bufferedWriter := bufio.NewWriter(logFile)
	defer bufferedWriter.Flush()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case logMsg, ok := <-logChan:
			if !ok {
				return // Channel closed, terminate goroutine
			}
			bufferedWriter.WriteString(logMsg.Message + "\n")
		case <-ticker.C:
			if err := bufferedWriter.Flush(); err != nil {
				log.Println("error flushing log buffer: ", err)
				// Handle error appropriately.
			}
		}
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

func pingUrl(url string, logChan chan<- LogMessage) (bool, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return false, err
    }

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/119.0")
	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")

    // Request only the first 512 bytes of the content
    req.Header.Add("Range", "bytes=0-511")

    client := &http.Client{}

    resp, err := client.Do(req)
    if err != nil {
        return false, err
    }
    defer resp.Body.Close()

    // Log detailed server response if status code is 403
    if resp.StatusCode == http.StatusForbidden {
        headersJson, err := json.Marshal(resp.Header)
        if err != nil {
            logChan <- LogMessage{Message: fmt.Sprintf("Worker: Error marshalling headers for URL %s: %v", url, err)}
        } else {
            logChan <- LogMessage{Message: fmt.Sprintf("Worker: 403 Forbidden for URL %s: Headers: %s", url, string(headersJson))}
        }
    }

    // You may also want to check for resp.StatusCode == http.StatusPartialContent if you expect partial content.
    if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
        errMsg := fmt.Sprintf("Non-200 HTTP response for %s: %d %s", url, resp.StatusCode, http.StatusText(resp.StatusCode))
        return false, fmt.Errorf(errMsg)
    }

    // Optionally read and discard the body to free network resources
    // This reads the body but ignores the returned data
    _, err = io.CopyN(io.Discard, resp.Body, 512)
    if err != nil && err != io.EOF {
        return false, err
    }

    return true, nil
}


func worker(urlsChan <-chan string, resultsChan chan<- bool, doneChan chan<- struct{}, workerID int, logChan chan<- LogMessage) {
	
	for url := range urlsChan {
		success, err := pingUrl(url, logChan)
		if err != nil {
			var message string
			if errors.Is(err, context.DeadlineExceeded) {
				message = fmt.Sprintf("Worker %d: Request to %s timed out", workerID, url)
			} else {
				message = fmt.Sprintf("Worker %d: Error pinging %s: %v", workerID, url, err)
			}
			logChan <- LogMessage{Message: message}
			success = false
		}
		resultsChan <- success
		
	}
	doneChan <- struct{}{} // Signal that this worker is done
}