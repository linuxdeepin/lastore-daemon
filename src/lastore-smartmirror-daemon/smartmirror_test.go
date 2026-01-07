package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/linuxdeepin/go-lib/log"
)

func TestSmartMirror_makeChoice(t *testing.T) {
	if os.Getenv("NO_TEST_NETWORK") == "1" {
		t.Skip("Skipping test due to NO_TEST_NETWORK=1")
	}
	logger.SetLogLevel(log.LevelDebug)
	// Create temporary test directory
	tempDir := t.TempDir()
	defer func() {
		for i := 0; i < 5; i++ {
			err := os.RemoveAll(tempDir)
			if err == nil {
				break
			}
			if i < 4 {
				time.Sleep(100 * time.Millisecond)
			} else {
				t.Logf("warn: Failed to remove temp directory after 5 attempts: %v", err)
			}
		}
	}()
	stateDirectory = tempDir

	// Prepare test data
	testSources := []map[string]interface{}{
		{
			"id":   "default",
			"name": "official",
			// NOTE: Intentionally using an incorrect CDN address here
			"url":    "https://x-cdn-community-packages.deepin.com/",
			"weight": 100,
		},
		{
			"id":           "TUNA",
			"name":         "[CN] Tsinghua University",
			"url":          "http://mirrors.tuna.tsinghua.edu.cn/deepin/",
			"name_locale":  map[string]string{"zh_CN": "[CN] 清华大学", "zh_TW": "[CN] 清華大學"},
			"weight":       60000,
			"country":      "CN",
			"adjust_delay": 0,
		},
	}

	// Write mirrors.json
	mirrorsData, _ := json.MarshalIndent(testSources, "", "  ")
	err := os.WriteFile(filepath.Join(tempDir, "mirrors.json"), mirrorsData, 0644)
	if err != nil {
		t.Fatalf("Failed to create test mirrors.json: %v", err)
	}

	// Write empty config file
	configData := []byte(`{"enable":true}`)
	err = os.WriteFile(filepath.Join(tempDir, "smartmirror_config.json"), configData, 0644)
	if err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Write quality data file
	qualityData := map[string]interface{}{
		"https://x-cdn-community-packages.deepin.com/": map[string]int{
			"detect_count":  50,
			"access_count":  50,
			"failed_count":  5,
			"average_delay": 150,
		},
		"http://mirrors.tuna.tsinghua.edu.cn/deepin/": map[string]int{
			"detect_count":  80,
			"access_count":  80,
			"failed_count":  2,
			"average_delay": 110,
		},
	}
	qualityJSON, _ := json.MarshalIndent(qualityData, "", "  ")
	err = os.WriteFile(filepath.Join(tempDir, "smartmirror_quality.json"), qualityJSON, 0644)
	if err != nil {
		t.Fatalf("Failed to create test quality.json: %v", err)
	}

	// Create SmartMirror instance
	s := newSmartMirror(nil)
	originalURL := "https://community-packages.deepin.com/beige/pool/main/f/fish/fish_3.7.1-1deepin1_amd64.deb"
	officialMirror := "https://community-packages.deepin.com/"

	t.Logf("Loaded sources count: %d", len(s.sources))
	for i, source := range s.sources {
		t.Logf("Source[%d]: %s - %s", i, source.Name, source.Url)
	}

	result := s.makeChoice(originalURL, officialMirror)
	t.Logf("Result URL: %s", result)

	// Verify result is not empty
	if result == "" {
		t.Error("makeChoice returned empty result")
	}
}
