// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeOstreeScript returns the content of a POSIX sh script that mimics the
// `ostree` binary for the subset of subcommands the OSTree wrapper exercises.
// Behaviour is driven through OSTREE_* environment variables so each test can
// control the output and exit codes independently.
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
    printf '%b' "${OSTREE_REFS_OUT:-branch1}"
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
	fake := filepath.Join(binDir, "ostree")
	require.NoError(t, os.WriteFile(fake, []byte(fakeOstreeScript), 0755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func newTestOSTree(t *testing.T) *OSTree {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	return &OSTree{repo: repo}
}

func TestNewOSTreeSuccessExtra(t *testing.T) {
	installFakeOstree(t)
	repo := filepath.Join(t.TempDir(), "repo")
	tree, err := NewOSTree(repo, "http://example.com/repo")
	require.NoError(t, err)
	require.NotNil(t, tree)
	assert.Equal(t, repo, tree.repo)
}

func TestNewOSTreeSameRemoteSkipsInitExtra(t *testing.T) {
	installFakeOstree(t)
	t.Setenv("OSTREE_REMOTE_URL", "http://example.com/repo")

	// repo dir intentionally absent; early return must not create it.
	repo := filepath.Join(t.TempDir(), "nonexistent", "repo")
	tree, err := NewOSTree(repo, "http://example.com/repo")
	require.NoError(t, err)
	require.NotNil(t, tree)
	_, statErr := os.Stat(repo)
	assert.Error(t, statErr)
}

func TestNewOSTreeRemoteAddFailedExtra(t *testing.T) {
	installFakeOstree(t)
	t.Setenv("OSTREE_REMOTE_ADD_EXIT", "1")
	repo := filepath.Join(t.TempDir(), "repo")
	tree, err := NewOSTree(repo, "http://example.com/repo")
	assert.Error(t, err)
	assert.Nil(t, tree)
}

func TestOSTreePullSuccessExtra(t *testing.T) {
	installFakeOstree(t)
	tree := newTestOSTree(t)
	require.NoError(t, tree.Pull("branch1"))
}

func TestOSTreePullFailedExtra(t *testing.T) {
	installFakeOstree(t)
	t.Setenv("OSTREE_PULL_EXIT", "1")
	tree := newTestOSTree(t)
	assert.Error(t, tree.Pull("branch1"))
}

func TestOSTreeListExtra(t *testing.T) {
	installFakeOstree(t)
	tree := newTestOSTree(t)
	out, err := tree.List("branch1", "/")
	require.NoError(t, err)
	assert.Equal(t, "a.txt\nb.txt", out)
}

func TestOSTreeRemoteURLExtra(t *testing.T) {
	installFakeOstree(t)
	t.Setenv("OSTREE_REMOTE_URL", "http://example.com/repo")
	tree := newTestOSTree(t)
	assert.Equal(t, "http://example.com/repo", tree.RemoteURL())
}

func TestOSTreeRemoteURLEmptyExtra(t *testing.T) {
	installFakeOstree(t)
	tree := newTestOSTree(t)
	assert.Empty(t, tree.RemoteURL())
}

func TestOSTreeRemoteURLErrorExtra(t *testing.T) {
	// Point PATH at a directory without an `ostree` binary so the subprocess
	// fails to launch, exercising the RemoteURL error branch.
	t.Setenv("PATH", t.TempDir())
	tree := newTestOSTree(t)
	assert.Empty(t, tree.RemoteURL())
}

func TestOSTreeHasBranchTrueExtra(t *testing.T) {
	installFakeOstree(t)
	t.Setenv("OSTREE_REFS_OUT", "branch1\nbranch2")
	tree := newTestOSTree(t)
	assert.True(t, tree.HasBranch("branch2"))
}

func TestOSTreeHasBranchFalseExtra(t *testing.T) {
	installFakeOstree(t)
	t.Setenv("OSTREE_REFS_OUT", "branch1")
	tree := newTestOSTree(t)
	assert.False(t, tree.HasBranch("missing"))
}

func TestOSTreeHasBranchRefsFailedExtra(t *testing.T) {
	installFakeOstree(t)
	t.Setenv("OSTREE_REFS_EXIT", "1")
	tree := newTestOSTree(t)
	assert.False(t, tree.HasBranch("branch1"))
}

func TestOSTreeNeedCheckoutMissingFileExtra(t *testing.T) {
	installFakeOstree(t)
	tree := newTestOSTree(t)
	target := filepath.Join(t.TempDir(), "target")
	assert.True(t, tree.NeedCheckout("branch1", target))
}

func TestOSTreeNeedCheckoutRevMismatchExtra(t *testing.T) {
	installFakeOstree(t)
	t.Setenv("OSTREE_REV", "deadbeef")
	tree := newTestOSTree(t)
	target := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(target, ".checkout_commit"), []byte("other"), 0644))
	assert.True(t, tree.NeedCheckout("branch1", target))
}

func TestOSTreeNeedCheckoutMatchExtra(t *testing.T) {
	installFakeOstree(t)
	t.Setenv("OSTREE_REV", "deadbeef")
	tree := newTestOSTree(t)
	target := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(target, ".checkout_commit"), []byte("deadbeef"), 0644))
	assert.False(t, tree.NeedCheckout("branch1", target))
}

func TestOSTreeCheckoutForceExtra(t *testing.T) {
	installFakeOstree(t)
	t.Setenv("OSTREE_REV", "rev-123")
	tree := newTestOSTree(t)
	target := filepath.Join(t.TempDir(), "out", "target")
	require.NoError(t, tree.Checkout("branch1", target, true))
	content, err := os.ReadFile(filepath.Join(target, ".checkout_commit"))
	require.NoError(t, err)
	assert.Equal(t, "rev-123", string(content))
}

func TestOSTreeCheckoutNoopExtra(t *testing.T) {
	installFakeOstree(t)
	t.Setenv("OSTREE_REV", "rev-123")
	tree := newTestOSTree(t)
	target := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(target, ".checkout_commit"), []byte("rev-123"), 0644))
	// force=false and up-to-date: Checkout must return without writing.
	require.NoError(t, tree.Checkout("branch1", target, false))
}

func TestOSTreeCheckoutFailedExtra(t *testing.T) {
	installFakeOstree(t)
	t.Setenv("OSTREE_CHECKOUT_EXIT", "1")
	tree := newTestOSTree(t)
	target := filepath.Join(t.TempDir(), "out", "target")
	assert.Error(t, tree.Checkout("branch1", target, true))
}

func TestOSTreeCatExtra(t *testing.T) {
	installFakeOstree(t)
	t.Setenv("OSTREE_CAT_OUT", "file-body")
	tree := newTestOSTree(t)
	out, err := tree.Cat("branch1", "/etc/os-release")
	require.NoError(t, err)
	assert.Equal(t, "file-body", out)
}

func TestOSTreeBuildDoExtra(t *testing.T) {
	tree := &OSTree{repo: "/repo/path"}
	cmd := tree.buildDo("ls", "branch1", "/")
	assert.Equal(t, []string{"ostree", "--repo=/repo/path", "ls", "branch1", "/"}, cmd.Args)
}

func TestOSTreeDoSuccessExtra(t *testing.T) {
	installFakeOstree(t)
	tree := newTestOSTree(t)
	out, err := tree.do("cat", "branch1", "/f")
	require.NoError(t, err)
	assert.Equal(t, "content", out)
}

func TestOSTreeDoErrorExtra(t *testing.T) {
	installFakeOstree(t)
	t.Setenv("OSTREE_CHECKOUT_EXIT", "2")
	tree := newTestOSTree(t)
	_, err := tree.do("checkout", "branch1", "/target")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checkout")
}
