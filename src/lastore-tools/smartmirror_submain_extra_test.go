// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/codegangsta/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmainMirrorSynProgress(t *testing.T) {
	parentSet := flag.NewFlagSet("parent", flag.ContinueOnError)
	_ = parentSet.String("official", "", "")
	_ = parentSet.String("mirrorlist", "", "")
	_ = parentSet.Int("parallel", 0, "")

	set := flag.NewFlagSet("child", flag.ContinueOnError)
	_ = set.String("index", "", "")
	_ = set.String("export", "", "")
	_ = set.Bool("list", true, "")

	parent := cli.NewContext(&cli.App{}, parentSet, nil)
	c := cli.NewContext(&cli.App{}, set, parent)

	// onlyList=true short-circuits before any network/mirror probing.
	assert.NoError(t, SubmainMirrorSynProgress(c))
}

// newMirrorSynProgressCtx builds a non-list context whose parent exposes the
// official/mirrorlist/parallel flags, with a controlled export path.
func newMirrorSynProgressCtx(t *testing.T, export string) *cli.Context {
	t.Helper()
	parentSet := flag.NewFlagSet("parent", flag.ContinueOnError)
	_ = parentSet.String("official", "", "")
	_ = parentSet.String("mirrorlist", "", "")
	_ = parentSet.Int("parallel", 0, "")

	set := flag.NewFlagSet("child", flag.ContinueOnError)
	_ = set.String("index", "", "")
	_ = set.String("export", export, "")
	_ = set.Bool("list", false, "")

	parent := cli.NewContext(&cli.App{}, parentSet, nil)
	return cli.NewContext(&cli.App{}, set, parent)
}

func TestSubmainMirrorSynProgressCreateError(t *testing.T) {
	// Empty export path makes os.Create fail after the (empty) mirror probing.
	c := newMirrorSynProgressCtx(t, "")
	assert.Error(t, SubmainMirrorSynProgress(c))
}

func TestSubmainMirrorSynProgressSaveInfos(t *testing.T) {
	// No mirror list and no official index: DetectServer returns nil, then the
	// empty result set is serialized to the export file successfully.
	export := filepath.Join(t.TempDir(), "progress.json")
	c := newMirrorSynProgressCtx(t, export)
	assert.NoError(t, SubmainMirrorSynProgress(c))

	data, err := os.ReadFile(export)
	require.NoError(t, err)
	assert.Equal(t, "null\n", string(data))
}
