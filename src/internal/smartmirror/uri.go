// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package smartmirror

import (
	"net/url"
	"strings"
)

func NormalizeBase(raw string) string { return strings.TrimRight(raw, "/") + "/" }

func Rewrite(original, official, entry string) (string, bool) {
	u, err := url.Parse(original)
	base, e := url.Parse(NormalizeBase(official))
	dest, d := url.Parse(NormalizeBase(entry))
	if err != nil || e != nil || d != nil || u.User != nil ||
		(dest.Scheme != "http" && dest.Scheme != "https") || dest.Host == "" ||
		u.Scheme != base.Scheme || u.Host != base.Host ||
		!strings.HasPrefix(u.EscapedPath(), base.EscapedPath()) {
		return original, false
	}
	suffix := strings.TrimPrefix(u.EscapedPath(), base.EscapedPath())
	v, err := url.Parse(NormalizeBase(entry) + suffix)
	if err != nil {
		return original, false
	}
	v.RawQuery = u.RawQuery
	return v.String(), true
}
