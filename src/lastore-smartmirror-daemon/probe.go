// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	nativesmartmirror "github.com/linuxdeepin/lastore-daemon/src/internal/smartmirror"
)

const (
	probe4KBytes        int64 = 4 << 10
	probe2MBytes        int64 = 2 << 20
	probe4KTotal              = 3 * time.Second
	probe2MTotal              = 8 * time.Second
	probeIdle                 = 5 * time.Second
	probeBatchSize            = 5
	probeSpeedBatch           = 3
	probeRoundsPerBatch       = 5
)

var errProbeLocal = errors.New("local probe unavailable")

// aptProbe runs the system APT methods directly, so a probe uses the same
// proxy, TLS, authentication and rate limit configuration as a real download.
type aptProbe struct {
	native nativesmartmirror.Native
	root   string
}

func newAPTProbe(root string) *aptProbe {
	if root == "" {
		root = os.TempDir()
	}
	return &aptProbe{
		native: nativesmartmirror.Native{
			Config: loadAPTConfiguration(),
			Stderr: io.Discard,
		},
		root: root,
	}
}

func loadAPTConfiguration() nativesmartmirror.Message {
	message := nativesmartmirror.Message{Code: 601, Text: "Configuration"}
	cmd := exec.Command("apt-config", "dump") // #nosec G204 - fixed executable and arguments
	out, err := cmd.Output()
	if err != nil {
		return message
	}
	for scanner := bufio.NewScanner(strings.NewReader(string(out))); scanner.Scan(); {
		line := strings.TrimSpace(scanner.Text())
		line = strings.TrimSuffix(line, ";")
		key, value, ok := strings.Cut(line, " ")
		if !ok || key == "" {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			if unquoted, err := strconv.Unquote(value); err == nil {
				value = unquoted
			}
		}
		message.Fields = append(message.Fields, nativesmartmirror.Field{
			Key: "Config-Item", Value: key + "=" + value,
		})
	}
	return message
}

func (p *aptProbe) available(uri string) bool {
	u, err := url.Parse(uri)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	if _, err := os.Stat(filepath.Join("/usr/lib/apt/methods", u.Scheme)); err != nil {
		return false
	}
	_, err = exec.LookPath("prlimit")
	return err == nil
}

func (p *aptProbe) run(uri string, bytes int64) (nativesmartmirror.Sample, error) {
	if !p.available(uri) {
		return nativesmartmirror.Sample{At: time.Now()}, fmt.Errorf("%w: native apt methods unavailable", errProbeLocal)
	}
	if err := os.MkdirAll(p.root, 0700); err != nil {
		return nativesmartmirror.Sample{At: time.Now()}, err
	}
	f, err := os.CreateTemp(p.root, ".lastore-smartmirror-probe-")
	if err != nil {
		return nativesmartmirror.Sample{At: time.Now()}, err
	}
	filename := f.Name()
	_ = f.Close()
	defer func() { _ = os.Remove(filename) }()

	budget := nativesmartmirror.Budget{Bytes: bytes, Idle: probeIdle}
	if bytes == probe4KBytes {
		budget.Total = probe4KTotal
	} else {
		budget.Total = probe2MTotal
	}
	req := nativesmartmirror.Message{Code: 600, Text: "URI Acquire", Fields: []nativesmartmirror.Field{
		{Key: "URI", Value: uri}, {Key: "Filename", Value: filename},
	}}
	logger.Infof("smartmirror probe start uri=%q limit=%d timeout=%s", uri, bytes, budget.Total)
	res := p.native.Fetch(context.Background(), req, budget, nil, nil)
	if res.Sample.At.IsZero() {
		res.Sample.At = time.Now()
	}
	logger.Infof("smartmirror probe end uri=%q endpoint=%q limit=%d bytes=%d body_ms=%d wait_ms=%d speed=%.0f error=%v",
		uri, res.Sample.Endpoint, bytes, res.Sample.Bytes, res.Sample.Body.Milliseconds(),
		res.Sample.Wait.Milliseconds(), clampSpeed(res.Sample.Bytes, res.Sample.Body), res.Err)
	return res.Sample, res.Err
}

func probeURI(original, official, mirror string) (string, bool) {
	return nativesmartmirror.Rewrite(original, official, mirror)
}

func persistQualityData(mq *MirrorQuality, fpath string) error {
	// Serialize snapshot + rename, so an older snapshot cannot overwrite a
	// newer report.
	mq.persistMu.Lock()
	defer mq.persistMu.Unlock()
	mq.prune(time.Now())
	mq.mux.Lock()
	data, err := json.Marshal(mq.QualityMap)
	mq.mux.Unlock()
	if err != nil {
		return err
	}
	return atomicQualityWrite(fpath, data)
}

func atomicQualityWrite(fpath string, data []byte) error {
	dir := filepath.Dir(fpath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".smartmirror-quality-")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Chmod(0644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, fpath)
}
