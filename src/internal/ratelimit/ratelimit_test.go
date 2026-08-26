// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package ratelimit

import (
	"encoding/json"
	"testing"
)

func TestRateInfoSameAsConfig(t *testing.T) {
	noLimit := RateInfo{
		LimitType:   RateLimitTypeNo,
		LimitRate:   DefaultRateLimit,
		CurrentRate: DefaultRateLimit,
	}
	localLimit := func(rate int64) RateInfo {
		return RateInfo{
			LimitType:   RateLimitTypeLocal,
			LimitRate:   rate,
			CurrentRate: rate,
		}
	}
	remoteLimit := RateInfo{
		LimitType:   RateLimitTypeRemote,
		LimitRate:   2048 * 1024,
		CurrentRate: 2048 * 1024,
	}
	noLimitPreservedRate := RateInfo{
		LimitType:   RateLimitTypeNo,
		LimitRate:   2048 * 1024,
		CurrentRate: 2048 * 1024,
	}

	tests := []struct {
		name      string
		configStr string
		rateInfo  RateInfo
		want      bool
	}{
		{
			name:      "empty config is never same",
			configStr: "",
			rateInfo:  noLimit,
			want:      false,
		},
		{
			name:      "invalid config is never same",
			configStr: "not-json",
			rateInfo:  noLimit,
			want:      false,
		},
		{
			name:      "no limit matches no limit ignoring rate",
			configStr: marshalRateInfo(t, noLimitPreservedRate),
			rateInfo:  noLimit,
			want:      true,
		},
		{
			name:      "no limit does not match local limit",
			configStr: marshalRateInfo(t, localLimit(2048*1024)),
			rateInfo:  noLimit,
			want:      false,
		},
		{
			name:      "local limit matches same rate",
			configStr: marshalRateInfo(t, localLimit(2048*1024)),
			rateInfo:  localLimit(2048 * 1024),
			want:      true,
		},
		{
			name:      "local limit does not match different rate",
			configStr: marshalRateInfo(t, localLimit(2048*1024)),
			rateInfo:  localLimit(4096 * 1024),
			want:      false,
		},
		{
			name:      "local limit does not match no limit",
			configStr: marshalRateInfo(t, noLimit),
			rateInfo:  localLimit(2048 * 1024),
			want:      false,
		},
		{
			name:      "unsupported rate limit type is never same",
			configStr: marshalRateInfo(t, remoteLimit),
			rateInfo:  remoteLimit,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rateInfo.SameAsConfig(tt.configStr); got != tt.want {
				t.Fatalf("SameAsConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func marshalRateInfo(t *testing.T, info RateInfo) string {
	t.Helper()
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("failed to marshal rate info: %v", err)
	}
	return string(data)
}
