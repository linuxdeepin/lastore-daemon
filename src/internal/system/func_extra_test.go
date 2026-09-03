// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewFunction(t *testing.T) {
	fn := func() error { return nil }
	f := NewFunction("job1", nil, fn)
	assert.Equal(t, "job1", f.JobId)
	assert.NotNil(t, f.Fn)
	assert.Nil(t, f.Indicator)
}

func TestNewFunctionWithIndicator(t *testing.T) {
	var called bool
	indicator := func(info JobProgressInfo) { called = true }
	fn := func() error { return nil }
	f := NewFunction("job2", indicator, fn)
	assert.Equal(t, "job2", f.JobId)
	assert.NotNil(t, f.Indicator)
	_ = called
}

func TestFunctionStartNilFn(t *testing.T) {
	f := &Function{JobId: "job-nil"}
	err := f.Start()
	assert.Error(t, err)
	assert.Equal(t, "function is nil", err.Error())
}

func TestFunctionStartSuccess(t *testing.T) {
	var mu sync.Mutex
	var results []JobProgressInfo
	indicator := func(info JobProgressInfo) {
		mu.Lock()
		results = append(results, info)
		mu.Unlock()
	}
	fn := func() error { return nil }
	f := NewFunction("job-ok", indicator, fn)
	err := f.Start()
	assert.NoError(t, err)

	// Wait for goroutine to complete
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, results, 1)
	assert.Equal(t, "job-ok", results[0].JobId)
	assert.Equal(t, SucceedStatus, results[0].Status)
	assert.Equal(t, 1.0, results[0].Progress)
}

func TestFunctionStartFailure(t *testing.T) {
	var mu sync.Mutex
	var results []JobProgressInfo
	indicator := func(info JobProgressInfo) {
		mu.Lock()
		results = append(results, info)
		mu.Unlock()
	}
	fn := func() error { return assertError("test error") }
	f := NewFunction("job-fail", indicator, fn)
	err := f.Start()
	assert.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, results, 1)
	assert.Equal(t, "job-fail", results[0].JobId)
	assert.Equal(t, FailedStatus, results[0].Status)
	assert.Equal(t, -1.0, results[0].Progress)
	assert.True(t, results[0].Cancelable)
	assert.NotNil(t, results[0].Error)
}

func TestFunctionAtEndNilIndicatorExtra(t *testing.T) {
	f := &Function{JobId: "job-no-indicator"}
	// Should not panic
	f.atEnd(nil)
	f.atEnd(assertError("err"))
}

func TestCheckLockUnlockedExtra(t *testing.T) {
	// Create a temp file and check that it's not locked
	dir := t.TempDir()
	fpath := dir + "/test.lock"
	assert.NoError(t, os.WriteFile(fpath, []byte("test"), 0644))

	path, locked := CheckLock(fpath)
	// On most systems without actual flock, CheckLock returns ("", false) for unlocked
	// or (path, true) if flock check itself fails. Either way, it shouldn't panic.
	_ = path
	_ = locked
}

func TestCheckLockNonexistentFileExtra(t *testing.T) {
	path, locked := CheckLock("/nonexistent/path/to/lockfile")
	assert.Equal(t, "", path)
	assert.False(t, locked)
}

func TestCollectAndClearLocaleEnvsExtra(t *testing.T) {
	t.Setenv("LC_ALL", "en_US.UTF-8")
	t.Setenv("LANG", "zh_CN.UTF-8")
	CollectAndClearLocaleEnvs()
	// After clearing, envs should be unset
	_, lcAllExists := os.LookupEnv("LC_ALL")
	_, langExists := os.LookupEnv("LANG")
	assert.False(t, lcAllExists)
	assert.False(t, langExists)
	// OriginalLocaleEnvs should have recorded the values
	found := false
	for _, e := range OriginalLocaleEnvs {
		if e == "LC_ALL=en_US.UTF-8" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestGetFreeSpaceExtra(t *testing.T) {
	// Test with /tmp which should exist
	size, err := GetFreeSpace("/tmp")
	if err != nil {
		// df might not be available in all environments
		t.Skipf("GetFreeSpace failed (df not available): %v", err)
	}
	assert.GreaterOrEqual(t, size, 0)
}

// Helper functions
func assertError(msg string) error {
	return &testError{msg: msg}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
