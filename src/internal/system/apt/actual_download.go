// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package apt

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
)

// Worker debug lines are percent-encoded by APT. Decode exactly once, using
// PathUnescape so a literal '+' in a package name is not turned into a space.
func parseAPTWorkerInfo(id, line string) (system.JobDeliveryDownloadInfo, bool) {
	info := system.JobDeliveryDownloadInfo{JobId: id}
	line = strings.TrimSpace(line)
	outgoing := strings.Contains(line, "-> ")
	incoming := strings.Contains(line, "<- ")
	if !outgoing && !incoming {
		return info, false
	}
	colon := strings.IndexByte(line, ':')
	if colon < 0 {
		return info, true
	}
	raw := line[colon+1:]
	if outgoing && incoming {
		// The direction marker can only be one of these in APT's debug format;
		// keep the first marker when a proxy prefixes the line unexpectedly.
		outgoing = strings.Index(line, "-> ") < strings.Index(line, "<- ")
	}
	if raw == "" {
		return info, true
	}
	message, err := url.PathUnescape(raw)
	if err != nil {
		return info, true
	}
	lines := strings.Split(message, "\n")
	header := strings.Fields(lines[0])
	if len(header) < 1 {
		return info, true
	}
	code, _ := strconv.Atoi(header[0])
	if (outgoing && code != 600) || (!outgoing && code != 102 && code != 200 && code != 201 && code != 400 && code != 103) {
		return info, true
	}
	fields := make(map[string]string)
	for _, line := range lines[1:] {
		if key, value, ok := strings.Cut(line, ":"); ok {
			fields[key] = strings.TrimSpace(value)
		}
	}
	info.MethodEvent = code
	info.NewURI, info.UsedMirror = fields["New-URI"], fields["UsedMirror"]
	info.FileName, info.FailReason = fields["Filename"], fields["FailReason"]
	info.DownloadSize, _ = strconv.ParseInt(fields["Size"], 10, 64)
	info.ResumePoint, _ = strconv.ParseInt(fields["Resume-Point"], 10, 64)
	// The method-side monotonic timestamps X-Deepin-Transfer-Start-USec and
	// X-Deepin-Transfer-End-USec are the only trusted transfer time.
	info.TransferStartUs, _ = strconv.ParseInt(fields["X-Deepin-Transfer-Start-USec"], 10, 64)
	info.TransferEndUs, _ = strconv.ParseInt(fields["X-Deepin-Transfer-End-USec"], 10, 64)
	info.DownloadedSize, _ = strconv.ParseInt(fields["DownloadedSize"], 10, 64)
	info.IMS = fields["IMS-Hit"] == "true"
	info.URI = fields["URI"]
	if info.URI != "" {
		parsed, parseErr := url.Parse(info.URI)
		if parseErr != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			info.URI = ""
		}
	}
	if info.URI == "" && info.NewURI != "" {
		info.URI = info.NewURI
	}
	if info.URI != "" {
		if parsed, parseErr := url.Parse(info.URI); parseErr == nil {
			info.Proto = parsed.Scheme
		}
	}
	info.IsFinished = code == 201 || code == 400
	info.Failed = code == 400
	if info.IsFinished {
		info.Speed = -1
	}
	return info, true
}
