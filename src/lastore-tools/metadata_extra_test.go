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

// fakeOstreeScript mimics the `ostree` binary for the subset of subcommands
// MainMetadata exercises through utils.OSTree. Behaviour is driven through
// OSTREE_* environment variables so each test controls output and exit codes.
const fakeOstreeScript = `#!/bin/sh
sub="${2:-}"
case "$sub" in
  init)
    exit "${OSTREE_INIT_EXIT:-0}"
    ;;
  remote)
    if [ "${3:-}" = "show-url" ]; then
      printf '%s' "${OSTREE_REMOTE_URL:-}"
      exit 0
    fi
    exit "${OSTREE_REMOTE_ADD_EXIT:-0}"
    ;;
  pull)
    exit "${OSTREE_PULL_EXIT:-0}"
    ;;
  ls)
    printf '%b' "${OSTREE_LS_OUT:-a.txt\nb.txt}"
    ;;
  refs)
    printf '%b' "${OSTREE_REFS_OUT:-origin:lastore}"
    exit "${OSTREE_REFS_EXIT:-0}"
    ;;
  rev-parse)
    printf '%s' "${OSTREE_REV:-deadbeef}"
    ;;
  checkout)
    mkdir -p "${5:-}"
    exit "${OSTREE_CHECKOUT_EXIT:-0}"
    ;;
  cat)
    printf '%s' "${OSTREE_CAT_OUT:-content}"
    exit "${OSTREE_CAT_EXIT:-0}"
    ;;
  *)
    exit 0
    ;;
esac
`

// installFakeOstree writes the fake binary and prepends its directory to PATH
// so that exec.Command("ostree", ...) resolves to it.
func installFakeOstree(t *testing.T) {
	t.Helper()
	binDir := filepath.Join(t.TempDir(), "bin")
	require.NoError(t, os.MkdirAll(binDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "ostree"), []byte(fakeOstreeScript), 0755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// newMetadataContext builds a cli.Context carrying the metadata command flags
// plus any positional package ids.
func newMetadataContext(t *testing.T, update, list bool, args ...string) *cli.Context {
	t.Helper()
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	_ = set.String("remote", "", "")
	_ = set.String("local", filepath.Join(t.TempDir(), "repo"), "")
	_ = set.String("checkout", filepath.Join(t.TempDir(), "checkout"), "")
	_ = set.Bool("update", update, "")
	_ = set.Bool("list", list, "")
	if len(args) > 0 {
		require.NoError(t, set.Parse(args))
	}
	return cli.NewContext(&cli.App{}, set, nil)
}

func TestMainMetadataNewOSTreeError(t *testing.T) {
	installFakeOstree(t)
	t.Setenv("OSTREE_REMOTE_ADD_EXIT", "1")

	set := flag.NewFlagSet("test", flag.ContinueOnError)
	_ = set.String("remote", "http://example.com/repo", "")
	_ = set.String("local", filepath.Join(t.TempDir(), "repo"), "")
	_ = set.String("checkout", filepath.Join(t.TempDir(), "checkout"), "")
	_ = set.Bool("update", false, "")
	_ = set.Bool("list", false, "")
	c := cli.NewContext(&cli.App{}, set, nil)

	assert.Error(t, MainMetadata(c))
}

func TestMainMetadataPullError(t *testing.T) {
	installFakeOstree(t)
	t.Setenv("OSTREE_PULL_EXIT", "1")
	c := newMetadataContext(t, true, false)
	assert.Error(t, MainMetadata(c))
}

func TestMainMetadataCheckoutError(t *testing.T) {
	installFakeOstree(t)
	t.Setenv("OSTREE_CHECKOUT_EXIT", "1")
	c := newMetadataContext(t, true, false)
	assert.Error(t, MainMetadata(c))
}

func TestMainMetadataListAfterUpdate(t *testing.T) {
	installFakeOstree(t)
	t.Setenv("OSTREE_LS_OUT", "pkg1\npkg2")
	c := newMetadataContext(t, true, true)
	assert.NoError(t, MainMetadata(c))
}

func TestMainMetadataListSkipUpdate(t *testing.T) {
	installFakeOstree(t)
	c := newMetadataContext(t, false, true)
	assert.NoError(t, MainMetadata(c))
}

func TestMainMetadataHasBranchFalseTriggersUpdate(t *testing.T) {
	installFakeOstree(t)
	t.Setenv("OSTREE_REFS_OUT", "other-branch")
	c := newMetadataContext(t, false, false)
	assert.NoError(t, MainMetadata(c))
}

func TestMainMetadataCat(t *testing.T) {
	installFakeOstree(t)
	t.Setenv("OSTREE_CAT_OUT", `{"id":"pkg1"}`)
	c := newMetadataContext(t, false, false, "pkg1", "pkg2")
	assert.NoError(t, MainMetadata(c))
}

func TestMainMetadataCatError(t *testing.T) {
	installFakeOstree(t)
	t.Setenv("OSTREE_CAT_EXIT", "1")
	c := newMetadataContext(t, false, false, "pkg1")
	assert.NoError(t, MainMetadata(c))
}
