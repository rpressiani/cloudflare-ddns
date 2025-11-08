package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Configuration - Set these via environment variables
type Config struct {
	APIToken   string // Cloudflare API Token
	ZoneID     string // Cloudflare Zone ID
	RecordName string // DNS record name (e.g., "home.example.com")
	RecordType string // DNS record type (usually "A" for IPv4)
}

// CloudflareRecord represents a DNS record in Cloudflare
type CloudflareRecord struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

// CloudflareResponse is the API response structure
// It handles both array results (list operations) and single results (create/update)
type CloudflareResponse struct {
	Success bool            `json:"success"`
	Errors  []string        `json:"errors"`
	Result  json.RawMessage `json:"result,omitempty"` // Can be array or object
}

func main() {
	// Load configuration from environment variables
	config := Config{
		APIToken:   os.Getenv("CF_API_TOKEN"),
		ZoneID:     os.Getenv("CF_ZONE_ID"),
		RecordName: os.Getenv("CF_RECORD_NAME"),
		RecordType: "A", // IPv4 address
	}

	// Validate configuration
	if config.APIToken == "" || config.ZoneID == "" || config.RecordName == "" {
		fmt.Println("Error: Missing required environment variables")
		fmt.Println("Required: CF_API_TOKEN, CF_ZONE_ID, CF_RECORD_NAME")
		os.Exit(1)
	}

	// Step 1: Get current public IP
	currentIP, err := getCurrentIP()
	if err != nil {
		fmt.Printf("Error getting current IP: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Current public IP: %s\n", currentIP)

	// Step 2: Check if DNS record exists
	existingRecord, err := getDNSRecord(config)
	if err != nil {
		fmt.Printf("Error checking DNS record: %v\n", err)
		os.Exit(1)
	}

	// Step 3: Update or create the record
	if existingRecord != nil {
		// Record exists - check if update is needed
		if existingRecord.Content == currentIP {
			fmt.Println("DNS record is already up to date")
			return
		}
		fmt.Printf("Updating DNS record from %s to %s\n", existingRecord.Content, currentIP)
		err = updateDNSRecord(config, existingRecord.ID, currentIP)
	} else {
		// Record doesn't exist - create it
		fmt.Println("DNS record not found, creating new record")
		err = createDNSRecord(config, currentIP)
	}

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("DNS update successful!")
}

// getCurrentIP fetches the current public IP address
func getCurrentIP() (string, error) {
	// Using a reliable IP checking service
	resp, err := http.Get("https://api.ipify.org?format=text")
	if err != nil {
		return "", fmt.Errorf("failed to get IP: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	ipBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(ipBytes), nil
}

// getDNSRecord checks if a DNS record exists and returns it
func getDNSRecord(config Config) (*CloudflareRecord, error) {
	// Build the API URL to list DNS records
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?type=%s&name=%s",
		config.ZoneID, config.RecordType, config.RecordName)

	// Create HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	req.Header.Set("Authorization", "Bearer "+config.APIToken)
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Parse the response
	var cfResp CloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !cfResp.Success {
		return nil, fmt.Errorf("cloudflare API error: %v", cfResp.Errors)
	}

	// Parse the Result field as an array of records
	var records []CloudflareRecord
	if err := json.Unmarshal(cfResp.Result, &records); err != nil {
		return nil, fmt.Errorf("failed to parse records: %w", err)
	}

	// Return the first matching record, or nil if none found
	if len(records) > 0 {
		return &records[0], nil
	}

	return nil, nil
}

// updateDNSRecord updates an existing DNS record
func updateDNSRecord(config Config, recordID, newIP string) error {
	// Build the API URL for updating
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s",
		config.ZoneID, recordID)

	// Prepare the update data
	record := CloudflareRecord{
		Type:    config.RecordType,
		Name:    config.RecordName,
		Content: newIP,
		TTL:     1,     // 1 = automatic
		Proxied: false, // Set to true if you want Cloudflare proxy
	}

	// Convert to JSON
	jsonData, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+config.APIToken)
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Parse the response
	var cfResp CloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !cfResp.Success {
		return fmt.Errorf("cloudflare API error: %v", cfResp.Errors)
	}

	return nil
}

// createDNSRecord creates a new DNS record
func createDNSRecord(config Config, ip string) error {
	// Build the API URL for creating
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records",
		config.ZoneID)

	// Prepare the record data
	record := CloudflareRecord{
		Type:    config.RecordType,
		Name:    config.RecordName,
		Content: ip,
		TTL:     1,     // 1 = automatic
		Proxied: false, // Set to true if you want Cloudflare proxy
	}

	// Convert to JSON
	jsonData, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+config.APIToken)
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Parse the response
	var cfResp CloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !cfResp.Success {
		return fmt.Errorf("cloudflare API error: %v", cfResp.Errors)
	}

	return nil
}
