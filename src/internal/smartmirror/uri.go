// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package smartmirror

import (
	"net/url"
	"strings"
)

func NormalizeBase(raw string) string { return strings.TrimRight(raw, "/") + "/" }

// SameBase reports whether original lies inside the official base: same scheme,
// same host and the official path as a prefix. Rewrite uses exactly this check,
// so a URI accepted here can always be rewritten for a usable mirror entry.
func SameBase(original, official string) bool {
	u, err := url.Parse(original)
	if err != nil {
		return false
	}
	base, err := url.Parse(NormalizeBase(official))
	if err != nil {
		return false
	}
	return u.User == nil && u.Scheme == base.Scheme && u.Host == base.Host &&
		strings.HasPrefix(u.EscapedPath(), base.EscapedPath())
}

func Rewrite(original, official, entry string) (string, bool) {
	u, err := url.Parse(original)
	base, e := url.Parse(NormalizeBase(official))
	dest, d := url.Parse(NormalizeBase(entry))
	if err != nil || e != nil || d != nil ||
		(dest.Scheme != "http" && dest.Scheme != "https") || dest.Host == "" ||
		!SameBase(original, official) {
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
