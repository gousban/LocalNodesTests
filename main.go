package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"LocalNodesTests/types"
)

type crlfWriter struct {
	Writer io.Writer
}

func (w *crlfWriter) Write(p []byte) (n int, err error) {
	p = []byte(strings.ReplaceAll(string(p), "\n", "\r\n"))
	return w.Writer.Write(p)
}

var xrayCmd *exec.Cmd

func waitForXray(timeout time.Duration, apiAddr string) error {
	start := time.Now()
	for {
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for xray to start")
		}
		conn, err := net.Dial("tcp", apiAddr)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func startXray(config types.Config) error {
	// Get absolute path to xray directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %v", err)
	}
	xrayPath := filepath.Join(wd, "xray", "xray.exe")

	xrayCmd = exec.Command(xrayPath, "-config", "config.json")
	xrayCmd.Dir = filepath.Join(wd, "xray") // Set working directory to xray folder
	xrayCmd.Stdout = os.Stdout
	xrayCmd.Stderr = os.Stderr
	if err := xrayCmd.Start(); err != nil {
		return fmt.Errorf("failed to start xray: %v", err)
	}

	// Wait for xray to be ready
	if err := waitForXray(5*time.Second, config.ApiAddr); err != nil {
		stopXray()
		return fmt.Errorf("xray failed to start: %v", err)
	}
	return nil
}

func stopXray() {
	if xrayCmd != nil && xrayCmd.Process != nil {
		xrayCmd.Process.Kill()
	}
}

func main() {

	// Start xray
	var config types.Config
	// Parse command-line flags
	configFile := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	if _, err := os.Stat(*configFile); err == nil {
		configData, err := os.ReadFile(*configFile)
		if err != nil {
			log.Fatalf("Failed to read config file %s: %v", *configFile, err)
		}
		if err := yaml.Unmarshal(configData, &config); err != nil {
			log.Fatalf("Config parse failed: %v", err)
		}
	} else {
		log.Printf("Config file %s not found, using default config", *configFile)
		config = types.Config{
			SpeedTestURL:    "https://speed.cloudflare.com/__down?bytes=10485760",
			Concurrent:      5,
			Timeout:         15000,
			MinSpeed:        256,
			SaveMethod:      "local",
			GistToken:       os.Getenv("GIST_TOKEN"),
			GistID:          os.Getenv("GIST_ID"),
			SubURLs:         []string{},
			ProxyAddr:       "127.0.0.1:10808",
			ApiAddr:         "127.0.0.1:10085", // Default API address for Xray
			AllOutputFile:   "all.yaml",
			UniqueNodesFile: "uniqueNodes.txt",
			TCPTestURL:      "https://www.apple.com/library/test/success.html",
			TCPTestMaxSpeed: 3000,
		}
	}

	// Override config with environment variables if provided
	if token := os.Getenv("GIST_TOKEN"); token != "" {
		config.GistToken = token
	}
	if gistID := os.Getenv("GIST_ID"); gistID != "" {
		config.GistID = gistID
	}

	if err := startXray(config); err != nil {
		log.Fatalf("Failed to start xray: %v", err)
	}
	defer stopXray()

	// Set up logging to runlog.txt
	logFile, err := os.Create("runlog.txt")
	if err != nil {
		fmt.Printf("Failed to create runlog.txt: %v\n", err)
		return
	}
	defer logFile.Close()
	log.SetOutput(&crlfWriter{Writer: logFile})

	// Set up simple logger to parsingLog.txt
	simpleLogFile, err := os.Create("parsingLog.txt")
	if err != nil {
		log.Printf("Failed to create parsingLog.txt: %v", err)
		return
	}
	defer simpleLogFile.Close()
	simpleLogger := log.New(&crlfWriter{Writer: simpleLogFile}, "", 0)

	// Progress update function
	updateProgress := func(stageName string) {
		fmt.Printf("\033[2K\rStage: %s\n", stageName)
	}

	// Display stages
	fmt.Println("There are 4 stages: Fetching, Parsing, Testing (optional), Saving")

	// Prompt user for test selection with 5-second timeout
	fmt.Print("Select test: (0) No test, (1) TCP test, (2) Download speed test, (3) Both [default 0 in 10s]: ")
	choiceChan := make(chan string, 1)
	go func() {
		var choice string
		if _, err := fmt.Scanln(&choice); err != nil {
			choice = "0" // Default to 0 on error or no input
		}
		choiceChan <- choice
	}()

	var testChoice string
	select {
	case choice := <-choiceChan:
		testChoice = choice
	case <-time.After(10 * time.Second):
		testChoice = "0"
		fmt.Println("\nDefaulting to (0) No test")
	}

	// Stage 1: Fetch content
	updateProgress("Fetching")
	content, err := fetchContent(config, updateProgress)
	if err != nil {
		return // Error already logged in fetchContent
	}

	// Stage 2: Parse nodes
	updateProgress("Parsing")
	stats := types.ProxyStats{}
	nodes := fetchNodes(content, simpleLogger, &stats)

	// Log parsing statistics
	simpleLogger.Printf("Total Success: %d", stats.TotalSuccess)
	simpleLogger.Printf("Total Fail: %d", stats.TotalFail)
	simpleLogger.Printf("SS Success: %d, SS Fail: %d", stats.SSSuccess, stats.SSFail)
	simpleLogger.Printf("VMess Success: %d, VMess Fail: %d", stats.VMessSuccess, stats.VMessFail)
	simpleLogger.Printf("Trojan Success: %d, Trojan Fail: %d", stats.TrojanSuccess, stats.TrojanFail)
	simpleLogger.Printf("Hysteria2 Success: %d, Hysteria2 Fail: %d", stats.Hysteria2Success, stats.Hysteria2Fail)
	simpleLogger.Printf("VLess Success: %d, VLess Fail: %d", stats.VLessSuccess, stats.VLessFail)
	simpleLogger.Println("--- Parsing Results ---")

	// Stage 3: Test nodes (if selected)
	var tested []types.Proxy
	updateProgress("Testing")
	tested = testNodes(config, nodes, testChoice)

	// Stage 4: Save results
	updateProgress("Saving")
	saveResults(config, tested, testChoice)

	// Completion
	updateProgress("Completed")
	fmt.Println()
	fmt.Println("Build and run completed!")
}
