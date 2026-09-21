// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package smartmirror

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestProtocolRoundTripAndRepeatedFields(t *testing.T) {
	in := "600 URI Acquire\nURI: https://example.invalid/pool/a.deb\nConfig-Item: A=one\nConfig-Item: A=two\n\n"
	m, err := ReadMessage(bufio.NewReader(strings.NewReader(in)))
	if err != nil {
		t.Fatal(err)
	}
	if m.Code != 600 || m.Get("URI") == "" {
		t.Fatalf("%+v", m)
	}
	if got := Configuration(m)["A"]; got != "two" {
		t.Fatalf("config=%q", got)
	}
	var out bytes.Buffer
	if err := WriteMessage(&out, m.Without("URI").With("URI", "https://example.invalid/pool/b.deb")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "URI: https://example.invalid/pool/b.deb\n") {
		t.Fatal(out.String())
	}
}

func TestProtocolRejectsMalformedAndUnsafeMessages(t *testing.T) {
	if _, err := ReadMessage(bufio.NewReader(strings.NewReader("600\n"))); err == nil {
		t.Fatal("missing field separator accepted")
	}
	var out bytes.Buffer
	if err := WriteMessage(&out, Message{Code: 400, Text: "x\nbad"}); err == nil {
		t.Fatal("newline text accepted")
	}
}
