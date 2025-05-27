// 316cde3a-4a88-460e-a78b-af5fb33ad88c

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// OpenAI-compatible API structures
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

type Choice struct {
	Index   int     `json:"index"`
	Message Message `json:"message"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type CompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type ExperimentResult struct {
	Timestamp time.Time          `json:"timestamp"`
	Model     string             `json:"model"`
	Request   CompletionRequest  `json:"request"`
	Response  CompletionResponse `json:"response"`
	QueryHash string             `json:"query_hash"`
	ErrorMsg  string             `json:"error,omitempty"`
}

type Config struct {
	APIEndpoint  string
	APIKey       string
	Models       []string
	SystemPrompt string
	UserPrompt   string
	OutputDir    string
	MaxTokens    int
	Temperature  float64
	TopP         float64
	SystemFile   string
	UserFile     string
	SaveMarkdown bool
}

func main() {
	config := parseFlags()

	if err := validateConfig(config); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Load prompts from files if specified
	if config.SystemFile != "" {
		content, err := os.ReadFile(config.SystemFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading system prompt file: %v\n", err)
			os.Exit(1)
		}
		config.SystemPrompt = string(content)
	}

	if config.UserFile != "" {
		content, err := os.ReadFile(config.UserFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading user prompt file: %v\n", err)
			os.Exit(1)
		}
		config.UserPrompt = string(content)
	}

	fmt.Printf("Running experiments with %d model(s)...\n", len(config.Models))

	for _, model := range config.Models {
		fmt.Printf("Testing model: %s\n", model)
		if err := runExperiment(config, model); err != nil {
			fmt.Fprintf(os.Stderr, "Error with model %s: %v\n", model, err)
		}
	}
}

func parseFlags() *Config {
	config := &Config{}

	var modelsFlag string

	flag.StringVar(&config.APIEndpoint, "endpoint", "https://chat-ai.example.com/v1/chat/completions", "API endpoint URL")
	flag.StringVar(&config.APIKey, "api-key", "", "API key for authentication")
	flag.StringVar(&modelsFlag, "models", "meta-llama-3.1-8b-instruct", "Comma-separated list of models to test")
	flag.StringVar(&config.SystemPrompt, "system", "You are an assistant.", "System prompt")
	flag.StringVar(&config.UserPrompt, "user", "", "User prompt (required)")
	flag.StringVar(&config.OutputDir, "output", "experiments", "Output directory for results")
	flag.IntVar(&config.MaxTokens, "max-tokens", 1000, "Maximum tokens to generate")
	flag.Float64Var(&config.Temperature, "temperature", 0.7, "Temperature for generation")
	flag.Float64Var(&config.TopP, "top-p", 0.9, "Top-p for generation")
	flag.StringVar(&config.SystemFile, "system-file", "", "File containing system prompt")
	flag.StringVar(&config.UserFile, "user-file", "", "File containing user prompt")
	flag.BoolVar(&config.SaveMarkdown, "save-md", false, "Save response content to separate .md file")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "LLM Experiment CLI Tool\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -api-key YOUR_KEY -user \"What is the weather today?\"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -api-key YOUR_KEY -models \"model1,model2\" -user-file prompt.txt\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -api-key YOUR_KEY -system-file sys.txt -user-file user.txt -output results/\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -api-key YOUR_KEY -user \"Code a palm tree\" -save-md\n", os.Args[0])
	}

	flag.Parse()

	// Parse models
	config.Models = strings.Split(modelsFlag, ",")
	for i, model := range config.Models {
		config.Models[i] = strings.TrimSpace(model)
	}

	return config
}

func validateConfig(config *Config) error {
	if config.APIKey == "" {
		return fmt.Errorf("API key is required")
	}

	if config.UserPrompt == "" && config.UserFile == "" {
		return fmt.Errorf("user prompt is required (use -user or -user-file)")
	}

	if len(config.Models) == 0 {
		return fmt.Errorf("at least one model must be specified")
	}

	return nil
}

func runExperiment(config *Config, model string) error {
	// Prepare request
	messages := []Message{}

	if config.SystemPrompt != "" {
		messages = append(messages, Message{
			Role:    "system",
			Content: config.SystemPrompt,
		})
	}

	messages = append(messages, Message{
		Role:    "user",
		Content: config.UserPrompt,
	})

	req := CompletionRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   config.MaxTokens,
		Temperature: config.Temperature,
		TopP:        config.TopP,
	}

	// Make API call
	response, err := makeAPICall(config.APIEndpoint, config.APIKey, req)

	// Create experiment result
	result := ExperimentResult{
		Timestamp: time.Now(),
		Model:     model,
		Request:   req,
		QueryHash: generateQueryHash(config.UserPrompt),
	}

	// Save result
	filename := generateFilename(model, config.UserPrompt, result.Timestamp)
	path := filepath.Join(config.OutputDir, filename)

	if err != nil {
		result.ErrorMsg = err.Error()
		fmt.Printf("  ❌ Error: %v\n", err)
	} else {
		result.Response = *response
		// Extract the actual response content for display
		var responseContent string
		if len(response.Choices) > 0 {
			responseContent = response.Choices[0].Message.Content
		}

		fmt.Printf("  ✅ Success: %d tokens used\n", response.Usage.TotalTokens)
		if responseContent != "" {
			// Show first 100 chars of response
			preview := responseContent
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			fmt.Printf("  📝 Response preview: %s\n", preview)

			// Save markdown file if requested and we have content
			if config.SaveMarkdown && responseContent != "" {
				mdFilename := strings.TrimSuffix(filename, ".json") + ".md"
				mdFilepath := filepath.Join(config.OutputDir, mdFilename)
				if err := saveMarkdown(mdFilepath, responseContent); err != nil {
					fmt.Printf("  ⚠️  Warning: Failed to save markdown file: %v\n", err)
				} else {
					fmt.Printf("  📄 Markdown saved to: %s\n", mdFilepath)
				}
			}

		} else {
			fmt.Printf("  ⚠️  Warning: Response content appears to be empty\n")
		}
	}

	if err := saveResult(path, result); err != nil {
		return fmt.Errorf("failed to save result: %v", err)
	}

	fmt.Printf("  📁 Saved to: %s\n", path)
	return nil
}

func makeAPICall(endpoint, apiKey string, req CompletionRequest) (*CompletionResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 210 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var response CompletionResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return &response, nil
}

func generateQueryHash(query string) string {
	words := strings.Fields(query)
	if len(words) == 0 {
		return "empty"
	}

	var hashWords []string
	for i, word := range words {
		if i >= 10 { // Use first 3 words
			break
		}
		hashWords = append(hashWords, word)
	}

	return sanitizeForFilename(strings.Join(hashWords, "_"))
}

func generateFilename(model, query string, timestamp time.Time) string {
	dateStr := timestamp.Format("20060102150405")
	modelStr := sanitizeForFilename(model)
	queryStr := generateQueryHash(query)

	return fmt.Sprintf("%s_%s_%s.json", dateStr, modelStr, queryStr)
}

func sanitizeForFilename(s string) string {
	// Replace invalid filename characters
	reg := regexp.MustCompile(`[<>:"/\\|?*\s]+`)
	sanitized := reg.ReplaceAllString(s, "_")

	// Remove leading/trailing underscores and limit length
	sanitized = strings.Trim(sanitized, "_")
	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}

	return sanitized
}

func saveResult(filepath string, result ExperimentResult) error {
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath, jsonData, 0644)
}

func saveMarkdown(filepath, content string) error {
	return os.WriteFile(filepath, []byte(content), 0644)
}
