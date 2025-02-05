package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// JSON-RPC request structure
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// Configuration structure for whitelist and blacklist
type Config struct {
	WhitelistedMethods []string `json:"whitelisted_methods"`
	BlacklistedMethods []string `json:"blacklisted_methods"`
}

// forwardToSnapshotProxy forwards the request to the snapshot proxy
func forwardToSnapshotProxy(r *http.Request, body []byte) (*http.Response, error) {
	// Proxy address set to localhost:8899
	snapshotProxyURL := "http://localhost:8899" // Updated to the correct URL of anza-snapshot-proxy

	// Forward the request differently based on GET or POST
	var req *http.Request
	var err error
	if r.Method == http.MethodPost {
		req, err = http.NewRequest(r.Method, snapshotProxyURL, bytes.NewReader(body))
	} else if r.Method == http.MethodGet {
		req, err = http.NewRequest(r.Method, snapshotProxyURL+"?"+r.URL.RawQuery, nil)
	} else {
		return nil, fmt.Errorf("unsupported HTTP method: %s", r.Method)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create forward request: %v", err)
	}

	// Copy headers from the incoming request to the new request
	req.Header = r.Header

	client := &http.Client{}
	return client.Do(req)
}

// isMethodAllowed checks if the method is allowed based on whitelist and blacklist
func isMethodAllowed(method string, config *Config) bool {
	// Check if method is blacklisted
	for _, blocked := range config.BlacklistedMethods {
		if method == blocked {
			log.Printf("Blocked method: %s", method)
			return false
		}
	}

	// Check if method is whitelisted
	for _, allowed := range config.WhitelistedMethods {
		if method == allowed {
			return true
		}
	}

	// Default to rejecting if the method is not whitelisted and not explicitly blocked
	log.Printf("Method not whitelisted or blacklisted: %s", method)
	return false
}

// logRequestAndForward logs the incoming request, the raw request body, and forwards it to the snapshot proxy
func logRequestAndForward(w http.ResponseWriter, r *http.Request, config *Config) {
	// Check for file download requests first (e.g., /genesis.tar.bz2)
	if r.URL.Path == "/genesis.tar.bz2" || strings.HasPrefix(r.URL.Path, "/snapshot-") || strings.HasPrefix(r.URL.Path, "/incremental-snapshot") {
		log.Printf("Received file download request: %s", r.URL.Path)
		handleFileDownload(w, r)
		return
	}

	// Otherwise, process it as a JSON-RPC request or other request type
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Unable to read request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	// Log the raw request data
	log.Printf("Received raw request body: %s", string(body))

	// Check if the request is JSON-RPC by attempting to unmarshal it
	var rpcRequest JSONRPCRequest
	if err := json.Unmarshal(body, &rpcRequest); err == nil {
		// Log the requester's IP address
		clientIP := r.RemoteAddr
		log.Printf("Requester IP: %s", clientIP)

		// Check if the method is allowed
		if !isMethodAllowed(rpcRequest.Method, config) {
			http.Error(w, "Method not allowed", http.StatusForbidden)
			return
		}
	}

	// Forward the request to the snapshot proxy for other types of requests (e.g., JSON-RPC)
	resp, err := forwardToSnapshotProxy(r, body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to forward request: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Read the response from the snapshot proxy
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read proxy response", http.StatusInternalServerError)
		return
	}

	// Log the raw response data
	log.Printf("Received raw response body: %s", string(respBody))

	// Respond with the JSON-RPC response from the snapshot proxy
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

// handleFileDownload handles file download requests and proxies them
func handleFileDownload(w http.ResponseWriter, r *http.Request) {
	snapshotProxyURL := "http://localhost:8899"
	var req *http.Request
	var err error

	// Determine method based on request type
	if strings.HasPrefix(r.URL.Path, "/snapshot-") || strings.HasPrefix(r.URL.Path, "/incremental-snapshot") {
		// Snapshots require POST with empty body
		req, err = http.NewRequest(http.MethodPost, snapshotProxyURL+r.URL.Path, nil)
		log.Printf("Forwarding snapshot download as POST: %s", r.URL.Path)
	} else {
		// Genesis uses GET
		req, err = http.NewRequest(http.MethodGet, snapshotProxyURL+r.URL.Path, nil)
	}

	if err != nil {
		http.Error(w, "Failed to create file download request", http.StatusInternalServerError)
		return
	}

	// Copy headers and add required headers for POST requests
	req.Header = r.Header.Clone()
	if strings.HasPrefix(r.URL.Path, "/snapshot-") || strings.HasPrefix(r.URL.Path, "/incremental-snapshot") {
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("Content-Length", "0")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to download file: %v", err)
		http.Error(w, "Failed to download file", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Handle non-success status codes
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("Proxy responded with error: %d - %s", resp.StatusCode, string(body))
		http.Error(w, fmt.Sprintf("Proxy error: %s", string(body)), resp.StatusCode)
		return
	}

	// Set headers for client
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", r.URL.Path))
	w.Header().Set("Content-Length", resp.Header.Get("Content-Length"))
	w.WriteHeader(resp.StatusCode)

	// Stream the response
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Error streaming response: %v", err)
	}
}

// setupServer sets up the HTTP server and starts listening on the given port
func setupServer(config *Config) *http.Server {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logRequestAndForward(w, r, config)
	})

	port := "14705"
	server := &http.Server{
		Addr:    ":" + port,
		Handler: nil,
	}

	log.Printf("Listening on port %s...", port)
	return server
}

// handleShutdown handles graceful shutdown of the server
func handleShutdown(server *http.Server) {
	// Channel to capture termination signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for a termination signal
	<-sigChan

	// Initiate graceful shutdown
	log.Println("Shutting down server...")
	if err := server.Close(); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}
	log.Println("Server shut down gracefully")
}

// loadConfig loads the configuration file to get the whitelist and blacklist methods
func loadConfig() (*Config, error) {
	// Read the configuration file
	configFile := "config.json"
	file, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	// Unmarshal the configuration into the Config struct
	var config Config
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	return &config, nil
}

func main() {
	// Load the configuration
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Setup server
	server := setupServer(config)

	// Start the server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Handle graceful shutdown
	handleShutdown(server)
}
