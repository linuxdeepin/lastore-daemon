package main

import (
	"archive/tar"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus/v5"
	ConfigManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
)

// UpdatePlatformManager 更新平台管理器
type UpdatePlatformManager struct {
	requestURL      string
	Token           string
	machineID       string
	currentBaseline string
	targetBaseline  string
}

const (
	dSettingsAppID          = "org.deepin.dde.lastore"
	dSettingsLastoreName    = "org.deepin.dde.lastore"
	dSettingsKeyPlatformUrl = "platform-url"
)

// getTokenFromAptConfig 从 apt-config 获取 Token
func getTokenFromAptConfig() string {
	cmd := exec.Command("apt-config", "dump", "Acquire::SmartMirrors::Token")
	output, err := cmd.Output()
	if err != nil {
		logger.Warningf("failed to get token from apt-config: %v", err)
		return ""
	}

	// 解析输出: Acquire::SmartMirrors::Token "token_value";
	line := strings.TrimSpace(string(output))
	if line == "" {
		logger.Warning("apt-config returned empty output")
		return ""
	}

	// 查找双引号之间的内容
	startIdx := strings.Index(line, "\"")
	if startIdx == -1 {
		logger.Warningf("failed to parse token: no opening quote found in: %s", line)
		return ""
	}
	endIdx := strings.LastIndex(line, "\"")
	if endIdx == -1 || endIdx <= startIdx {
		logger.Warningf("failed to parse token: no closing quote found in: %s", line)
		return ""
	}

	token := line[startIdx+1 : endIdx]
	logger.Debugf("Token loaded from apt-config: %s", token)
	return token
}

func extractMachineIDFromToken(token string) string {
	if token == "" {
		return ""
	}

	// Token format: a=value;b=value;i=machineID;...
	fields := strings.Split(token, ";")
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if strings.HasPrefix(field, "i=") {
			machineID := strings.TrimPrefix(field, "i=")
			logger.Debugf("Machine ID extracted from token: %s", machineID)
			return machineID
		}
	}

	logger.Warning("Machine ID (i=) not found in token")
	return ""
}

// getPlatformURLFromDSettings 从 dSettings 获取平台 URL
func getPlatformURLFromDSettings() string {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warningf("failed to get system bus: %v", err)
		return ""
	}

	ds := ConfigManager.NewConfigManager(sysBus)
	dsPath, err := ds.AcquireManager(0, dSettingsAppID, dSettingsLastoreName, "")
	if err != nil {
		logger.Warningf("failed to acquire dSettings manager: %v", err)
		return ""
	}

	dsManager, err := ConfigManager.NewManager(sysBus, dsPath)
	if err != nil {
		logger.Warningf("failed to create dSettings manager: %v", err)
		return ""
	}

	v, err := dsManager.Value(0, dSettingsKeyPlatformUrl)
	if err != nil {
		logger.Warningf("failed to get platform URL from dSettings: %v", err)
		return ""
	}

	url := v.Value().(string)
	logger.Debugf("Platform URL loaded from dSettings: %s", url)
	return url
}

// getClientPackageInfo 获取客户端包信息
func getClientPackageInfo(clientPackageName string) string {
	_ = clientPackageName
	return "client=lastore-daemon&version=6.2.45"
}

// getResponseData 解析 HTTP 响应数据
func getResponseData(response *http.Response, reqType requestType) (json.RawMessage, error) {
	if http.StatusOK == response.StatusCode {
		respData, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf("%v failed to read response body: %v ", response.Request.RequestURI, err.Error())
		}
		logger.Debugf("%v request for %v respData:%s ", reqType.string(), response.Request.URL, string(respData))
		msg := &tokenMessage{}
		err = json.Unmarshal(respData, msg)
		if err != nil {
			logger.Warningf("%v request for %v respData:%s ", reqType.string(), response.Request.URL, string(respData))
			return nil, fmt.Errorf("%v failed to Unmarshal respData to tokenMessage: %v ", reqType.string(), err.Error())
		}
		if !msg.Result {
			logger.Warningf("result is not true! %v request for %v respData:%s ", reqType.string(), response.Request.URL, string(respData))
			errorMsg := &tokenErrorMessage{}
			err = json.Unmarshal(respData, errorMsg)
			if err != nil {
				return nil, fmt.Errorf("%v request for %s", reqType.string(), response.Request.RequestURI)
			}
			return nil, fmt.Errorf("%v request for %s err:%s", reqType.string(), response.Request.RequestURI, errorMsg.Msg)
		}
		return msg.Data, nil
	} else {
		return nil, fmt.Errorf("request for %s failed, response code=%d", response.Request.RequestURI, response.StatusCode)
	}
}

// marshalJSON marshal object to JSON bytes
func marshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// logRequestHeaders logs all headers of an HTTP request
func logRequestHeaders(request *http.Request) {
	logger.Debugf("Request headers:")
	for key, values := range request.Header {
		for _, value := range values {
			logger.Debugf("  %s: %s", key, value)
		}
	}
}

// tarFiles create a tar archive from multiple files
func tarFiles(files []string, outFile string) error {
	// Create tar file
	tarFile, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("create tar failed: %v", err)
	}
	defer tarFile.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(tarFile)
	defer tarWriter.Close()

	// Add files to tar
	for _, filePath := range files {
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("open file failed: %v", err)
		}
		defer file.Close()

		// Get file info
		info, err := file.Stat()
		if err != nil {
			return fmt.Errorf("get file info failed: %v", err)
		}

		// Create tar header
		header := &tar.Header{
			Name:    filepath.Base(filePath),
			Size:    info.Size(),
			Mode:    int64(info.Mode()),
			ModTime: info.ModTime(),
		}

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("write tar header failed: %v", err)
		}

		// Write file content
		if _, err := io.Copy(tarWriter, file); err != nil {
			return fmt.Errorf("write tar content failed: %v", err)
		}
	}

	return nil
}

// Encryption constants and functions for message encryption
const (
	BlockSize = 32
	randomLen = 16 // random value length
)

const encodingAesKey = secret

// EncryptMsg encrypts message using AES-CBC
func EncryptMsg(data []byte) ([]byte, error) {
	// Get 16-byte random string and prepend to plaintext
	replyMsgBytes, err := GetRandomBytes(randomLen)
	if err != nil {
		return nil, err
	}
	replyMsgBytes = append(replyMsgBytes, data...)
	// Use first 16 bytes of key as AES-CBC IV
	iv := []byte(Substr(encodingAesKey, 0, 16))

	// Generate ciphertext based on key
	block, err := aes.NewCipher([]byte(encodingAesKey))
	if err != nil {
		logger.Warningf("[encrypt] aes-cbc encrypt data failed, error:%v", err)
		return nil, err
	}
	encodeBytes := replyMsgBytes
	encodeBytes = PKCS7Encode(encodeBytes, BlockSize)
	blockMode := cipher.NewCBCEncrypter(block, iv)
	crypted := make([]byte, len(encodeBytes))
	if len(crypted)%block.BlockSize() != 0 {
		return nil, errors.New("encrypt failed, input not full blocks")
	}
	blockMode.CryptBlocks(crypted, encodeBytes)
	return crypted, nil
}

// PKCS7Encode pads plaintext for encryption
func PKCS7Encode(text []byte, blockSize int) []byte {
	textLen := len(text)
	padding := blockSize - (textLen % blockSize)
	if padding == 0 {
		padding = blockSize
	}

	paddingByte := bytes.Repeat([]byte(string(rune(padding))), padding)

	return append(text, paddingByte...)
}

// GetRandomBytes generates random bytes of specified length
func GetRandomBytes(length uint32) ([]byte, error) {
	res := make([]byte, length)
	_, err := rand.Read(res)
	if err != nil {
		logger.Warning(err)
		return nil, err
	}
	return res, nil
}

// Substr extracts substring from string
func Substr(str string, start int, length int) string {
	if start < 0 {
		start = 0
	}
	if length < 0 {
		length = 0
	}

	strSlice := strings.Split(str, "")
	strSliceLen := len(strSlice)

	end := start + length

	if end > strSliceLen {
		end = strSliceLen
	}
	if start > strSliceLen {
		start = strSliceLen
	}

	return strings.Join(strSlice[start:end], "")
}
