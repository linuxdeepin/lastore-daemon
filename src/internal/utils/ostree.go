// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
)

type OSTree struct {
	repo string
}

func NewOSTree(repo string, remote string) (*OSTree, error) {
	tree := &OSTree{repo}
	if remote == tree.RemoteURL() {
		return tree, nil
	}
	if err := EnsureBaseDir(repo); err != nil {
		return nil, err
	}
	_, _ = tree.do("init")
	_, err := tree.do("remote", "--no-gpg-verify", "add", "origin", remote)
	if err != nil {
		return nil, err
	}
	return tree, nil
}

func (tree *OSTree) Pull(branch string) error {
	cmd := tree.buildDo("pull", "origin", branch)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stderr
	return cmd.Run()
}

func (tree *OSTree) List(branch string, root string) (string, error) {
	return tree.do("ls", branch, root)
}

func (tree *OSTree) RemoteURL() string {
	url, err := tree.do("remote", "show-url", "origin")
	if err != nil {
		return ""
	}
	return url
}

func (tree *OSTree) HasBranch(branch string) bool {
	raw, err := tree.do("refs")
	if err != nil {
		return false
	}
	return strings.Contains(raw, branch)
}

// NeedCheckout check whether the target content by target/.checkout_commit file
func (tree *OSTree) NeedCheckout(branch string, target string) bool {
	bs, err := os.ReadFile(path.Join(target, ".checkout_commit"))
	if err != nil {
		return true
	}

	rev, err := tree.do("rev-parse", branch)
	if err != nil || string(rev) != string(bs) {
		return true
	}
	return false
}

func (tree *OSTree) Checkout(branch string, target string, force bool) error {
	if !force && !tree.NeedCheckout(branch, target) {
		return nil
	}

	if err := EnsureBaseDir(target); err != nil {
		return err
	}
	// TODO: Record the old file list and remove the they after checkout end
	_, err := tree.do("checkout", "--union", branch, target)
	if err != nil {
		return err
	}

	rev, err := tree.do("rev-parse", branch)
	if err != nil {
		return err
	}
	return os.WriteFile(path.Join(target, ".checkout_commit"), ([]byte)(rev), 0644)
}

func (tree *OSTree) Cat(branch string, fpath string) (string, error) {
	return tree.do("cat", branch, fpath)
}

func (tree *OSTree) buildDo(args ...string) *exec.Cmd {
	return exec.Command("ostree", append([]string{"--repo=" + tree.repo}, args...)...)
}

func (tree *OSTree) do(args ...string) (string, error) {
	cmd := tree.buildDo(args...)
	bs, err := cmd.CombinedOutput()
	r := strings.TrimSpace(string(bs))

	if err != nil {
		return r, fmt.Errorf("%q %v: %s", cmd.Args, err, r)
	}
	return r, nil
}
