package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/spf13/cobra"
)

var postProcessCmd = &cobra.Command{
	Use:   "post_process",
	Short: "Post process status message or log files to update platform",
	Long:  "Send status messages or compressed log files to the update platform for tracking",
	Run:   runPostProcess,
}

const (
	MessageTypeInfo    = "info"
	MessageTypeWarning = "warning"
	MessageTypeError   = "error"
)

const secret = "DflXyFwTmaoGmbDkVj8uD62XGb01pkJn"

var (
	processMessageType     string
	processUpdateType      string
	processJobDesc         string
	processDetail          string
	processLogFiles        []string
	processDataFile        string
	processCurrentBaseline string
	processTargetBaseline  string
)

func init() {
	postProcessCmd.Flags().StringVarP(&processMessageType, "message-type", "m", MessageTypeInfo, "Message type (info/warning/error)")
	postProcessCmd.Flags().StringVarP(&processUpdateType, "update-type", "u", "", "Update type")
	postProcessCmd.Flags().StringVarP(&processJobDesc, "job-desc", "j", "", "Job description")
	postProcessCmd.Flags().StringVarP(&processDetail, "detail", "d", "", "Message detail")
	postProcessCmd.Flags().StringSliceVarP(&processLogFiles, "log-files", "l", nil, "Log files to upload (comma-separated)")
	postProcessCmd.Flags().StringVarP(&processDataFile, "data-file", "f", "", "JSON file containing status message data")
	postProcessCmd.Flags().StringVarP(&processCurrentBaseline, "current-baseline", "c", "", "Current baseline version")
	postProcessCmd.Flags().StringVarP(&processTargetBaseline, "target-baseline", "t", "", "Target baseline version")
	rootCmd.AddCommand(postProcessCmd)
}

func runPostProcess(cmd *cobra.Command, args []string) {
	// Handle log files upload
	if len(processLogFiles) > 0 {
		uploadLogFiles(processLogFiles)
		return
	}

	// Handle status message
	var buf *bytes.Buffer
	if processDataFile != "" {
		// Read from file
		data, err := os.ReadFile(processDataFile)
		if err != nil {
			logger.Warningf("failed to read data file: %v", err)
			os.Exit(1)
		}
		buf = bytes.NewBuffer(data)
	} else {
		// Build from command line arguments
		message := StatusMessage{
			Type:           processMessageType,
			UpdateType:     processUpdateType,
			JobDescription: processJobDesc,
			Detail:         processDetail,
		}

		logger.Debugf("Status Message: %s", spew.Sdump(message))

		data, err := json.Marshal(message)
		if err != nil {
			logger.Warningf("failed to marshal status message: %v", err)
			os.Exit(1)
		}
		logger.Debugf("Status Message JSON (buf): %s", data)
		buf = bytes.NewBuffer(data)
	}

	// Generate temporary file path
	filePath := fmt.Sprintf("/tmp/%s_%s.xz", "update", time.Now().Format("20060102150405"))

	updatePlatform.currentBaseline = processCurrentBaseline
	updatePlatform.targetBaseline = processTargetBaseline

	// Send request
	response, err := genPostProcessResponse(updatePlatform.requestURL, updatePlatform.Token, buf, filePath)
	if err != nil {
		logger.Warningf("genPostProcessResponse failed: %v", err)
		os.Exit(1)
	}

	data, err := getResponseData(response, PostProcess)
	if err != nil {
		logger.Warningf("getResponseData failed: %v", err)
		os.Exit(1)
	}

	logger.Infof("Process status message posted successfully")
	logger.Debugf("Response data: %s", string(data))
}

func uploadLogFiles(files []string) {
	logger.Infof("Uploading %d log files", len(files))

	// Validate all files exist
	for _, file := range files {
		if _, err := os.Stat(file); err != nil {
			logger.Warningf("file does not exist: %s", file)
			os.Exit(1)
		}
	}

	// Create tar archive
	outFilename := fmt.Sprintf("/tmp/%s_%s.tar", "update_logs", time.Now().Format("20060102150405"))
	err := tarFiles(files, outFilename)
	if err != nil {
		logger.Warningf("failed to tar log files: %v", err)
		os.Exit(1)
	}

	// Open tar file
	tarFile, err := os.Open(outFilename)
	if err != nil {
		logger.Warningf("failed to open tar file: %v", err)
		os.Exit(1)
	}
	defer tarFile.Close()

	// Upload
	xzFilePath := outFilename + ".xz"
	response, err := genPostProcessResponse(updatePlatform.requestURL, updatePlatform.Token, tarFile, xzFilePath)
	if err != nil {
		logger.Warningf("failed to upload log files: %v", err)
		os.Exit(1)
	}

	data, err := getResponseData(response, PostProcess)
	if err != nil {
		logger.Warningf("getResponseData failed: %v", err)
		os.Exit(1)
	}

	logger.Infof("Log files uploaded successfully")
	logger.Debugf("Response data: %s", string(data))

	// Clean up tar file if not in debug mode
	if !globalDebug {
		_ = os.Remove(outFilename)
	}
}

func genPostProcessResponse(requestURL, token string, buf io.Reader, filePath string) (*http.Response, error) {
	policyURL := requestURL + Urls[PostProcess].path
	client := newHTTPClient()

	// Clean up xz file if not in debug mode
	if !globalDebug {
		defer os.RemoveAll(filePath)
	}

	// Create xz compressed file
	xzFile, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("create file failed: %v", err)
	}

	xzCmd := exec.Command("xz", "-z", "-c")
	xzCmd.Stdin = buf
	xzCmd.Stdout = xzFile
	if err := xzCmd.Run(); err != nil {
		_ = xzFile.Close()
		return nil, fmt.Errorf("exec xz command failed: %v", err)
	}
	_ = xzFile.Close()

	// Calculate signature
	hash := sha256.New()
	xTime := fmt.Sprintf("%d", time.Now().Unix())

	xzFileContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read xz file failed: %v", err)
	}
	body := bytes.NewBuffer(xzFileContent)

	hash.Write([]byte(fmt.Sprintf("%s%s%s", secret, xTime, xzFileContent)))
	sign := base64.StdEncoding.EncodeToString([]byte(hex.EncodeToString(hash.Sum(nil))))

	// Create HTTP request
	request, err := http.NewRequest(Urls[PostProcess].method, policyURL, body)
	if err != nil {
		return nil, fmt.Errorf("%v new request failed: %v", PostProcess.string(), err.Error())
	}

	// Set headers
	request.Header.Set("X-Time", xTime)
	request.Header.Set("X-Sign", sign)
	request.Header.Set("X-Repo-Token", base64.RawStdEncoding.EncodeToString([]byte(token)))
	request.Header.Set("X-MachineID", updatePlatform.machineID)
	request.Header.Set("X-CurrentBaseline", updatePlatform.currentBaseline)
	request.Header.Set("X-Baseline", updatePlatform.targetBaseline)

	logRequestHeaders(request)
	return client.Do(request)
}
