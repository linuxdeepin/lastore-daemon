package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/spf13/cobra"
)

var postProcessEventCmd = &cobra.Command{
	Use:   "post_process_event",
	Short: "Post upgrade process event to update platform",
	Long:  "Send upgrade process events to the update platform for tracking",
	Run:   runPostProcessEvent,
}

var (
	eventTaskID   int
	eventType     ProcessEventType
	eventStatus   bool
	eventContent  string
	eventDataFile string
)

func init() {
	postProcessEventCmd.Flags().IntVarP(&eventTaskID, "task-id", "t", 0, "Task ID")
	postProcessEventCmd.Flags().IntVarP((*int)(&eventType), "event-type", "e", 0, "Event type (1:CheckEnv, 2:GetUpdate, 3:StartDownload, 4:DownloadComplete, 5:StartBackUp, 6:BackUpComplete, 7:StartInstall)")
	postProcessEventCmd.Flags().BoolVarP(&eventStatus, "status", "s", false, "Event status (true for success, false for failure)")
	postProcessEventCmd.Flags().StringVarP(&eventContent, "content", "c", "", "Event content/message")
	postProcessEventCmd.Flags().StringVarP(&eventDataFile, "data-file", "f", "", "JSON file containing event data (alternative to flags)")
	rootCmd.AddCommand(postProcessEventCmd)
}

func runPostProcessEvent(cmd *cobra.Command, args []string) {
	var event *ProcessEvent

	// 从文件读取或从命令行参数构建
	if eventDataFile != "" {
		data, err := os.ReadFile(eventDataFile)
		if err != nil {
			logger.Warningf("failed to read data file: %v", err)
			os.Exit(1)
		}
		event = &ProcessEvent{}
		err = json.Unmarshal(data, event)
		if err != nil {
			logger.Warningf("failed to parse JSON data: %v", err)
			os.Exit(1)
		}
	} else {
		event = &ProcessEvent{
			TaskID:       eventTaskID,
			EventType:    eventType,
			EventStatus:  eventStatus,
			EventContent: eventContent,
		}
	}
	logger.Debugf("Process Event: %s", spew.Sdump(event))

	response, err := genPostProcessEventResponse(updatePlatform.requestURL, updatePlatform.Token, event)
	if err != nil {
		logger.Warningf("genPostProcessEventResponse failed: %v", err)
		os.Exit(1)
	}

	data, err := getResponseData(response, PostProcessEvent)
	if err != nil {
		logger.Warningf("getResponseData failed: %v", err)
		os.Exit(1)
	}

	logger.Infof("Process event posted successfully")
	logger.Debugf("Response data: %s", string(data))
}

func genPostProcessEventResponse(requestUrl, token string, event *ProcessEvent) (*http.Response, error) {
	policyUrl := requestUrl + Urls[PostProcessEvent].path
	client := &http.Client{
		Timeout: 40 * time.Second,
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event data: %v", err)
	}

	body := bytes.NewBuffer(eventData)
	request, err := http.NewRequest(Urls[PostProcessEvent].method, policyUrl, body)
	if err != nil {
		return nil, fmt.Errorf("%v new request failed: %v ", PostProcessEvent.string(), err.Error())
	}
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(token)))
	request.Header.Set("Content-Type", "application/json")
	return client.Do(request)
}
