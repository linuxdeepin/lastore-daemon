/*
 * Copyright (C) 2017 ~ 2017 Deepin Technology Co., Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package utils

import (
	"os"
	"testing"
)

func TestRemoteFingerprint(t *testing.T) {
	if os.Getenv("NO_TEST_NETWORK") == "1" {
		t.Skip()
	}
	line, err := RemoteCatLine("http://packages.deepin.com/deepin/fixme/index")
	if err != nil {
		t.Fatal("E:", err)
	}
	if len(line) != 32 {
		t.Fatalf("%q not a md5sum value", line)
	}
}
