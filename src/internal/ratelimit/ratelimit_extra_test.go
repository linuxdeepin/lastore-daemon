// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package ratelimit

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
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

func TestSetIPFSDownloadRateLimitSystemBusError(t *testing.T) {
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", "unix:path=/tmp/lastore-nonexistent-dbus-socket")
	assert.Error(t, SetIPFSDownloadRateLimit(1024))
}

func TestSetIPFSUploadRateLimitSystemBusError(t *testing.T) {
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", "unix:path=/tmp/lastore-nonexistent-dbus-socket")
	assert.Error(t, SetIPFSUploadRateLimit(1024))
}

func TestGetDeliveryUploadRateLimitSystemBusError(t *testing.T) {
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", "unix:path=/tmp/lastore-nonexistent-dbus-socket")
	_, err := GetDeliveryUploadRateLimit()
	assert.Error(t, err)
}

func TestGetDeliveryDownloadRateLimitSystemBusError(t *testing.T) {
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", "unix:path=/tmp/lastore-nonexistent-dbus-socket")
	_, err := GetDeliveryDownloadRateLimit()
	assert.Error(t, err)
}

func TestGetDeliveryRateLimitSystemBusError(t *testing.T) {
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", "unix:path=/tmp/lastore-nonexistent-dbus-socket")
	ev, err := getDeliveryRateLimit("DownloadLimitSpeed")
	assert.Error(t, err)
	assert.Equal(t, RateInfoEvent{}, ev)
}

// fakeBusObject is a minimal dbus.BusObject double used to exercise the D-Bus
// call/get-property branches without connecting to the real system bus.
type fakeBusObject struct {
	callErr     error
	callMethod  string
	callFlags   dbus.Flags
	callArgs    []interface{}
	property    dbus.Variant
	propertyErr error
	gotProperty string
}

func (f *fakeBusObject) Call(method string, flags dbus.Flags, args ...interface{}) *dbus.Call {
	f.callMethod = method
	f.callFlags = flags
	f.callArgs = args
	return &dbus.Call{Err: f.callErr}
}

func (f *fakeBusObject) CallWithContext(ctx context.Context, method string, flags dbus.Flags, args ...interface{}) *dbus.Call {
	return f.Call(method, flags, args...)
}

func (f *fakeBusObject) Go(method string, flags dbus.Flags, ch chan *dbus.Call, args ...interface{}) *dbus.Call {
	return f.Call(method, flags, args...)
}

func (f *fakeBusObject) GoWithContext(ctx context.Context, method string, flags dbus.Flags, ch chan *dbus.Call, args ...interface{}) *dbus.Call {
	return f.Call(method, flags, args...)
}

func (f *fakeBusObject) AddMatchSignal(iface, member string, options ...dbus.MatchOption) *dbus.Call {
	return &dbus.Call{}
}

func (f *fakeBusObject) RemoveMatchSignal(iface, member string, options ...dbus.MatchOption) *dbus.Call {
	return &dbus.Call{}
}

func (f *fakeBusObject) GetProperty(p string) (dbus.Variant, error) {
	f.gotProperty = p
	return f.property, f.propertyErr
}

func (f *fakeBusObject) StoreProperty(p string, value interface{}) error {
	return nil
}

func (f *fakeBusObject) SetProperty(p string, v interface{}) error {
	return nil
}

func (f *fakeBusObject) Destination() string {
	return UPGRADE_DELIVERY_SERVICE
}

func (f *fakeBusObject) Path() dbus.ObjectPath {
	return dbus.ObjectPath(UPGRADE_DELIVERY_OBJECT_PATH)
}

// stubUpgradeDeliveryBusObject replaces getUpgradeDeliveryBusObject with a fake
// that always returns the supplied object, restoring the original on cleanup.
func stubUpgradeDeliveryBusObject(t *testing.T, fake *fakeBusObject) {
	t.Helper()
	old := getUpgradeDeliveryBusObject
	getUpgradeDeliveryBusObject = func() (dbus.BusObject, error) {
		return fake, nil
	}
	t.Cleanup(func() { getUpgradeDeliveryBusObject = old })
}

func TestSetIPFSDownloadRateLimitCall(t *testing.T) {
	t.Run("no limit -1 passes through to dbus", func(t *testing.T) {
		fake := &fakeBusObject{}
		stubUpgradeDeliveryBusObject(t, fake)
		assert.NoError(t, SetIPFSDownloadRateLimit(-1))
		assert.Equal(t, UPGRADE_DELIVERY_INTERFACE+".SetDownloadRateLimit", fake.callMethod)
		assert.Equal(t, []interface{}{-1}, fake.callArgs)
	})

	t.Run("call error propagated", func(t *testing.T) {
		fake := &fakeBusObject{callErr: errors.New("dbus call failed")}
		stubUpgradeDeliveryBusObject(t, fake)
		err := SetIPFSDownloadRateLimit(1024)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to set download rate limit")
	})
}

func TestSetIPFSUploadRateLimitCall(t *testing.T) {
	t.Run("no limit -1 passes through to dbus", func(t *testing.T) {
		fake := &fakeBusObject{}
		stubUpgradeDeliveryBusObject(t, fake)
		assert.NoError(t, SetIPFSUploadRateLimit(-1))
		assert.Equal(t, UPGRADE_DELIVERY_INTERFACE+".SetUploadRateLimit", fake.callMethod)
		assert.Equal(t, []interface{}{-1}, fake.callArgs)
	})

	t.Run("call error propagated", func(t *testing.T) {
		fake := &fakeBusObject{callErr: errors.New("dbus call failed")}
		stubUpgradeDeliveryBusObject(t, fake)
		err := SetIPFSUploadRateLimit(2048)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to set upload rate limit")
	})
}

func TestGetDeliveryRateLimitParsing(t *testing.T) {
	t.Run("valid rate info event decode", func(t *testing.T) {
		fake := &fakeBusObject{property: dbus.MakeVariant(`{"LimitType":1,"LimitRate":102400,"CurrentRate":102400,"RateType":2,"Speed":102400}`)}
		stubUpgradeDeliveryBusObject(t, fake)
		ev, err := getDeliveryRateLimit("DownloadLimitSpeed")
		assert.NoError(t, err)
		assert.Equal(t, UPGRADE_DELIVERY_INTERFACE+".DownloadLimitSpeed", fake.gotProperty)
		assert.Equal(t, 1, ev.LimitType)
		assert.Equal(t, int64(102400), ev.LimitRate)
		assert.Equal(t, int64(102400), ev.CurrentRate)
		assert.Equal(t, 2, ev.RateType)
		assert.Equal(t, int64(102400), ev.Speed)
	})

	t.Run("missing fields decode to zero values", func(t *testing.T) {
		fake := &fakeBusObject{property: dbus.MakeVariant(`{"LimitType":2}`)}
		stubUpgradeDeliveryBusObject(t, fake)
		ev, err := getDeliveryRateLimit("UploadLimitSpeed")
		assert.NoError(t, err)
		assert.Equal(t, 2, ev.LimitType)
		assert.Equal(t, int64(0), ev.LimitRate)
		assert.Equal(t, int64(0), ev.CurrentRate)
		assert.Equal(t, 0, ev.RateType)
		assert.Equal(t, int64(0), ev.Speed)
	})

	t.Run("invalid json", func(t *testing.T) {
		fake := &fakeBusObject{property: dbus.MakeVariant(`{not-json`)}
		stubUpgradeDeliveryBusObject(t, fake)
		_, err := getDeliveryRateLimit("DownloadLimitSpeed")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal limit speed")
	})

	t.Run("empty speed", func(t *testing.T) {
		fake := &fakeBusObject{property: dbus.MakeVariant("")}
		stubUpgradeDeliveryBusObject(t, fake)
		_, err := getDeliveryRateLimit("DownloadLimitSpeed")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "limit speed is empty")
	})

	t.Run("get property error", func(t *testing.T) {
		fake := &fakeBusObject{propertyErr: errors.New("no such property")}
		stubUpgradeDeliveryBusObject(t, fake)
		_, err := getDeliveryRateLimit("DownloadLimitSpeed")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get limit speed")
	})
}
