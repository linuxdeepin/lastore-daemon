// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package ratelimit

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestConvertRateLimitWithTimeToRateInfo(t *testing.T) {
	tests := []struct {
		name   string
		rlwt   *RateLimitWithTime
		assert func(t *testing.T, ri *RateInfo)
	}{
		{
			name: "nil input returns nil",
			rlwt: nil,
			assert: func(t *testing.T, ri *RateInfo) {
				assert.Nil(t, ri)
			},
		},
		{
			name: "type 1 remote limit with time",
			rlwt: &RateLimitWithTime{
				StartTime: "22:00:00",
				EndTime:   "06:00:00",
				RateLimit: 1024,
				Type:      1,
			},
			assert: func(t *testing.T, ri *RateInfo) {
				assert.NotNil(t, ri)
				assert.Equal(t, RateLimitTypeRemote, ri.LimitType)
				assert.Equal(t, int64(1024*1024), ri.LimitRate)
				assert.Equal(t, int64(1024*1024), ri.CurrentRate)
				expectedStart, _ := time.Parse("15:04:05", "22:00:00")
				assert.Equal(t, expectedStart, ri.StartTime)
				expectedEnd, _ := time.Parse("15:04:05", "06:00:00")
				assert.Equal(t, expectedEnd, ri.EndTime)
			},
		},
		{
			name: "type 0 no limit",
			rlwt: &RateLimitWithTime{
				RateLimit: 0,
				Type:      0,
			},
			assert: func(t *testing.T, ri *RateInfo) {
				assert.NotNil(t, ri)
				assert.Equal(t, RateLimitTypeNo, ri.LimitType)
				assert.Equal(t, int64(DefaultRateLimit), ri.LimitRate)
				assert.Equal(t, int64(DefaultRateLimit), ri.CurrentRate)
			},
		},
		{
			name: "invalid time format ignored",
			rlwt: &RateLimitWithTime{
				StartTime: "invalid",
				EndTime:   "also-invalid",
				RateLimit: 512,
				Type:      1,
			},
			assert: func(t *testing.T, ri *RateInfo) {
				assert.NotNil(t, ri)
				assert.True(t, ri.StartTime.IsZero())
				assert.True(t, ri.EndTime.IsZero())
			},
		},
		{
			name: "empty time strings",
			rlwt: &RateLimitWithTime{
				StartTime: "",
				EndTime:   "",
				RateLimit: 2048,
				Type:      1,
			},
			assert: func(t *testing.T, ri *RateInfo) {
				assert.NotNil(t, ri)
				assert.True(t, ri.StartTime.IsZero())
				assert.True(t, ri.EndTime.IsZero())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ri := convertRateLimitWithTimeToRateInfo(tt.rlwt)
			tt.assert(t, ri)
		})
	}
}

func TestValidateRateInfo(t *testing.T) {
	t.Run("nil does nothing", func(t *testing.T) {
		ValidateRateInfo(nil)
	})

	t.Run("valid rate unchanged", func(t *testing.T) {
		ri := &RateInfo{LimitRate: 100 * 1024, CurrentRate: 200 * 1024}
		ValidateRateInfo(ri)
		assert.Equal(t, int64(100*1024), ri.LimitRate)
		assert.Equal(t, int64(200*1024), ri.CurrentRate)
	})

	t.Run("below min rate reset to default", func(t *testing.T) {
		ri := &RateInfo{LimitRate: 100, CurrentRate: 100}
		ValidateRateInfo(ri)
		assert.Equal(t, int64(DefaultRateLimit), ri.LimitRate)
		assert.Equal(t, int64(DefaultRateLimit), ri.CurrentRate)
	})

	t.Run("above max rate reset to default", func(t *testing.T) {
		ri := &RateInfo{LimitRate: MaxRateLimit + 1, CurrentRate: MaxRateLimit + 1}
		ValidateRateInfo(ri)
		assert.Equal(t, int64(DefaultRateLimit), ri.LimitRate)
		assert.Equal(t, int64(DefaultRateLimit), ri.CurrentRate)
	})

	t.Run("exactly min rate unchanged", func(t *testing.T) {
		ri := &RateInfo{LimitRate: MinRateLimit, CurrentRate: MinRateLimit}
		ValidateRateInfo(ri)
		assert.Equal(t, int64(MinRateLimit), ri.LimitRate)
		assert.Equal(t, int64(MinRateLimit), ri.CurrentRate)
	})

	t.Run("exactly max rate unchanged", func(t *testing.T) {
		ri := &RateInfo{LimitRate: MaxRateLimit, CurrentRate: MaxRateLimit}
		ValidateRateInfo(ri)
		assert.Equal(t, int64(MaxRateLimit), ri.LimitRate)
		assert.Equal(t, int64(MaxRateLimit), ri.CurrentRate)
	})
}

func TestLocalRateLimitConfigValidate(t *testing.T) {
	t.Run("all nil", func(t *testing.T) {
		c := &LocalRateLimitConfig{}
		c.Validate()
		assert.Nil(t, c.Global)
		assert.Nil(t, c.Busy)
		assert.Nil(t, c.Free)
	})

	t.Run("invalid rates corrected", func(t *testing.T) {
		c := &LocalRateLimitConfig{
			Global: &RateInfo{LimitRate: 1, CurrentRate: 1},
			Busy:   &RateInfo{LimitRate: MaxRateLimit + 1, CurrentRate: MaxRateLimit + 1},
			Free:   &RateInfo{LimitRate: 100 * 1024, CurrentRate: 100 * 1024},
		}
		c.Validate()
		assert.Equal(t, int64(DefaultRateLimit), c.Global.LimitRate)
		assert.Equal(t, int64(DefaultRateLimit), c.Busy.LimitRate)
		assert.Equal(t, int64(100*1024), c.Free.LimitRate)
	})
}

func TestGetLocalRateLimitFromConfig(t *testing.T) {
	validRateInfo := RateInfo{
		LimitType:   RateLimitTypeLocal,
		LimitRate:   100 * 1024,
		CurrentRate: 100 * 1024,
	}
	validJSON, _ := json.Marshal(validRateInfo)

	t.Run("all empty strings", func(t *testing.T) {
		c := GetLocalRateLimitFromConfig("", "", "")
		assert.NotNil(t, c)
		assert.Nil(t, c.Global)
		assert.Nil(t, c.Busy)
		assert.Nil(t, c.Free)
	})

	t.Run("valid json for all", func(t *testing.T) {
		c := GetLocalRateLimitFromConfig(string(validJSON), string(validJSON), string(validJSON))
		assert.NotNil(t, c.Global)
		assert.Equal(t, RateLimitTypeLocal, c.Global.LimitType)
		assert.NotNil(t, c.Busy)
		assert.NotNil(t, c.Free)
	})

	t.Run("invalid json ignored", func(t *testing.T) {
		c := GetLocalRateLimitFromConfig("invalid", "{bad", "")
		assert.Nil(t, c.Global)
		assert.Nil(t, c.Busy)
		assert.Nil(t, c.Free)
	})

	t.Run("partial valid", func(t *testing.T) {
		c := GetLocalRateLimitFromConfig(string(validJSON), "", "")
		assert.NotNil(t, c.Global)
		assert.Nil(t, c.Busy)
		assert.Nil(t, c.Free)
	})
}

func TestGetIPFSLimitRateBySyncLimit(t *testing.T) {
	t.Run("empty sync limit", func(t *testing.T) {
		lr, err := GetIPFSLimitRateBySyncLimit(SyncLimit{})
		assert.NoError(t, err)
		assert.Nil(t, lr.GlobalLimitRemote)
		assert.Nil(t, lr.BusyLimitRemote)
		assert.Nil(t, lr.FreeLimitRemote)
	})

	t.Run("all day limit with type 1", func(t *testing.T) {
		sl := SyncLimit{
			AllDayRateLimit: &RateLimitWithTime{RateLimit: 1024, Type: 1},
		}
		lr, err := GetIPFSLimitRateBySyncLimit(sl)
		assert.NoError(t, err)
		assert.NotNil(t, lr.GlobalLimitRemote)
		assert.Equal(t, RateLimitTypeRemote, lr.GlobalLimitRemote.LimitType)
	})

	t.Run("busy limit with type 0 not set", func(t *testing.T) {
		sl := SyncLimit{
			BusyTimeRateLimit: &RateLimitWithTime{RateLimit: 1024, Type: 0},
		}
		lr, err := GetIPFSLimitRateBySyncLimit(sl)
		assert.NoError(t, err)
		assert.Nil(t, lr.BusyLimitRemote)
	})

	t.Run("busy and free limits with type 1", func(t *testing.T) {
		sl := SyncLimit{
			BusyTimeRateLimit: &RateLimitWithTime{RateLimit: 512, Type: 1, StartTime: "09:00:00", EndTime: "18:00:00"},
			FreeTimeRateLimit: &RateLimitWithTime{RateLimit: 2048, Type: 1, StartTime: "18:00:00", EndTime: "09:00:00"},
		}
		lr, err := GetIPFSLimitRateBySyncLimit(sl)
		assert.NoError(t, err)
		assert.NotNil(t, lr.BusyLimitRemote)
		assert.Equal(t, int64(512*1024), lr.BusyLimitRemote.LimitRate)
		assert.NotNil(t, lr.FreeLimitRemote)
		assert.Equal(t, int64(2048*1024), lr.FreeLimitRemote.LimitRate)
	})

	t.Run("all day with type 0 sets no limit", func(t *testing.T) {
		sl := SyncLimit{
			AllDayRateLimit: &RateLimitWithTime{RateLimit: 1024, Type: 0},
		}
		lr, err := GetIPFSLimitRateBySyncLimit(sl)
		assert.NoError(t, err)
		assert.NotNil(t, lr.GlobalLimitRemote)
		assert.Equal(t, RateLimitTypeNo, lr.GlobalLimitRemote.LimitType)
		assert.Equal(t, int64(DefaultRateLimit), lr.GlobalLimitRemote.LimitRate)
	})
}

func TestSameAsConfig(t *testing.T) {
	tests := []struct {
		name      string
		rate      RateInfo
		configStr string
		want      bool
	}{
		{
			name:      "empty config string",
			rate:      RateInfo{LimitType: RateLimitTypeNo},
			configStr: "",
			want:      false,
		},
		{
			name:      "invalid JSON config",
			rate:      RateInfo{LimitType: RateLimitTypeNo},
			configStr: "{bad json}",
			want:      false,
		},
		{
			name:      "no limit type matches",
			rate:      RateInfo{LimitType: RateLimitTypeNo},
			configStr: `{"LimitType":0}`,
			want:      true,
		},
		{
			name:      "no limit type does not match local",
			rate:      RateInfo{LimitType: RateLimitTypeNo},
			configStr: `{"LimitType":1}`,
			want:      false,
		},
		{
			name:      "local limit type matches with same rate",
			rate:      RateInfo{LimitType: RateLimitTypeLocal, LimitRate: 10240},
			configStr: `{"LimitType":1,"LimitRate":10240}`,
			want:      true,
		},
		{
			name:      "local limit type does not match with different rate",
			rate:      RateInfo{LimitType: RateLimitTypeLocal, LimitRate: 10240},
			configStr: `{"LimitType":1,"LimitRate":20480}`,
			want:      false,
		},
		{
			name:      "local limit type does not match with no limit config",
			rate:      RateInfo{LimitType: RateLimitTypeLocal, LimitRate: 10240},
			configStr: `{"LimitType":0}`,
			want:      false,
		},
		{
			name:      "remote limit type returns false",
			rate:      RateInfo{LimitType: RateLimitTypeRemote},
			configStr: `{"LimitType":2}`,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.rate.SameAsConfig(tt.configStr))
		})
	}
}
func TestSetIPFSRateLimitSystemBusError(t *testing.T) {
	// Force dbus.SystemBus() to fail fast at connect (nonexistent socket).
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", "unix:path=/tmp/lastore-nonexistent-dbus-socket")

	// Fully populate all six limit-rate pointers with LimitType == RateLimitTypeNo,
	// which exercises every nil-out conditional plus the two json.Marshal calls.
	noLimit := &RateInfo{LimitType: RateLimitTypeNo, LimitRate: DefaultRateLimit, CurrentRate: DefaultRateLimit}
	upload := IPFSLimitRate{
		GlobalLimitRemote: noLimit,
		GlobalLimitLocal:  &RateInfo{LimitType: RateLimitTypeLocal, LimitRate: DefaultRateLimit, CurrentRate: DefaultRateLimit},
		BusyLimitRemote:   noLimit,
		BusyLimitLocal:    noLimit,
		FreeLimitRemote:   noLimit,
		FreeLimitLocal:    noLimit,
	}
	download := IPFSLimitRate{
		GlobalLimitRemote: noLimit,
		GlobalLimitLocal:  &RateInfo{LimitType: RateLimitTypeLocal, LimitRate: DefaultRateLimit, CurrentRate: DefaultRateLimit},
		BusyLimitRemote:   noLimit,
		BusyLimitLocal:    noLimit,
		FreeLimitRemote:   noLimit,
		FreeLimitLocal:    noLimit,
	}

	err := SetIPFSRateLimit(upload, download)
	assert.Error(t, err)
}
