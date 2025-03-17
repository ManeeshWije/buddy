package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type Request struct {
	UserID         string `json:"userId"`
	Query          string `json:"query"`
	ConversationID string `json:"conversationId,omitempty"`
}

type Response struct {
	ConversationID string `json:"conversationId"`
	Response       string `json:"response"`
}

func main() {
	// Define command line flags
	apiURL := flag.String("api", "", "API Gateway URL (required)")
	apiKey := flag.String("key", "", "API Key for authentication (required)")
	userID := flag.String("user", "default-user", "User ID")
	convID := flag.String("conv", "", "Conversation ID (optional)")
	flag.Parse()

	// Check if API URL and API Key are provided
	if *apiURL == "" || *apiKey == "" {
		fmt.Println("Error: API URL and API Key are required")
		fmt.Println("Usage: go run cmd/cli/cli.go -api <API_URL> -key <API_KEY> [-user <USER_ID>] [-conv <CONVERSATION_ID>]")
		os.Exit(1)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 60 * time.Second, // Increased timeout for longer responses
	}

	// Start conversation loop
	currentConvID := *convID
	fmt.Println("CLI Assistant is ready. Type your queries (type 'exit' to quit):")
	fmt.Println("------------------------------------------------------------")
	for {
		// Get user input
		fmt.Print("> ")
		reader := bufio.NewReader(os.Stdin)
		query, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		// Remove trailing newline
		query = query[:len(query)-1]

		// Check if user wants to exit
		if query == "exit" {
			fmt.Println("Goodbye!")
			break
		}

		// Prepare request
		req := Request{
			UserID:         *userID,
			Query:          query,
			ConversationID: currentConvID,
		}

		// Convert request to JSON
		reqJSON, err := json.Marshal(req)
		if err != nil {
			fmt.Printf("Error creating request: %v\n", err)
			continue
		}

		httpReq, err := http.NewRequest("POST", *apiURL, bytes.NewBuffer(reqJSON))
		if err != nil {
			fmt.Printf("Error creating HTTP request: %v\n", err)
			continue
		}

		// Set required headers including the API key
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("x-api-key", *apiKey) // Add the API key header

		// Show "thinking" animation while waiting for response
		doneCh := make(chan struct{})
		go showThinkingAnimation(doneCh)

		// Get response
		resp, err := client.Do(httpReq)

		// Stop the thinking animation
		close(doneCh)
		fmt.Print("\r                    \r") // Clear the animation line

		if err != nil {
			fmt.Printf("Error calling API: %v\n", err)
			continue
		}
		defer resp.Body.Close()

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("Error reading response: %v\n", err)
			continue
		}

		// Check status code
		if resp.StatusCode != http.StatusOK {
			fmt.Printf("API returned error (status %d): %s\n", resp.StatusCode, body)
			continue
		}

		// Parse response
		var apiResp Response
		if err := json.Unmarshal(body, &apiResp); err != nil {
			fmt.Printf("Error parsing response: %v\n", err)
			continue
		}

		// Update conversation ID
		currentConvID = apiResp.ConversationID

		// Print response
		fmt.Println("------------------------------------------------------------")
		fmt.Println(apiResp.Response)
		fmt.Println("------------------------------------------------------------")
	}
}

// showThinkingAnimation displays a simple animation while waiting for a response
func showThinkingAnimation(done chan struct{}) {
	spinners := []string{"-", "\\", "|", "/"}
	i := 0
	for {
		select {
		case <-done:
			return
		default:
			fmt.Printf("\rThinking %s", spinners[i])
			i = (i + 1) % len(spinners)
			time.Sleep(100 * time.Millisecond)
		}
	}
}
