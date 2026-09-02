// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package apt

import (
	"strings"
	"testing"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/stretchr/testify/assert"
)

func TestParseProgressField(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{"valid float", "50.5", 50.5, false},
		{"zero", "0", 0, false},
		{"integer", "100", 100, false},
		{"invalid string", "abc", -1, true},
		{"empty", "", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseProgressField(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseProgressInfo(t *testing.T) {
	tests := []struct {
		name          string
		id            string
		line          string
		wantStatus    system.Status
		wantProgress  float64
		wantErr       bool
		wantCancelable bool
	}{
		{
			name:          "dlstatus",
			id:            "job1",
			line:          "dlstatus:running:50:Downloading",
			wantStatus:    system.RunningStatus,
			wantProgress:  0.5,
			wantCancelable: true,
		},
		{
			name:          "pmstatus",
			id:            "job1",
			line:          "pmstatus:running:50:Installing",
			wantStatus:    system.RunningStatus,
			wantProgress:  0.5,
			wantCancelable: false,
		},
		{
			name:          "dummy status",
			id:            "job1",
			line:          "dummy:running:100:Done",
			wantStatus:    system.RunningStatus,
			wantProgress:  100,
			wantCancelable: true,
		},
		{
			name:   "pmerror non-distupgrade",
			id:     "job1",
			line:   "pmerror:failed:-1:some error",
			wantStatus: system.FailedStatus,
			wantProgress: -1,
			wantCancelable: true,
		},
		{
			name:   "pmerror distupgrade no failure",
			id:     system.DistUpgradeJobType,
			line:   "pmerror:running:-1:some error",
			wantStatus: system.Status(""),
			wantProgress: -1,
		},
		{
			name:    "unknown status type",
			id:      "job1",
			line:    "pmconffile:running:50:config",
			wantErr: true,
		},
		{
			name:    "invalid line too few fields",
			id:      "job1",
			line:    "dlstatus:running",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := parseProgressInfo(tt.id, tt.line)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.id, info.JobId)
			assert.Equal(t, tt.wantStatus, info.Status)
			if tt.wantProgress != 0 || tt.name == "zero" {
				assert.InDelta(t, tt.wantProgress, info.Progress, 0.001)
			}
			if tt.name != "pmerror distupgrade no failure" {
				assert.Equal(t, tt.wantCancelable, info.Cancelable)
			}
		})
	}
}

func TestParseDeliveryDownloadInfo(t *testing.T) {
	t.Run("non-status line returns empty info", func(t *testing.T) {
		info, err := parseDeliveryDownloadInfo("job1", "some random line")
		assert.NoError(t, err)
		assert.Equal(t, "job1", info.JobId)
	})

	t.Run("status line with IsFinish true", func(t *testing.T) {
		line := "102 Status[{IsFinish true} {Speed 1024} {Proto http}]"
		info, err := parseDeliveryDownloadInfo("job1", line)
		assert.NoError(t, err)
		assert.Equal(t, "job1", info.JobId)
		assert.True(t, info.IsFinished)
		assert.Equal(t, int64(-1), info.Speed)
		assert.Equal(t, "http", info.Proto)
	})

	t.Run("status line with IsFinish false", func(t *testing.T) {
		line := "102 Status[{IsFinish false} {Speed 2048} {Proto https}]"
		info, err := parseDeliveryDownloadInfo("job1", line)
		assert.NoError(t, err)
		assert.False(t, info.IsFinished)
		assert.Equal(t, int64(2048), info.Speed)
		assert.Equal(t, "https", info.Proto)
	})

	t.Run("status line with invalid IsFinish defaults to true", func(t *testing.T) {
		line := "102 Status[{IsFinish notabool} {Speed 1024} {Proto http}]"
		info, err := parseDeliveryDownloadInfo("job1", line)
		assert.NoError(t, err)
		assert.True(t, info.IsFinished)
	})

	t.Run("status line with missing keys", func(t *testing.T) {
		line := "102 Status[{UnknownKey value}]"
		info, err := parseDeliveryDownloadInfo("job1", line)
		assert.NoError(t, err)
		assert.Equal(t, "job1", info.JobId)
	})
}

func TestParseJobError(t *testing.T) {
	tests := []struct {
		name     string
		stdErr   string
		stdOut   string
		wantType system.JobErrorType
	}{
		{"fetch failed", "E: Failed to fetch http://example.com/pkg.deb", "", system.ErrorFetchFailed},
		{"operation not permitted", "E: Failed to fetch http://example.com/pkg.deb: rename failed, Operation not permitted", "", system.ErrorOperationNotPermitted},
		{"insufficient space fetch", "E: Failed to fetch http://example.com/pkg.deb: No space left on device", "", system.ErrorInsufficientSpace},
		{"dpkg error", "Sub-process /usr/bin/dpkg returned an error code (1)", "", system.ErrorDpkgError},
		{"dpkg segfault", "Sub-process /usr/bin/dpkg received a segmentation fault.", "", system.ErrorDpkgError},
		{"pkg not found", "E: Unable to locate package vim", "", system.ErrorPkgNotFound},
		{"unmet dependencies", "Unable to correct problems, you have held broken packages", "The following packages have unmet dependencies:\n vim : Depends: x", system.ErrorUnmetDependencies},
		{"no installation candidate", "Package foo has no installation candidate", "", system.ErrorNoInstallationCandidate},
		{"insufficient space general", "You don't have enough free space", "", system.ErrorInsufficientSpace},
		{"no space general", "No space left on device", "", system.ErrorInsufficientSpace},
		{"unauthenticated packages", "There were unauthenticated packages", "", system.ErrorUnauthenticatedPackages},
		{"io error", "I/O error occurred", "", system.ErrorIO},
		{"permission denied", "don't have permission to access", "", system.ErrorOperationNotPermitted},
		{"damage package unpack", "dpkg: error processing archive foo.deb (--unpack)", "", system.ErrorDamagePackage},
		{"hash sum mismatch", "Hash Sum mismatch", "", system.ErrorDamagePackage},
		{"corrupted file", "Corrupted file", "", system.ErrorDamagePackage},
		{"invalid sources list", "The list of sources could not be read", "", system.ErrorInvalidSourcesList},
		{"unknown error", "Some random error", "", system.ErrorUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseJobError(tt.stdErr, tt.stdOut)
			assert.NotNil(t, err)
			assert.Equal(t, tt.wantType, err.ErrType)
		})
	}
}

func TestParsePkgSystemError(t *testing.T) {
	t.Run("empty error returns nil", func(t *testing.T) {
		err := parsePkgSystemError([]byte("output"), []byte{})
		assert.Nil(t, err)
	})

	t.Run("dpkg interrupted", func(t *testing.T) {
		err := parsePkgSystemError([]byte("out"), []byte("dpkg was interrupted"))
		assert.NotNil(t, err)
		je, ok := err.(*system.JobError)
		assert.True(t, ok)
		assert.Equal(t, system.ErrorDpkgInterrupted, je.ErrType)
	})

	t.Run("unmet dependencies", func(t *testing.T) {
		err := parsePkgSystemError([]byte("The following packages have unmet dependencies:"), []byte("Unmet dependencies"))
		assert.NotNil(t, err)
		je, ok := err.(*system.JobError)
		assert.True(t, ok)
		assert.Equal(t, system.ErrorDependenciesBroken, je.ErrType)
	})

	t.Run("generated breaks", func(t *testing.T) {
		err := parsePkgSystemError([]byte("some output"), []byte("generated breaks"))
		assert.NotNil(t, err)
		je, ok := err.(*system.JobError)
		assert.True(t, ok)
		assert.Equal(t, system.ErrorDependenciesBroken, je.ErrType)
	})

	t.Run("invalid sources list", func(t *testing.T) {
		err := parsePkgSystemError([]byte("out"), []byte("The list of sources could not be read"))
		assert.NotNil(t, err)
		je, ok := err.(*system.JobError)
		assert.True(t, ok)
		assert.Equal(t, system.ErrorInvalidSourcesList, je.ErrType)
	})

	t.Run("unknown error", func(t *testing.T) {
		err := parsePkgSystemError([]byte("out"), []byte("some unknown error"))
		assert.NotNil(t, err)
		je, ok := err.(*system.JobError)
		assert.True(t, ok)
		assert.Equal(t, system.ErrorUnknown, je.ErrType)
	})
}

func TestParsePkgSystemErrorWrapper(t *testing.T) {
	t.Run("empty error returns nil", func(t *testing.T) {
		err := ParsePkgSystemError([]byte("output"), []byte{})
		assert.Nil(t, err)
	})

	t.Run("non-empty error returns error", func(t *testing.T) {
		err := ParsePkgSystemError([]byte("output"), []byte("dpkg was interrupted"))
		assert.NotNil(t, err)
	})
}

func TestOptionToArgs(t *testing.T) {
	t.Run("nil options", func(t *testing.T) {
		args := OptionToArgs(nil)
		assert.Nil(t, args)
	})

	t.Run("empty options", func(t *testing.T) {
		args := OptionToArgs(map[string]string{})
		assert.Nil(t, args)
	})

	t.Run("with options", func(t *testing.T) {
		opts := map[string]string{
			"APT::Status-Fd":  "3",
			"Debug::NoLocking": "1",
		}
		args := OptionToArgs(opts)
		assert.Len(t, args, 4)
		// Each pair should be "-o", "key=value"
		for i := 0; i < len(args); i += 2 {
			assert.Equal(t, "-o", args[i])
			assert.Contains(t, args[i+1], "=")
		}
	})
}

func TestParseAptShowList(t *testing.T) {
	input := "Reading package lists...\nThe following packages will be upgraded:\n  vim git curl\n  wget\nDone\n"
	result := parseAptShowList(strings.NewReader(input), "The following packages will be upgraded:")
	assert.Contains(t, result, "vim")
	assert.Contains(t, result, "git")
	assert.Contains(t, result, "curl")
	assert.Contains(t, result, "wget")
}

func TestParseAptShowListNoMatch(t *testing.T) {
	input := "Reading package lists...\nNo upgrades available.\nDone\n"
	result := parseAptShowList(strings.NewReader(input), "The following packages will be upgraded:")
	assert.Empty(t, result)
}

func TestParseBackupJobError(t *testing.T) {
	t.Run("invalid json output", func(t *testing.T) {
		je := parseBackupJobError("stderr msg", "not json")
		assert.NotNil(t, je)
		assert.Equal(t, system.ErrorUnknown, je.ErrType)
	})

	t.Run("valid json with error", func(t *testing.T) {
		jsonOut := `{"code":1,"message":"failed","error":{"code":"ERR_001","message":["something went wrong"]}}`
		je := parseBackupJobError("stderr msg", jsonOut)
		assert.NotNil(t, je)
		assert.Equal(t, system.ErrorUnknown, je.ErrType)
		assert.Contains(t, je.ErrDetail, "ERR_001")
	})

	t.Run("valid json code 0 gets corrected to 1", func(t *testing.T) {
		jsonOut := `{"code":0,"message":"ok"}`
		je := parseBackupJobError("stderr msg", jsonOut)
		assert.NotNil(t, je)
		assert.Contains(t, je.ErrDetail, "1")
	})

	t.Run("valid json with nil error field", func(t *testing.T) {
		jsonOut := `{"code":2,"message":"failed"}`
		je := parseBackupJobError("stderr msg", jsonOut)
		assert.NotNil(t, je)
		assert.Contains(t, je.ErrDetail, "failed")
	})
}

func TestValidatePackageNames(t *testing.T) {
	tests := []struct {
		name    string
		pkgs    []string
		wantErr bool
	}{
		{"valid single", []string{"vim"}, false},
		{"valid multiple", []string{"vim", "emacs", "gcc"}, false},
		{"valid with arch", []string{"vim:amd64"}, false},
		{"valid with version", []string{"vim=2:9.0.0"}, false},
		{"valid complex name", []string{"libgtk-3-0"}, false},
		{"empty slice", []string{}, false},
		{"dash prefix", []string{"-vim"}, true},
		{"uppercase", []string{"Vim"}, true},
		{"option injection", []string{"--allow-unauthenticated"}, true},
		{"space in name", []string{"vim test"}, true},
		{"valid and invalid mix", []string{"vim", "-evil"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePackageNames(tt.pkgs)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
