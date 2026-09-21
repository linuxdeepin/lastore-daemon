// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

// Package smartmirror implements speed selection around the native APT methods.
package smartmirror

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Field struct{ Key, Value string }

// Message preserves repeated Config-Item fields and native method hash fields.
type Message struct {
	Code   int
	Text   string
	Fields []Field
}

func (m Message) Get(key string) string {
	for _, f := range m.Fields {
		if strings.EqualFold(f.Key, key) {
			return f.Value
		}
	}
	return ""
}

func (m Message) With(key, value string) Message {
	n := Message{Code: m.Code, Text: m.Text}
	for _, f := range m.Fields {
		if !strings.EqualFold(f.Key, key) {
			n.Fields = append(n.Fields, f)
		}
	}
	n.Fields = append(n.Fields, Field{key, value})
	return n
}

func (m Message) Without(keys ...string) Message {
	n := Message{Code: m.Code, Text: m.Text}
	for _, f := range m.Fields {
		keep := true
		for _, key := range keys {
			if strings.EqualFold(f.Key, key) {
				keep = false
			}
		}
		if keep {
			n.Fields = append(n.Fields, f)
		}
	}
	return n
}

func ReadMessage(r *bufio.Reader) (Message, error) {
	m := Message{}
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return m, err
		}
		if len(line) > 1024*1024 {
			return m, fmt.Errorf("APT protocol line too long")
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if m.Code != 0 {
				return m, nil
			}
			continue
		}
		if m.Code == 0 {
			code, text, ok := strings.Cut(line, " ")
			m.Code, err = strconv.Atoi(code)
			if err != nil || !ok {
				return m, fmt.Errorf("invalid APT message")
			}
			m.Text = text
		} else {
			key, value, ok := strings.Cut(line, ":")
			if !ok {
				return m, fmt.Errorf("invalid APT field")
			}
			m.Fields = append(m.Fields, Field{key, strings.TrimSpace(value)})
		}
	}
}

func WriteMessage(w io.Writer, m Message) error {
	if strings.ContainsAny(m.Text, "\r\n") {
		return fmt.Errorf("newline in APT message")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d %s\n", m.Code, m.Text)
	for _, f := range m.Fields {
		// The wire format has no escaping for embedded newlines.
		if strings.ContainsAny(f.Key+f.Value, "\r\n") {
			return fmt.Errorf("newline in APT field")
		}
		fmt.Fprintf(&b, "%s: %s\n", f.Key, f.Value)
	}
	b.WriteByte('\n')
	_, err := io.WriteString(w, b.String())
	return err
}

func Configuration(m Message) map[string]string {
	c := map[string]string{}
	for _, f := range m.Fields {
		if f.Key != "Config-Item" {
			continue
		}
		k, v, ok := strings.Cut(f.Value, "=")
		if ok {
			c[k] = v
		}
	}
	return c
}
