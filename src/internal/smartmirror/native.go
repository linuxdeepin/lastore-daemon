// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package smartmirror

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

type Budget struct {
	Total, Body, Idle time.Duration
	Bytes             int64
}

type Native struct {
	MethodsDir string
	Config     Message
	Stderr     io.Writer
}

type Transfer struct {
	Sample Sample
	Done   Message
	Err    error
}

// Fetch keeps native APT's authentication, proxy, TLS, Range/resume and hashing.
// A probe uses RLIMIT_FSIZE in a child, so ignored Range headers cannot fill disk.
// The time budget includes redirection and startup, not just body transfer.
func (n Native) Fetch(ctx context.Context, req Message, budget Budget, emit func(Message), slow func(int64, time.Duration) bool) Transfer {
	start := time.Now()
	if budget.Total > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, budget.Total)
		defer cancel()
	}
	target := req.Get("URI")
	sample := Sample{}
	var totalBytes int64
	for redirects := 0; redirects <= 10; redirects++ {
		attemptBudget := budget
		if budget.Bytes > 0 {
			attemptBudget.Bytes = budget.Bytes - totalBytes
			if attemptBudget.Bytes <= 0 {
				sample.Capped = true
				sample.Bytes = totalBytes
				return Transfer{Sample: sample}
			}
		}
		res := n.once(ctx, req.With("URI", target), attemptBudget, emit, slow)
		totalBytes += res.Sample.Bytes
		sample = res.Sample
		sample.Bytes = totalBytes
		sample.Endpoint = target
		if budget.Bytes > 0 && totalBytes >= budget.Bytes {
			sample.Capped = true
			res.Err = nil
			return Transfer{Sample: sample, Done: res.Done}
		}
		if res.Done.Code != 103 {
			res.Sample = sample
			if sample.Body == 0 {
				res.Sample.Wait = time.Since(start)
			}
			return res
		}
		next, err := url.Parse(res.Done.Get("New-URI"))
		if err != nil || (next.Scheme != "http" && next.Scheme != "https") || next.Host == "" {
			return Transfer{Sample: sample, Err: fmt.Errorf("invalid redirect")}
		}
		prev, _ := url.Parse(target)
		if prev != nil && prev.Scheme == "https" && next.Scheme == "http" &&
			Configuration(n.Config)["Acquire::https::AllowRedirect"] == "false" {
			return Transfer{Sample: sample, Err: fmt.Errorf("redirect disabled")}
		}
		target = next.String()
	}
	return Transfer{Sample: sample, Err: fmt.Errorf("too many redirects")}
}

func (n Native) once(ctx context.Context, req Message, budget Budget, emit func(Message), slow func(int64, time.Duration) bool) (out Transfer) {
	u, err := url.Parse(req.Get("URI"))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		out.Err = fmt.Errorf("unsupported native URI")
		return
	}
	dir := n.MethodsDir
	if dir == "" {
		dir = "/usr/lib/apt/methods"
	}
	method := filepath.Join(dir, u.Scheme)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, method)
	if budget.Bytes > 0 {
		// The byte cap is enforced by the kernel. util-linux prlimit execs the
		// method without a shell, so an ignored Range header cannot fill the disk.
		limit := strconv.FormatInt(budget.Bytes, 10)
		prlimit, err := exec.LookPath("prlimit")
		if err != nil {
			out.Err = fmt.Errorf("prlimit not found: %w", err)
			return
		}
		cmd = exec.CommandContext(ctx, prlimit, "--fsize="+limit+":"+limit, "--", method)
	}
	cmd.Stderr = n.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		out.Err = err
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		out.Err = err
		return
	}
	if err = cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		out.Err = err
		return
	}
	defer func() {
		cancel()
		stdin.Close()
		cmd.Wait()
	}()

	type event struct {
		message Message
		err     error
	}
	events := make(chan event)
	go func() {
		defer close(events)
		r := bufio.NewReader(stdout)
		for {
			m, err := ReadMessage(r)
			select {
			case events <- event{m, err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	begin := time.Now()
	bodyStart := time.Time{}
	lastProgress := begin
	initial := fileSize(req.Get("Filename"))
	lastSize := initial
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	update := func() {
		size := fileSize(req.Get("Filename"))
		if size < initial {
			initial = 0
		} // native method restarted instead of resuming
		if size > lastSize {
			lastProgress = time.Now()
			if bodyStart.IsZero() {
				bodyStart = lastProgress
			}
		}
		lastSize = size
		out.Sample.Bytes = size - initial
		if !bodyStart.IsZero() {
			out.Sample.Wait = bodyStart.Sub(begin)
			out.Sample.Body = time.Since(bodyStart)
		}
		out.Sample.At = time.Now()
		out.Sample.Capped = budget.Bytes > 0 && size >= budget.Bytes
	}
	defer update()
	for {
		select {
		case <-ctx.Done():
			update()
			out.Err = ctx.Err()
			// A probe is successful only after the byte cap was reached. A
			// timeout after partial progress remains an error; callers may use
			// the measured partial speed as negative evidence, but must not let
			// it open the probe gate.
			if out.Sample.Capped {
				out.Err = nil
			}
			return
		case <-tick.C:
			update()
			if out.Sample.Capped || (budget.Body > 0 && !bodyStart.IsZero() && time.Since(bodyStart) >= budget.Body) {
				if !out.Sample.Capped && budget.Bytes > 0 {
					out.Err = fmt.Errorf("probe body window ended before byte cap")
				} else if out.Sample.Bytes == 0 {
					out.Err = fmt.Errorf("probe has no body")
				}
				return
			}
			if budget.Idle > 0 && time.Since(lastProgress) >= budget.Idle {
				out.Err = fmt.Errorf("download made no progress for %s", budget.Idle)
				return
			}
			if slow != nil && !bodyStart.IsZero() && slow(out.Sample.Bytes, time.Since(bodyStart)) {
				out.Err = fmt.Errorf("download slower than available alternative")
				return
			}
		case ev, ok := <-events:
			update()
			if !ok || ev.err != nil {
				if out.Sample.Capped {
					return
				}
				out.Err = fmt.Errorf("native method stopped before completion")
				return
			}
			m := ev.message
			switch m.Code {
			case 100:
				if err := WriteMessage(stdin, n.Config); err != nil {
					out.Err = err
					return
				}
				if err := WriteMessage(stdin, req); err != nil {
					out.Err = err
					return
				}
			case 200:
				if bodyStart.IsZero() {
					bodyStart = time.Now()
				}
				if emit != nil {
					emit(m)
				}
			case 201:
				out.Done = m
				return
			case 103:
				out.Done = m
				return
			case 400, 401:
				if out.Sample.Capped {
					return
				}
				out.Done = m
				out.Err = fmt.Errorf("%s", m.Get("Message"))
				return
			case 101, 102:
				// Do not leak rewritten URIs or credentials into normal progress.
			default:
				out.Err = fmt.Errorf("unsupported native message %d", m.Code)
				return
			}
		}
	}
}

func fileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}
