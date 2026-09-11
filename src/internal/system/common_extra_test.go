// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetEditionName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "os-version")
	require.NoError(t, os.WriteFile(path, []byte("[Version]\nEditionName=Community\n"), 0644))

	edition, err := getEditionNameFromFile(path)
	assert.NoError(t, err)
	assert.Equal(t, "Community", edition)
}

func TestIsActiveCodeExistSystemBusError(t *testing.T) {
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", "unix:path=/tmp/lastore-nonexistent-dbus-socket")
	assert.False(t, IsActiveCodeExist())
}

func TestGetEditionNameFromVar(t *testing.T) {
	path := filepath.Join(t.TempDir(), "os-version")
	require.NoError(t, os.WriteFile(path, []byte("[Version]\nEditionName=Community\n"), 0644))

	old := osVersionFile
	osVersionFile = path
	t.Cleanup(func() { osVersionFile = old })

	edition, err := getEditionName()
	assert.NoError(t, err)
	assert.Equal(t, "Community", edition)
}

func TestIsAuthorizedCommunity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "os-version")
	require.NoError(t, os.WriteFile(path, []byte("[Version]\nEditionName=Community\n"), 0644))

	old := osVersionFile
	osVersionFile = path
	t.Cleanup(func() { osVersionFile = old })

	assert.True(t, IsAuthorized())
}

func TestIsAuthorizedNoVersionFile(t *testing.T) {
	old := osVersionFile
	osVersionFile = filepath.Join(t.TempDir(), "nonexistent-os-version")
	t.Cleanup(func() { osVersionFile = old })

	assert.False(t, IsAuthorized())
}

func setNonCommunityEdition(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "os-version")
	require.NoError(t, os.WriteFile(path, []byte("[Version]\nEditionName=Professional\n"), 0644))

	old := osVersionFile
	osVersionFile = path
	t.Cleanup(func() { osVersionFile = old })
}

func TestIsAuthorizedAuthorizedState(t *testing.T) {
	setNonCommunityEdition(t)
	old := licenseAuthorizationStateFn
	licenseAuthorizationStateFn = func() (int32, error) { return int32(Authorized), nil }
	t.Cleanup(func() { licenseAuthorizationStateFn = old })

	assert.True(t, IsAuthorized())
}

func TestIsAuthorizedTrialAuthorizedState(t *testing.T) {
	setNonCommunityEdition(t)
	old := licenseAuthorizationStateFn
	licenseAuthorizationStateFn = func() (int32, error) { return int32(TrialAuthorized), nil }
	t.Cleanup(func() { licenseAuthorizationStateFn = old })

	assert.True(t, IsAuthorized())
}

func TestIsAuthorizedUnauthorizedState(t *testing.T) {
	setNonCommunityEdition(t)
	old := licenseAuthorizationStateFn
	licenseAuthorizationStateFn = func() (int32, error) { return int32(Unauthorized), nil }
	t.Cleanup(func() { licenseAuthorizationStateFn = old })

	assert.False(t, IsAuthorized())
}

func TestIsAuthorizedLicenseError(t *testing.T) {
	setNonCommunityEdition(t)
	old := licenseAuthorizationStateFn
	licenseAuthorizationStateFn = func() (int32, error) { return 0, assert.AnError }
	t.Cleanup(func() { licenseAuthorizationStateFn = old })

	assert.False(t, IsAuthorized())
}

func TestIsAuthorizedNoSystemBus(t *testing.T) {
	setNonCommunityEdition(t)
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", "unix:path=/tmp/lastore-nonexistent-dbus-socket")
	assert.False(t, IsAuthorized())
}

func TestIsActiveCodeExistPresent(t *testing.T) {
	old := licenseActiveCodeFn
	licenseActiveCodeFn = func() (string, error) { return "ABC-123-456", nil }
	t.Cleanup(func() { licenseActiveCodeFn = old })

	assert.True(t, IsActiveCodeExist())
}

func TestIsActiveCodeExistBlank(t *testing.T) {
	old := licenseActiveCodeFn
	licenseActiveCodeFn = func() (string, error) { return "   \n\t", nil }
	t.Cleanup(func() { licenseActiveCodeFn = old })

	assert.False(t, IsActiveCodeExist())
}

func TestIsActiveCodeExistError(t *testing.T) {
	old := licenseActiveCodeFn
	licenseActiveCodeFn = func() (string, error) { return "", assert.AnError }
	t.Cleanup(func() { licenseActiveCodeFn = old })

	assert.False(t, IsActiveCodeExist())
}

func TestGetEditionNameFromFileMissingKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "os-version")
	require.NoError(t, os.WriteFile(path, []byte("[Version]\nSomeOther=value\n"), 0644))

	_, err := getEditionNameFromFile(path)
	assert.Error(t, err)
}

func TestGetFreeSpaceError(t *testing.T) {
	_, err := GetFreeSpace("/nonexistent/path/xyz123")
	assert.Error(t, err)
}
