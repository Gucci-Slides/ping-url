package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
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
	logFile, err := os.OpenFile("myapp8.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
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

func pingUrl(url string) (bool, int, time.Duration, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Custom CheckRedirect function to handle redirects
    client := &http.Client{
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            if len(via) >= 10 {
                return http.ErrUseLastResponse
            }
            return nil
        },
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
    if err != nil {
        return false, 0, 0, err
    }

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36")
	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Add("Accept-Language", "en-US,en;q=0.5")
	req.Header.Add("Accept-Encoding", "gzip, deflate, br")
	req.Header.Add("Referer", "http://www.google.com/")

	start := time.Now()
    resp, err := client.Do(req)
	duration := time.Since(start)
    if err != nil {
        return false, 0, duration, err
    }
    defer resp.Body.Close()


    statusCode := resp.StatusCode
    // Consider the request a success if status code is within 200-299
    success := statusCode >= 200 && statusCode < 300

    return success, statusCode, duration, nil
}


func worker(urlsChan <-chan string, resultsChan chan<- bool, doneChan chan<- struct{}, workerID int, logChan chan<- LogMessage) {
    for url := range urlsChan {
        success, statusCode, duration, err := pingUrl(url)

        if err != nil {
            if errors.Is(err, context.DeadlineExceeded) {
                logChan <- LogMessage{Message: fmt.Sprintf("Worker %d: Request to %s timed out after %v", workerID, url, duration)}
            } else {
                logChan <- LogMessage{Message: fmt.Sprintf("Worker %d: Error pinging %s after %v: %v", workerID, url, duration, err)}
            }
        } else {
            logChan <- LogMessage{Message: fmt.Sprintf("Worker %d: Received HTTP status %d for URL %s in %v", workerID, statusCode, url, duration)}
        }

        resultsChan <- success
    }
    doneChan <- struct{}{} // Signal that this worker is done
}