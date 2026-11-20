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
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	nativesmartmirror "github.com/linuxdeepin/lastore-daemon/src/internal/smartmirror"
	"github.com/linuxdeepin/lastore-daemon/src/internal/utils"
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

	// sniffTotal bounds one availability check. The check runs inside the Query
	// handler and gates the download, so it gets a shorter budget than the
	// 4 KiB probe even though native APT startup is part of it.
	sniffTotal = 2 * time.Second
)

var errProbeLocal = errors.New("local probe unavailable")

// sniffResult classifies the outcome of the per-request availability check.
type sniffResult int

const (
	// sniffUnavailable means the check itself failed (network error, timeout,
	// unusable response). The mirror is not proven bad, so it is not blacklisted.
	sniffUnavailable sniffResult = iota
	// sniffNotFound means the server answered 404/410: the mirror does not serve
	// this object.
	sniffNotFound
	// sniffOK means the object is downloadable.
	sniffOK
)

// sniffResourceFn is an injectable seam so tests can replace the availability
// check without running the native APT methods.
var sniffResourceFn = (*aptProbe).sniff

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
	budget := nativesmartmirror.Budget{Bytes: bytes, Idle: probeIdle, Total: probe4KTotal}
	if bytes != probe4KBytes {
		budget.Total = probe2MTotal
	}
	logger.Infof("smartmirror probe start uri=%q limit=%d timeout=%s", uri, bytes, budget.Total)
	res, err := p.fetch(uri, budget)
	logger.Infof("smartmirror probe end uri=%q endpoint=%q limit=%d bytes=%d body_ms=%d wait_ms=%d speed=%.0f error=%v",
		uri, res.Sample.Endpoint, bytes, res.Sample.Bytes, res.Sample.Body.Milliseconds(),
		res.Sample.Wait.Milliseconds(), clampSpeed(res.Sample.Bytes, res.Sample.Body), err)
	return res.Sample, err
}

// sniff reports whether uri serves data right now. The check goes through the
// native APT method so it sees the same proxy, TLS, authentication and rate
// limit configuration as the real download; a plain HTTP client would disagree
// with apt on exactly the deployments that need the check.
func (p *aptProbe) sniff(uri string) sniffResult {
	if p == nil {
		return sniffUnavailable
	}
	res, err := p.fetch(uri, nativesmartmirror.Budget{Bytes: probe4KBytes, Total: sniffTotal})
	if err == nil {
		return sniffOK
	}
	status := sniffStatus(res.Done)
	switch {
	case errors.Is(err, errProbeLocal):
		// The environment has no usable native methods; warning for every URL
		// would drown the log.
		logger.Debugf("smartmirror sniff skipped uri=%q: %v", uri, err)
		return sniffUnavailable
	case status == http.StatusNotFound || status == http.StatusGone:
		logger.Warningf("smartmirror sniff uri=%q status=%d error=%v", uri, status, err)
		return sniffNotFound
	default:
		// 401/403/408/429/5xx and transport errors do not prove the object is
		// missing, so the mirror is tried again instead of being blocked.
		logger.Warningf("smartmirror sniff uri=%q status=%d transient error=%v", uri, status, err)
		return sniffUnavailable
	}
}

// sniffStatus reads the HTTP status an APT method reports for a failed
// transfer. Newer methods expose it as FailReason=HttpError404; older ones only
// prefix the message with the numeric status. It returns 0 when the method did
// not report a status at all.
func sniffStatus(done nativesmartmirror.Message) int {
	if reason := done.Get("FailReason"); strings.HasPrefix(reason, "HttpError") {
		if code, err := strconv.Atoi(strings.TrimPrefix(reason, "HttpError")); err == nil && code >= 100 && code <= 599 {
			return code
		}
	}
	fields := strings.Fields(done.Get("Message"))
	if len(fields) == 0 {
		return 0
	}
	code, err := strconv.Atoi(fields[0])
	if err != nil || code < 100 || code > 599 {
		return 0
	}
	return code
}

// fetch performs one native APT transfer and returns the untouched result so a
// caller can classify a failure from the done message.
func (p *aptProbe) fetch(uri string, budget nativesmartmirror.Budget) (nativesmartmirror.Transfer, error) {
	if p == nil || !p.available(uri) {
		return nativesmartmirror.Transfer{Sample: nativesmartmirror.Sample{At: time.Now()}},
			fmt.Errorf("%w: native apt methods unavailable", errProbeLocal)
	}
	if err := os.MkdirAll(p.root, 0700); err != nil {
		return nativesmartmirror.Transfer{Sample: nativesmartmirror.Sample{At: time.Now()}}, err
	}
	f, err := os.CreateTemp(p.root, ".lastore-smartmirror-probe-")
	if err != nil {
		return nativesmartmirror.Transfer{Sample: nativesmartmirror.Sample{At: time.Now()}}, err
	}
	filename := f.Name()
	_ = f.Close()
	defer func() { _ = os.Remove(filename) }()

	req := nativesmartmirror.Message{Code: 600, Text: "URI Acquire", Fields: []nativesmartmirror.Field{
		{Key: "URI", Value: uri}, {Key: "Filename", Value: filename},
	}}
	res := p.native.Fetch(context.Background(), req, budget, nil, nil)
	if res.Sample.At.IsZero() {
		res.Sample.At = time.Now()
	}
	return res, res.Err
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
	// The state directory is normally created by systemd; keep the 0700
	// fallback here because the util creates directories with 0755.
	if err := os.MkdirAll(filepath.Dir(fpath), 0700); err != nil {
		return err
	}
	return utils.WriteFileAtomic(fpath, data, 0644)
}
