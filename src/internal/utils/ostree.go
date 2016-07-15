package utils

import (
	"fmt"
	"os"
	"os/exec"
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
	_, err := tree.do("init")
	if err != nil {
		return nil, err
	}
	_, err = tree.do("remote", "--no-gpg-verify", "add", "origin", remote)
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

func (tree *OSTree) Checkout(branch string, target string) error {
	_, err := tree.do("checkout", "--union", branch, target)
	return err
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
