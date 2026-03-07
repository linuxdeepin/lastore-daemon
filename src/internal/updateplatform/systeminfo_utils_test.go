// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package updateplatform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSystemInfoUtil(t *testing.T) {
	sys := getSystemInfo(true, true)
	assert.NotEmpty(t, sys)
}

func TestGetDefaultRouteIface(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name: "default route found",
			output: `Kernel IP routing table
Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
0.0.0.0         10.20.33.1      0.0.0.0         UG    100    0        0 eno1
10.20.33.0      0.0.0.0         255.255.255.0   U     100    0        0 eno1
`,
			want: "eno1",
		},
		{
			name: "no default route",
			output: `Kernel IP routing table
Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
10.20.33.0      0.0.0.0         255.255.255.0   U     100    0        0 eno1
`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, getDefaultRouteIface(tt.output))
		})
	}
}
