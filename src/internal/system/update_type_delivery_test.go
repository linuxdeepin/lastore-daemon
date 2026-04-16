// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package system

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReplaceMatchedReposWithDelivery(t *testing.T) {
	localContent := strings.Join([]string{
		"deb https://packages.example.com/desktop beige main",
		"deb https://packages.example.com/custom beige main",
		"# keep comments untouched",
		"deb https://security.example.com beige-security main",
	}, "\n")

	platformRepos := []string{
		"deb https://packages.example.com/desktop beige main",
	}

	got := replaceMatchedReposWithDelivery(localContent, platformRepos)
	lines := strings.Split(got, "\n")

	assert.Equal(t, "deb delivery://packages.example.com/desktop beige main", lines[0])
	assert.Equal(t, "deb https://packages.example.com/custom beige main", lines[1])
	assert.Equal(t, "# keep comments untouched", lines[2])
	assert.Equal(t, "deb https://security.example.com beige-security main", lines[3])
}

func TestReplaceMatchedReposWithDeliveryRequiresExactMatch(t *testing.T) {
	localContent := "deb https://packages.example.com/desktop beige main"
	platformRepos := []string{
		"deb http://packages.example.com/desktop beige main",
	}

	got := replaceMatchedReposWithDelivery(localContent, platformRepos)

	assert.Equal(t, "deb delivery://packages.example.com/desktop beige main", got)
}

func TestReplaceMatchedReposWithDeliveryWithoutPlatformReposKeepsOriginal(t *testing.T) {
	localContent := "deb https://packages.example.com/desktop beige main"

	got := replaceMatchedReposWithDelivery(localContent, nil)

	assert.Equal(t, localContent, got)
}

func TestReplaceMatchedReposWithDeliveryTrailingSlash(t *testing.T) {
	testCases := []struct {
		name           string
		localContent   string
		platformRepos  []string
		expectedResult string
	}{
		{
			name:           "platform has trailing slash, local does not",
			localContent:   "deb https://packages.example.com/desktop beige main",
			platformRepos:  []string{"deb https://packages.example.com/desktop/ beige main"},
			expectedResult: "deb delivery://packages.example.com/desktop beige main",
		},
		{
			name:           "local has trailing slash, platform does not",
			localContent:   "deb https://packages.example.com/desktop/ beige main",
			platformRepos:  []string{"deb https://packages.example.com/desktop beige main"},
			expectedResult: "deb delivery://packages.example.com/desktop beige main",
		},
		{
			name:           "both have trailing slash",
			localContent:   "deb https://packages.example.com/desktop/ beige main",
			platformRepos:  []string{"deb https://packages.example.com/desktop/ beige main"},
			expectedResult: "deb delivery://packages.example.com/desktop beige main",
		},
		{
			name:           "neither has trailing slash",
			localContent:   "deb https://packages.example.com/desktop beige main",
			platformRepos:  []string{"deb https://packages.example.com/desktop beige main"},
			expectedResult: "deb delivery://packages.example.com/desktop beige main",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := replaceMatchedReposWithDelivery(tc.localContent, tc.platformRepos)
			assert.Equal(t, tc.expectedResult, got)
		})
	}
}

func TestExtractURLPath(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"https://packages.example.com/desktop", "packages.example.com/desktop"},
		{"https://packages.example.com/desktop/", "packages.example.com/desktop"},
		{"http://security.example.com/", "security.example.com"},
		{"delivery://packages.example.com/desktop", "packages.example.com/desktop"},
		{"no-url-here", ""},
		{"", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			got := extractURLPath(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}
