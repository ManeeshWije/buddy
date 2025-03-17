package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LlamaRequest struct {
	Prompt      string  `json:"prompt"`
	Temperature float64 `json:"temperature,omitempty"`
}

type LlamaResponse struct {
	Generation string `json:"generation"`
}

type Message struct {
	ConversationID string    `json:"conversationId" dynamodbav:"conversationId"`
	MessageID      string    `json:"messageId" dynamodbav:"messageId"`
	UserID         string    `json:"userId" dynamodbav:"userId"`
	Role           string    `json:"role" dynamodbav:"role"`
	Content        string    `json:"content" dynamodbav:"content"`
	Timestamp      time.Time `json:"timestamp" dynamodbav:"timestamp"`
}

// Define regex patterns for cleaning responses
var (
	systemTagPattern    = regexp.MustCompile(`(?i)<\|system\|>[\s\S]*?(?:<\|user\|>|<\|assistant\|>|$)`)
	userTagPattern      = regexp.MustCompile(`(?i)<\|user\|>[\s\S]*?(?:<\|system\|>|<\|assistant\|>|$)`)
	assistantTagPattern = regexp.MustCompile(`(?i)<\|assistant\|>`)
	allTagsPattern      = regexp.MustCompile(`(?i)<\|(?:system|user|assistant)\|>`)
)

func cleanResponse(response string) string {
	// Remove any system message blocks completely
	response = systemTagPattern.ReplaceAllString(response, "")

	// Remove any user message blocks completely
	response = userTagPattern.ReplaceAllString(response, "")

	// Remove just the <|assistant|> tag
	response = assistantTagPattern.ReplaceAllString(response, "")

	// Remove any remaining tags that might have been missed
	response = allTagsPattern.ReplaceAllString(response, "")

	// Clean up extra whitespace
	response = strings.TrimSpace(response)

	return response
}

func Handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Parse the request
	var req Request
	err := json.Unmarshal([]byte(request.Body), &req)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       fmt.Sprintf("Invalid request: %v", err),
		}, nil
	}

	// Create a new AWS config
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(os.Getenv("AWS_REGION")))
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to load AWS config: %v", err),
		}, nil
	}

	// Create DynamoDB client
	dynamoClient := dynamodb.NewFromConfig(cfg)

	// Create Bedrock client
	bedrockClient := bedrockruntime.NewFromConfig(cfg)

	// Get conversation history or create a new conversation
	conversationID := req.ConversationID
	var messages []ChatMessage

	// Define system message
	systemMessage := ChatMessage{
		Role:    "system",
		Content: "You are a helpful CLI assistant specialized in Linux and terminal commands. Provide concise, accurate information and examples when asked. Be concise and never output a large wall of text that is hard to parse through for the user. Only respond to what the user is specifically asking about. Never assume or make up information about the user's environment or previous conversations. Only use information that the user has explicitly provided. DO NOT include any <|system|>, <|user|>, or <|assistant|> tags in your responses.",
	}

	// Start with system message
	messages = []ChatMessage{systemMessage}

	if conversationID != "" {
		// Get conversation history
		history, err := getConversationHistory(ctx, dynamoClient, req.UserID, conversationID)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 500,
				Body:       fmt.Sprintf("Failed to get conversation history: %v", err),
			}, nil
		}

		// Add conversation history
		messages = append(messages, history...)
	} else {
		// Create a new conversation ID
		conversationID = fmt.Sprintf("conv-%d", time.Now().UnixNano())
	}

	// Add the user's current query
	messages = append(messages, ChatMessage{
		Role:    "user",
		Content: req.Query,
	})

	// Store the user's message in DynamoDB
	err = storeMessage(ctx, dynamoClient, req.UserID, conversationID, "user", req.Query)
	if err != nil {
		log.Printf("Failed to store user message: %v", err)
		// Continue processing even if storage fails
	}

	// Create the prompt for Llama
	var fullPrompt strings.Builder

	// Start with system message
	fullPrompt.WriteString("<|system|>\n")
	fullPrompt.WriteString(systemMessage.Content)
	fullPrompt.WriteString("\n")

	// Only add user and assistant messages from the history
	for _, msg := range messages[1:] { // Skip system message since we added it already
		if msg.Role == "user" {
			fullPrompt.WriteString("<|user|>\n")
			fullPrompt.WriteString(msg.Content)
			fullPrompt.WriteString("\n")
		} else if msg.Role == "assistant" {
			fullPrompt.WriteString("<|assistant|>\n")
			fullPrompt.WriteString(msg.Content)
			fullPrompt.WriteString("\n")
		}
	}

	// Add final assistant turn marker
	fullPrompt.WriteString("<|assistant|>\n")

	// Prepare the Llama 3 request
	llamaRequest := LlamaRequest{
		Prompt:      fullPrompt.String(),
		Temperature: 0.5, // Lower temperature for more deterministic responses
	}

	// Convert the request to JSON
	reqJSON, err := json.Marshal(llamaRequest)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to marshal Llama request: %v", err),
		}, nil
	}

	// Get the correct model ID from environment variable
	modelID := os.Getenv("BEDROCK_MODEL_ID")
	if modelID == "" {
		modelID = "us.meta.llama3-3-70b-instruct-v1:0" // default
	}

	// Create the InvokeModel request
	invokeInput := &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(modelID),
		Body:        reqJSON,
		ContentType: aws.String("application/json"),
	}

	// Invoke the model
	invokeOutput, err := bedrockClient.InvokeModel(ctx, invokeInput)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to invoke Bedrock model: %v", err),
		}, nil
	}

	// Parse the response
	var llamaResponse LlamaResponse
	err = json.Unmarshal(invokeOutput.Body, &llamaResponse)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to unmarshal Bedrock response: %v", err),
		}, nil
	}

	// Clean the response before storing and returning
	cleanedResponse := cleanResponse(llamaResponse.Generation)

	// Store the cleaned assistant's response in DynamoDB
	err = storeMessage(ctx, dynamoClient, req.UserID, conversationID, "assistant", cleanedResponse)
	if err != nil {
		log.Printf("Failed to store assistant message: %v", err)
		// Continue processing even if storage fails
	}

	// Prepare the response
	response := Response{
		ConversationID: conversationID,
		Response:       cleanedResponse,
	}

	// Convert to JSON
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to marshal response: %v", err),
		}, nil
	}

	// Return the response
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type":                "application/json",
			"Access-Control-Allow-Origin": "*", // For CORS support
		},
		Body: string(responseJSON),
	}, nil
}

func getConversationHistory(ctx context.Context, client *dynamodb.Client, userID, conversationID string) ([]ChatMessage, error) {
	// Query DynamoDB for messages in this conversation
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(os.Getenv("MESSAGES_TABLE")),
		KeyConditionExpression: aws.String("conversationId = :conversationId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":conversationId": &types.AttributeValueMemberS{Value: conversationID},
			":userId":         &types.AttributeValueMemberS{Value: userID},
		},
		FilterExpression: aws.String("userId = :userId"),
		ScanIndexForward: aws.Bool(true), // Sort by timestamp (oldest first)
	}

	result, err := client.Query(ctx, queryInput)
	if err != nil {
		return nil, err
	}

	var messages []Message
	err = attributevalue.UnmarshalListOfMaps(result.Items, &messages)
	if err != nil {
		return nil, err
	}

	// Convert to ChatMessage format
	var chatMessages []ChatMessage
	for _, msg := range messages {
		if msg.Role == "user" || msg.Role == "assistant" { // Only include user and assistant messages
			chatMessages = append(chatMessages, ChatMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	return chatMessages, nil
}

func storeMessage(ctx context.Context, client *dynamodb.Client, userID, conversationID, role, content string) error {
	messageID := fmt.Sprintf("msg-%d", time.Now().UnixNano())
	timestamp := time.Now()

	message := Message{
		ConversationID: conversationID, // Primary partition key first
		MessageID:      messageID,      // Primary sort key second
		UserID:         userID,
		Role:           role,
		Content:        content,
		Timestamp:      timestamp,
	}

	item, err := attributevalue.MarshalMap(message)
	if err != nil {
		return err
	}

	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(os.Getenv("MESSAGES_TABLE")),
		Item:      item,
	})

	return err
}

func main() {
	lambda.Start(Handler)
}
