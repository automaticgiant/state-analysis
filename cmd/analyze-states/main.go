package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type StateFile struct {
	Version          int    `json:"version"`
	TerraformVersion string `json:"terraform_version"`
	Serial           int    `json:"serial"`
	Lineage          string `json:"lineage"`
	Resources        []struct {
		Mode     string `json:"mode"`
		Type     string `json:"type"`
		Name     string `json:"name"`
		Provider string `json:"provider"`
	} `json:"resources"`
	FileInfo os.FileInfo
	Values   map[string]interface{} `json:"values"`
}

func main() {
	if err := godotenv.Load(); err != nil {
		fmt.Printf("Error loading .env file: %v\n", err)
		os.Exit(1)
	}

	dir := os.Getenv("STATES_DIR")
	if dir == "" {
		fmt.Println("STATES_DIR must be set in .env file")
		os.Exit(1)
	}

	lineageMap := make(map[string][]StateFile)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if the path is a directory
		if info.IsDir() {
			return nil
		}

		if filepath.Ext(path) != ".tfstate" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error reading %s: %v", path, err)
		}

		var state StateFile
		if err := json.Unmarshal(data, &state); err != nil {
			return fmt.Errorf("error parsing %s: %v", path, err)
		}

		state.FileInfo = info
		lineageMap[state.Lineage] = append(lineageMap[state.Lineage], state)

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking directory: %v\n", err)
		os.Exit(1)
	}

	// Create report file
	reportFileName := fmt.Sprintf("report_%s.txt", strings.ReplaceAll(filepath.Base(dir), "/", "_"))
	reportFile, err := os.Create(reportFileName)
	if err != nil {
		fmt.Printf("Error creating report file: %v\n", err)
		os.Exit(1)
	}
	defer reportFile.Close()

	report := func(format string, a ...interface{}) {
		fmt.Printf(format, a...)
		fmt.Fprintf(reportFile, format, a...)
	}

	report("States Directory: %s\n\n", dir)

	for lineage, states := range lineageMap {
		report("Lineage: %s\n", lineage)
		report("Found %d state files\n\n", len(states))

		// Sort by Serial
		sort.Slice(states, func(i, j int) bool {
			return states[i].Serial < states[j].Serial
		})

		var prevCount int
		var prevResourceTypes map[string]int
		var prevTimestamp time.Time
		for i, state := range states {
			report("File: %s\n", state.FileInfo.Name())
			report("Serial: %d\n", state.Serial)
			report("Lineage: %s\n", state.Lineage)

			// Extract timestamp from file name
			timestampStr := strings.TrimSuffix(state.FileInfo.Name(), filepath.Ext(state.FileInfo.Name()))
			// Assuming the timestamp is at the end of the filename, separated by a hyphen
			timestampStr = timestampStr[strings.LastIndex(timestampStr, "-")+1:]
			timestamp, err := time.Parse("20060102T150405Z", timestampStr)
			if err == nil {
				report("Timestamp: %s\n", timestamp.Format(time.RFC3339))
				if !prevTimestamp.IsZero() {
					timeDelta := timestamp.Sub(prevTimestamp)
					report("Time delta since last change: %s\n", timeDelta)
				}
				prevTimestamp = timestamp
			} else {
				report("Error parsing timestamp from file name: %v\n", err)
			}

			// Extract AWS caller identity more safely
			if state.Values != nil {
				if data, ok := state.Values["data"].(map[string]interface{}); ok {
					if aws, ok := data["aws_caller_identity"].(map[string]interface{}); ok {
						if current, ok := aws["current"].(map[string]interface{}); ok {
							if userId, ok := current["user_id"].(string); ok {
								report("AWS Caller ID: %s\n", userId)
							}
						}
					}
				}
			}

			resourceTypes := make(map[string]int)
			for _, r := range state.Resources {
				resourceTypes[r.Type]++
			}

			resourceCount := len(state.Resources)
			report("Resource count: %d\n", resourceCount)

			if i > 0 {
				diff := resourceCount - prevCount
				if diff != 0 {
					report("Change in resources: %+d\n", diff)
					report("\nResource type changes:\n")
					for rType, count := range resourceTypes {
						prevCount, exists := prevResourceTypes[rType]
						if !exists || prevCount != count {
							report("  %s: %d (was %d)\n", rType, count, prevCount)
						}
					}
				}
			}
			prevCount = resourceCount
			prevResourceTypes = resourceTypes
			report("---\n")
		}
	}
}
