package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

// LogMessage represents a log entry
type LogMessage struct {
	Message string
}

func main() {
	logChan := make(chan LogMessage, 100)
	logDoneChan := make(chan struct{}) 

	// Start the logging goroutine
	go asyncLogger(logChan, logDoneChan)

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

	for i := 0; i < 3; i++ {
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
	<-logDoneChan
	
}

func asyncLogger(logChan <-chan LogMessage, doneChan chan<- struct{}) {
	logFile, err := os.OpenFile("logs/thousand/echo1.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
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
                bufferedWriter.Flush() // Flush remaining logs before terminating
                doneChan <- struct{}{}     // Signal that logging is done
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
	// Create a context with a 10-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pinger, err := probing.NewPinger(url)
	if err != nil {
		return false, err
	}
	pinger.Count = 3
	var allPacketsReceived bool
	
	pinger.OnFinish = func(stats *probing.Statistics) {
	    logMsg := fmt.Sprintf("%s: %d/%d packets, %v%% loss, round-trip min/avg/max/stddev = %v/%v/%v/%v",
	        stats.Addr, 
	        stats.PacketsSent, 
	        stats.PacketsRecv, 
	        stats.PacketLoss, 
	        stats.MinRtt, 
	        stats.AvgRtt, 
	        stats.MaxRtt, 
	        stats.StdDevRtt)
	    logChan <- LogMessage{Message: logMsg}

		allPacketsReceived = stats.PacketsSent == stats.PacketsRecv
	}

	err = pinger.RunWithContext(ctx)
	if err != nil {
		return false, err
	}

	return allPacketsReceived, nil
}

func worker(urlsChan <-chan string, resultsChan chan<- bool, doneChan chan<- struct{}, workerID int, logChan chan<- LogMessage) {
    for url := range urlsChan {
        // pass the logChan to pingUrl
        success, err := pingUrl(url, logChan)

        if !success || err != nil {
            logChan <- LogMessage{Message: fmt.Sprintf("Worker %d: Error pinging %s: %v", workerID, url, err)}
            resultsChan <- false
        } else {
            resultsChan <- true
        }
    }
    doneChan <- struct{}{} // Signal that this worker is done
}
