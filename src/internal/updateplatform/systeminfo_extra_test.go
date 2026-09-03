// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunLsCpu(t *testing.T) {
	result, err := runLsCpu()
	// lscpu should be available on the test system
	if err != nil {
		t.Skip("lscpu not available:", err)
	}
	assert.NotNil(t, result, "runLsCpu should return non-nil map")
}

func TestGetDefaultMac(t *testing.T) {
	mac, err := getDefaultMac()
	// route command may or may not be available; just verify it doesn't panic
	_ = mac
	_ = err
}

func TestGetDefaultRouteIfaceEmpty(t *testing.T) {
	result := getDefaultRouteIface("")
	assert.Empty(t, result, "getDefaultRouteIface with empty string should return empty")
}

func TestGetDefaultRouteIfaceNoDefault(t *testing.T) {
	input := "Kernel IP routing table\nDestination     Gateway         Genmask         Flags Metric Ref    Use Iface\n192.168.1.0     0.0.0.0         255.255.255.0   U     0      0        0 eth0\n"
	result := getDefaultRouteIface(input)
	assert.Empty(t, result, "getDefaultRouteIface with no default route should return empty")
}

func TestGetDefaultRouteIfaceWithDefault(t *testing.T) {
	input := "Kernel IP routing table\nDestination     Gateway         Genmask         Flags Metric Ref    Use Iface\n0.0.0.0         192.168.1.1     0.0.0.0         UG    100    0        0 eth0\n"
	result := getDefaultRouteIface(input)
	assert.Equal(t, "eth0", result, "getDefaultRouteIface should return the default route interface")
}

func TestGetMachineType(t *testing.T) {
	result := getMachineType()
	// Just verify it doesn't panic; result may be empty if DMI files don't exist
	_ = result
}

func TestGetProjectIDNonExistent(t *testing.T) {
	_, err := getProjectID("/nonexistent/path/xyz123.json")
	assert.Error(t, err, "getProjectID should return error for non-existent file")
}

func TestGetProjectIDValid(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "info.json")
	err := os.WriteFile(tmpFile, []byte(`{"id":"test-project-id"}`), 0644)
	require.NoError(t, err)

	id, err := getProjectID(tmpFile)
	require.NoError(t, err)
	assert.Equal(t, "test-project-id", id)
}

func TestGetProjectIDInvalidJSON(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "info.json")
	err := os.WriteFile(tmpFile, []byte(`invalid json`), 0644)
	require.NoError(t, err)

	_, err = getProjectID(tmpFile)
	assert.Error(t, err, "getProjectID should return error for invalid JSON")
}

func TestGetOEMIDNonExistent(t *testing.T) {
	// /etc/.oemid typically doesn't exist
	_, err := getOEMID()
	// Just verify it doesn't panic; error is expected if file doesn't exist
	_ = err
}

func TestLoadFileNonExistent(t *testing.T) {
	_, err := loadFile("/nonexistent/path/xyz123")
	assert.Error(t, err, "loadFile should return error for non-existent file")
}

func TestLoadFileValid(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	err := os.WriteFile(tmpFile, []byte("line1\nline2\nline3\n"), 0644)
	require.NoError(t, err)

	lines, err := loadFile(tmpFile)
	require.NoError(t, err)
	assert.Len(t, lines, 3)
	assert.Equal(t, "line1", lines[0])
	assert.Equal(t, "line2", lines[1])
	assert.Equal(t, "line3", lines[2])
}

func TestGetOSVersionInfoNonExistent(t *testing.T) {
	_, err := GetOSVersionInfo("/nonexistent/path/xyz123")
	assert.Error(t, err, "GetOSVersionInfo should return error for non-existent file")
}

func TestGetOSVersionInfoValid(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "os-version")
	content := "[Version]\nSystemName=Deepin\nProductType=Community\nEditionName=Community\nMajorVersion=25\nMinorVersion=1\nOsBuild=1000\n"
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	info, err := GetOSVersionInfo(tmpFile)
	require.NoError(t, err)
	assert.Equal(t, "Deepin", info["SystemName"])
	assert.Equal(t, "Community", info["ProductType"])
	assert.Equal(t, "25", info["MajorVersion"])
}

func TestGetOSVersionInfoMissingKeys(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "os-version")
	content := "SystemName=Deepin\n"
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	_, err = GetOSVersionInfo(tmpFile)
	assert.Error(t, err, "GetOSVersionInfo should return error when necessary keys are missing")
}

func TestGetArchInfo(t *testing.T) {
	arch, err := GetArchInfo()
	// dpkg --print-architecture should work on the test system
	if err != nil {
		t.Skip("dpkg not available:", err)
	}
	assert.NotEmpty(t, arch, "GetArchInfo should return non-empty architecture")
}

func TestGetProcessorInfoNonExistent(t *testing.T) {
	_, err := getProcessorInfo("/nonexistent/path/xyz123")
	assert.Error(t, err, "getProcessorInfo should return error for non-existent file")
}

func TestGetProcessorInfoValid(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "cpuinfo")
	content := "processor\t: 0\nmodel name\t: Test CPU Model\nprocessor\t: 1\nmodel name\t: Test CPU Model\n"
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	cpu, err := getProcessorInfo(tmpFile)
	require.NoError(t, err)
	assert.Equal(t, "Test CPU Model", cpu)
}

func TestGetCPUInfoFromMapFound(t *testing.T) {
	data := map[string]string{"model name": "Intel i7"}
	result, err := getCPUInfoFromMap("model name", data)
	require.NoError(t, err)
	assert.Equal(t, "Intel i7", result)
}

func TestGetCPUInfoFromMapNotFound(t *testing.T) {
	data := map[string]string{"other key": "value"}
	_, err := getCPUInfoFromMap("model name", data)
	assert.Error(t, err)
}

func TestGetCPUInfoFromMapMultipleSpaces(t *testing.T) {
	data := map[string]string{"model name": "Intel  Core   i7"}
	result, err := getCPUInfoFromMap("model name", data)
	require.NoError(t, err)
	assert.Equal(t, "Intel Core i7", result)
}

func TestParseInfoFileNonExistent(t *testing.T) {
	_, err := parseInfoFile("/nonexistent/path/xyz123", ":")
	assert.Error(t, err)
}

func TestParseInfoFileValid(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "info.txt")
	content := "key1:value1\nkey2:value2\n"
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	data, err := parseInfoFile(tmpFile, ":")
	require.NoError(t, err)
	assert.Equal(t, "value1", data["key1"])
	assert.Equal(t, "value2", data["key2"])
}

func TestGetHardwareVersion(t *testing.T) {
	_, err := getHardwareVersion()
	// dmidecode may require root; just verify it doesn't panic
	_ = err
}

func TestGetClientPackageInfoUpdatePlatform(t *testing.T) {
	result := getClientPackageInfo("lastore-daemon")
	// dpkg-query may fail; just verify it doesn't panic
	_ = result
}
