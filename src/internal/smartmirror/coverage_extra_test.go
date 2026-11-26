// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package smartmirror

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSameBaseInvalidOfficialURL(t *testing.T) {
	if SameBase("https://cdn.example.com/deepin/a.deb", "://invalid") {
		t.Fatal("SameBase accepted invalid official URL")
	}
}

func TestRewriteRejectsInvalidEntry(t *testing.T) {
	original := "https://cdn.example.com/deepin/beige/pool/a.deb"
	official := "https://cdn.example.com/deepin/"

	if _, ok := Rewrite(original, official, "ftp://mirrors.example/deepin/"); ok {
		t.Fatal("Rewrite accepted non-http entry scheme")
	}
	if _, ok := Rewrite(original, official, "http:///deepin/"); ok {
		t.Fatal("Rewrite accepted entry with empty host")
	}
}

func TestReadMessageLineTooLong(t *testing.T) {
	long := strings.Repeat("x", 1024*1024+1) + "\n"
	_, err := ReadMessage(bufio.NewReader(strings.NewReader(long)))
	if err == nil {
		t.Fatal("accepted line exceeding 1 MiB")
	}
}

func TestReadMessageInvalidCode(t *testing.T) {
	_, err := ReadMessage(bufio.NewReader(strings.NewReader("notacode text\n\n")))
	if err == nil {
		t.Fatal("accepted non-numeric code")
	}
}

func TestReadMessageSecondBlankTerminates(t *testing.T) {
	in := "600 URI Acquire\n\nURI: https://example.invalid/pool/a.deb\n\n"
	m, err := ReadMessage(bufio.NewReader(strings.NewReader(in)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Code != 600 {
		t.Fatalf("code = %d, want 600", m.Code)
	}
	if len(m.Fields) != 0 {
		t.Fatalf("fields = %d, want 0", len(m.Fields))
	}
}

func TestWriteMessageRejectsNewlineInField(t *testing.T) {
	var buf strings.Builder
	err := WriteMessage(&buf, Message{
		Code:   200,
		Text:   "URI Start",
		Fields: []Field{{"URI", "https://example.invalid\nbad"}},
	})
	if err == nil {
		t.Fatal("accepted newline in field value")
	}
}

func TestFetchUnsupportedURIScheme(t *testing.T) {
	n := Native{Config: nativeConfig(), Stderr: io.Discard}
	res := n.Fetch(context.Background(),
		Message{Code: 600, Text: "URI Acquire", Fields: []Field{{"URI", "ftp://example.invalid/file"}}},
		Budget{Total: time.Second}, nil, nil)
	if res.Err == nil {
		t.Fatal("Fetch accepted unsupported URI scheme")
	}
}

func TestFetchWithNonExistentMethodsDir(t *testing.T) {
	n := Native{
		MethodsDir: "/nonexistent-apt-methods",
		Config:     nativeConfig(),
		Stderr:     io.Discard,
	}
	res := n.Fetch(context.Background(),
		request(t, "http://example.invalid/file", nil),
		Budget{Total: 2 * time.Second}, nil, nil)
	if res.Err == nil {
		t.Fatal("expected error for non-existent methods dir")
	}
}

func TestFetchEmitCallbackInvoked(t *testing.T) {
	data := []byte(strings.Repeat("x", 1024))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		w.Write(data)
	}))
	defer srv.Close()

	var emitted []Message
	emit := func(m Message) { emitted = append(emitted, m) }
	res := (Native{Config: nativeConfig(), Stderr: io.Discard}).
		Fetch(context.Background(), request(t, srv.URL+"/file", nil),
			Budget{Total: 3 * time.Second}, emit, nil)
	if res.Err != nil {
		t.Fatalf("unexpected error: %v", res.Err)
	}
	if len(emitted) == 0 {
		t.Fatal("emit callback was not invoked")
	}
}
