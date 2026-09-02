// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package coremodules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorError(t *testing.T) {
	tests := []struct {
		err  Error
		want string
	}{
		{Error{Code: 1, Ext: 0, Msg: "not found"}, "Code: 1, Ext: 0, Msg: not found"},
		{Error{Code: 500, Ext: 3, Msg: "internal error"}, "Code: 500, Ext: 3, Msg: internal error"},
		{Error{Code: 0, Ext: 0, Msg: ""}, "Code: 0, Ext: 0, Msg: "},
	}
	for _, tt := range tests {
		got := tt.err.Error()
		assert.Equal(t, tt.want, got)
	}
}
