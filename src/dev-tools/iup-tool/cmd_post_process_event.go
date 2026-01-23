package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

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
	eventTaskID          int
	eventType            ProcessEventType
	eventStatus          bool
	eventContent         string // message
	eventDataFile        string
	eventCurrentBaseline string
	eventTargetBaseline  string
)

func init() {
	postProcessEventCmd.Flags().IntVarP(&eventTaskID, "task-id", "T", 0, "Task ID")
	postProcessEventCmd.Flags().IntVarP((*int)(&eventType), "event-type", "e", 0, "Event type (1:CheckEnv, 2:GetUpdate, 3:StartDownload, 4:DownloadComplete, 5:StartBackUp, 6:BackUpComplete, 7:StartInstall)")
	postProcessEventCmd.Flags().BoolVarP(&eventStatus, "status", "s", false, "Event status (true for success, false for failure)")
	postProcessEventCmd.Flags().StringVarP(&eventContent, "message", "m", "", "Event content/message")
	postProcessEventCmd.Flags().StringVarP(&eventDataFile, "data-file", "f", "", "JSON file containing event data (alternative to flags)")
	postProcessEventCmd.Flags().StringVarP(&eventCurrentBaseline, "current-baseline", "c", "", "Current baseline version")
	postProcessEventCmd.Flags().StringVarP(&eventTargetBaseline, "target-baseline", "t", "", "Target baseline version")
	rootCmd.AddCommand(postProcessEventCmd)
}

func runPostProcessEvent(cmd *cobra.Command, args []string) {
	var event *ProcessEvent

	// Read from file or build from command line arguments
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

	// Limit event content length to 950 characters (consistent with message_report.go)
	if len(event.EventContent) >= 950 {
		event.EventContent = event.EventContent[:950]
	}

	// Validate event type (must be 1 to MaxEventType-1)
	if !event.EventType.IsValid() {
		logger.Warningf("invalid event type: %d, must be between %d and %d", event.EventType, CheckEnv, MaxProcessEventType-1)
		// Build valid event types description dynamically
		var validTypes []string
		for i := CheckEnv; i < MaxProcessEventType; i++ {
			validTypes = append(validTypes, fmt.Sprintf("%d=%s", i, i.String()))
		}
		logger.Warningf("valid event types: %s", strings.Join(validTypes, ", "))
		os.Exit(1)
	}

	logger.Debugf("Process Event: %s", spew.Sdump(event))

	// Set baseline versions to updatePlatform
	updatePlatform.currentBaseline = eventCurrentBaseline
	updatePlatform.targetBaseline = eventTargetBaseline

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
	client := newHTTPClient()

	// Marshal event to JSON
	eventData, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event data: %v", err)
	}

	logger.Debugf("upgrade post process event msg is %v", string(eventData))

	// Encrypt message (consistent with message_report.go)
	encryptMsg, err := EncryptMsg(eventData)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt message: %v", err)
	}

	// Create request with base64-encoded encrypted data
	body := strings.NewReader(base64.StdEncoding.EncodeToString(encryptMsg))
	request, err := http.NewRequest(Urls[PostProcessEvent].method, policyUrl, body)
	if err != nil {
		return nil, fmt.Errorf("%v new request failed: %v ", PostProcessEvent.string(), err.Error())
	}

	// Set headers (consistent with message_report.go)
	request.Header.Set("X-MachineID", updatePlatform.machineID)
	request.Header.Set("X-CurrentBaseline", updatePlatform.currentBaseline)
	request.Header.Set("X-Baseline", updatePlatform.targetBaseline)
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(token)))

	logRequestHeaders(request)
	return client.Do(request)
}
