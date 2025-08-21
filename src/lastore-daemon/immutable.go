package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
)

// 格式化输出需要添加-j 参数
func (i *immutableManager) osTreeCmd(args []string) (out string, err error) {
	if system.NormalFileExists(system.DeepinImmutableCtlPath) {
		cmd := exec.Command(system.DeepinImmutableCtlPath, args...) // #nosec G204
		cmd.Env = append(os.Environ(), "IMMUTABLE_DISABLE_REMOUNT=false")
		cmd.Env = append(cmd.Env, originalLocaleEnvs...)
		logger.Info("run command:", cmd.Args)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		defer func() {
			i.indicator(system.JobProgressInfo{
				OnlyLog:     true,
				OriginalLog: fmt.Sprintf("=== Ostree %v end ===\n", cmd.Args),
			})
		}()

		i.indicator(system.JobProgressInfo{
			OnlyLog:     true,
			OriginalLog: fmt.Sprintf("=== Ostree cmd running: %v ===\n", cmd.Args),
		})
		err = cmd.Run()
		if err != nil {
			return "", fmt.Errorf("%v", stderr.String())
		} else {
			return stdout.String(), nil
		}
	} else {
		return "", fmt.Errorf("%v not found", system.DeepinImmutableCtlPath)
	}
}

type ostreeError struct {
	Code    string   `json:"code"`
	Message []string `json:"message"`
}

type ostreeRollbackData struct {
	Version     int    `json:"version"`
	CanRollback bool   `json:"can_rollback"`
	Time        int64  `json:"time"`
	Name        string `json:"name"`
	Auto        bool   `json:"auto"`
	Reboot      bool   `json:"reboot"`
}

type ostreeResponse struct {
	Code    uint8           `json:"code"`
	Message string          `json:"message"`
	Error   *ostreeError    `json:"error"`
	Data    json.RawMessage `json:"data"`
}

type immutableManager struct {
	indicator system.Indicator
}

func newImmutableManager(indicator system.Indicator) *immutableManager {
	return &immutableManager{indicator: indicator}
}

func (i *immutableManager) osTreeRefresh() error {
	_, err := i.osTreeCmd([]string{"admin", "deploy", "--refresh"})
	if err != nil {
		return err
	}
	return nil
}

func (i *immutableManager) osTreeFinalize() error {
	_, err := i.osTreeCmd([]string{"admin", "deploy", "--finalize"})
	if err != nil {
		return err
	}
	return nil
}

func (i *immutableManager) osTreeRollback() error {
	_, err := i.osTreeCmd([]string{"admin", "rollback"})
	if err != nil {
		return err
	}
	return nil
}

func (i *immutableManager) osTreeParseRollbackData() (string, error) {
	out, err := i.osTreeCmd([]string{"admin", "rollback", "--can-rollback", "-j"})
	if err != nil {
		logger.Warning("osTreeCmd failed:", err)
		return "", err
	}

	logger.Info("osTree rollback output:", out)

	var resp ostreeResponse
	err = json.Unmarshal([]byte(out), &resp)
	if err != nil {
		logger.Warning("unmarshal ostree response failed:", err)
		return "", err
	}

	if resp.Error != nil {
		logger.Warning("ostree response has error:", resp.Error)
		return "", fmt.Errorf("ostree error: %v", resp.Error)
	}

	return string(resp.Data), nil
}

func (i *immutableManager) osTreeCanRollback() (bool, string) {
	dataJson, err := i.osTreeParseRollbackData()
	if err != nil {
		return false, ""
	}

	var data ostreeRollbackData
	err = json.Unmarshal([]byte(dataJson), &data)
	if err != nil {
		return false, ""
	}

	return data.CanRollback, dataJson
}

func (i *immutableManager) osTreeNeedRebootAfterRollback() bool {
	dataJson, err := i.osTreeParseRollbackData()
	if err != nil {
		return false
	}

	var data ostreeRollbackData
	err = json.Unmarshal([]byte(dataJson), &data)
	if err != nil {
		return false
	}

	// 兼容旧版本，Auto为true时，表示不需要重启
	var rawData map[string]json.RawMessage
	if err := json.Unmarshal([]byte(dataJson), &rawData); err == nil {
		if _, hasReboot := rawData["reboot"]; !hasReboot {
			return !data.Auto
		}
	}

	return data.Reboot
}
