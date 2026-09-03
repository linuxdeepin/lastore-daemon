// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package cache

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCheckTag(t *testing.T) {
	st := reflect.TypeOf(InternalState{})

	for i := 0; i < st.NumField(); i++ {
		field := st.Field(i)
		tag := GetCheckTag(field)
		if field.Name == "IsPreCheck" {
			assert.Equal(t, "PreCheck", tag)
		} else {
			assert.Empty(t, tag, "field %s should have empty cktag", field.Name)
		}
	}
}

func TestGetCheckTagNoTag(t *testing.T) {
	type testStruct struct {
		Field1 string `json:"field1"`
		Field2 string `json:"field2" cktag:"MyTag"`
	}
	st := reflect.TypeOf(testStruct{})

	f1, _ := st.FieldByName("Field1")
	assert.Equal(t, "", GetCheckTag(f1))

	f2, _ := st.FieldByName("Field2")
	assert.Equal(t, "MyTag", GetCheckTag(f2))
}
