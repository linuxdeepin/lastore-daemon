// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// expectedCacheTTL bounds how long one apt-cache result is reused. The
	// index a running apt uses is stable, but an apt update replaces it, so a
	// short window keeps the check from comparing against a replaced index for
	// long while still avoiding one apt-cache call per pool object.
	expectedCacheTTL = time.Hour

	expectedCacheMaxItems = 512
	aptCacheShowTimeout   = 10 * time.Second

	// aptListsDir is where apt keeps its index files. Its modification time
	// changes when apt update replaces them, which invalidates cached rows
	// even inside expectedCacheTTL.
	aptListsDir = "/var/lib/apt/lists"
)

var aptListsModTime = func() time.Time {
	info, err := os.Stat(aptListsDir)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// packageNamePattern is the subset of the Debian package name syntax that can
// safely be passed to apt-cache. Package names are lower case; the argument is
// still validated because Query is reachable from the system bus.
var packageNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]*$`)

var errInvalidPoolURL = errors.New("not a package pool URL")

// expectedSizeFn is an injectable seam so tests can exercise the size check
// without apt-cache.
var expectedSizeFn = func(r *expectedResolver, rawURL string) (int64, bool) {
	return r.resolveSize(rawURL)
}

// expectedResolver maps a pool URL to the size the archive publishes for that
// file. The value is what apt itself compares the download against (the
// Checksum-FileSize entry), so a mismatch predicts that apt would refuse the
// object; it does not prove anything about bytes of the same size.
type expectedResolver struct {
	mu       sync.Mutex
	cache    map[string]expectedCacheEntry
	now      func() time.Time
	show     func(pkg string) ([]byte, error)
	max      int
	listsAt  time.Time
	listsMod func() time.Time
}

type expectedCacheEntry struct {
	size int64
	at   time.Time
}

func newExpectedResolver() *expectedResolver {
	return &expectedResolver{
		cache:    make(map[string]expectedCacheEntry),
		now:      time.Now,
		show:     aptCacheShow,
		max:      expectedCacheMaxItems,
		listsMod: aptListsModTime,
	}
}

// aptCacheShow reads the package records apt has on disk. -- stops option
// parsing so a malformed name cannot turn into an apt-cache option.
func aptCacheShow(pkg string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), aptCacheShowTimeout)
	defer cancel()
	return exec.CommandContext(ctx, "apt-cache", "show", "--", pkg).Output() // #nosec G204 - fixed executable, validated package name
}

// resolveSize returns the published size for rawURL, or false when the
// metadata is unavailable, ambiguous or unusable. Callers treat false as "do
// not know" and must not reject a mirror because of it.
func (r *expectedResolver) resolveSize(rawURL string) (int64, bool) {
	parsed, pkg, err := parsePoolURL(rawURL)
	if err != nil {
		return 0, false
	}
	key := parsed.Path
	now := r.now()
	listsAt := time.Time{}
	if r.listsMod != nil {
		listsAt = r.listsMod()
	}

	r.mu.Lock()
	if r.cache == nil {
		r.cache = make(map[string]expectedCacheEntry)
	}
	if !listsAt.IsZero() && !listsAt.Equal(r.listsAt) {
		// apt update replaced the index; rows cached against the old one would
		// compare a mirror object with a stale expectation.
		r.cache = make(map[string]expectedCacheEntry)
		r.listsAt = listsAt
	}
	if entry, ok := r.cache[key]; ok && now.Sub(entry.at) < expectedCacheTTL {
		r.mu.Unlock()
		return entry.size, entry.size > 0
	}
	r.mu.Unlock()

	out, err := r.show(pkg)
	if err != nil {
		return 0, false
	}
	size, ok := parseExpectedSize(out, parsed)
	if !ok {
		return 0, false
	}

	limit := r.max
	if limit <= 0 {
		limit = expectedCacheMaxItems
	}
	r.mu.Lock()
	if r.cache == nil {
		r.cache = make(map[string]expectedCacheEntry)
	}
	if len(r.cache) >= limit {
		r.evictOldestLocked()
	}
	r.cache[key] = expectedCacheEntry{size: size, at: now}
	if !listsAt.IsZero() {
		r.listsAt = listsAt
	}
	r.mu.Unlock()
	return size, true
}

func (r *expectedResolver) evictOldestLocked() {
	var oldestKey string
	var oldest time.Time
	for key, entry := range r.cache {
		if oldestKey == "" || entry.at.Before(oldest) {
			oldestKey, oldest = key, entry.at
		}
	}
	if oldestKey != "" {
		delete(r.cache, oldestKey)
	}
}

// parsePoolURL derives the package name from the pool filename. The name is
// the part before the first underscore, which is how APT names a binary
// package archive: <package>_<version>_<arch>.deb.
func parsePoolURL(rawURL string) (*url.URL, string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.User != nil ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, "", errInvalidPoolURL
	}
	base := path.Base(parsed.Path)
	if base == "." || base == "/" || path.Ext(base) != ".deb" {
		return nil, "", errInvalidPoolURL
	}
	underscore := strings.IndexByte(base, '_')
	if underscore <= 0 {
		return nil, "", errInvalidPoolURL
	}
	name := base[:underscore]
	if !packageNamePattern.MatchString(name) {
		return nil, "", errInvalidPoolURL
	}
	return parsed, name, nil
}

// parseExpectedSize finds the size of the pool file in apt-cache show output.
// Records without a Filename (the installed-version records) cannot describe a
// download and are skipped. When several records match with different sizes
// the archive is inconsistent, so the result is inconclusive rather than an
// arbitrary pick.
func parseExpectedSize(out []byte, parsed *url.URL) (int64, bool) {
	var found int64
	normalized := strings.ReplaceAll(string(out), "\r\n", "\n")
	for _, stanza := range strings.Split(normalized, "\n\n") {
		fields := parsePackageStanza(stanza)
		filename := fields["filename"]
		if filename == "" || !poolFilenameMatches(parsed, filename) {
			continue
		}
		size, err := strconv.ParseInt(fields["size"], 10, 64)
		if err != nil || size <= 0 {
			continue
		}
		if found == 0 {
			found = size
			continue
		}
		if found != size {
			return 0, false
		}
	}
	return found, found > 0
}

// parsePackageStanza reads the top-level fields of one Packages record.
// Continuation lines start with a space or tab and are not needed here.
func parsePackageStanza(stanza string) map[string]string {
	fields := make(map[string]string)
	for _, line := range strings.Split(stanza, "\n") {
		if line == "" || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		fields[key] = strings.TrimSpace(value)
	}
	return fields
}

// poolFilenameMatches reports whether filename names the file in the URL. The
// record stores the path relative to the archive root while the URL may carry
// an extra base prefix such as /deepin/ or /beige/, so the comparison is a
// path suffix. Both the decoded and the escaped path are accepted because
// epochs appear as %3a in pool filenames.
func poolFilenameMatches(parsed *url.URL, filename string) bool {
	filename = strings.TrimPrefix(strings.TrimSpace(filename), "/")
	if filename == "" || strings.Contains(filename, "://") {
		return false
	}
	candidates := []string{filename}
	if unescaped, err := url.PathUnescape(filename); err == nil && unescaped != filename {
		candidates = append(candidates, unescaped)
	}
	paths := []string{parsed.Path}
	if escaped := parsed.EscapedPath(); escaped != parsed.Path {
		paths = append(paths, escaped)
	}
	for _, candidate := range candidates {
		for _, p := range paths {
			if p == "/"+candidate || p == candidate || strings.HasSuffix(p, "/"+candidate) {
				return true
			}
		}
	}
	return false
}
