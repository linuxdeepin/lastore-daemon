// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package ratelimit

import (
		"testing"
	
	"github.com/stretchr/testify/assert"
)

func TestSameAsConfig(t *testing.T) {
	tests := []struct {
		name       string
		rate       RateInfo
		configStr  string
		want       bool
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





