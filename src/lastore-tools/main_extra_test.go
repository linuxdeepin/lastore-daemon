// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/codegangsta/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMainUpdater(t *testing.T) {
	dir := t.TempDir()
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	_ = set.String("output", filepath.Join(dir, "apps.json"), "")
	_ = set.String("job", "categories", "")
	_ = set.String("repo", "desktop", "")
	c := cli.NewContext(&cli.App{}, set, nil)
	assert.NoError(t, MainUpdater(c))
}

func TestMainUpdaterUpdateInfos(t *testing.T) {
	old := queryDpkgUpgradeInfoByAptListFn
	t.Cleanup(func() { queryDpkgUpgradeInfoByAptListFn = old })
	// Force the source-not-exist path so GenerateUpdateInfos never invokes apt.
	queryDpkgUpgradeInfoByAptListFn = func(string) ([]string, error) { return nil, os.ErrNotExist }

	dir := t.TempDir()
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	_ = set.String("output", filepath.Join(dir, "update_infos.json"), "")
	_ = set.String("job", "update_infos", "")
	_ = set.String("repo", "desktop", "")
	c := cli.NewContext(&cli.App{}, set, nil)
	assert.NoError(t, MainUpdater(c))
}

func TestMainUpdaterDefault(t *testing.T) {
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	_ = set.String("output", "", "")
	_ = set.String("job", "", "")
	_ = set.String("repo", "desktop", "")
	app := &cli.App{
		Writer:   io.Discard,
		Commands: []cli.Command{CMDUpdater},
	}
	c := cli.NewContext(app, set, nil)
	assert.NoError(t, MainUpdater(c))
}

func TestMainUpdaterXCategories(t *testing.T) {
	oldBase := BaseDir
	t.Cleanup(func() { BaseDir = oldBase })

	base := t.TempDir()
	BaseDir = base
	xcatDir := filepath.Join(base, "override", "xcategories")
	require.NoError(t, os.MkdirAll(xcatDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(xcatDir, "x.json"), []byte(`{"Audio":"multimedia"}`), 0644))

	output := filepath.Join(t.TempDir(), "xcategories.json")
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	_ = set.String("output", output, "")
	_ = set.String("job", "xcategories", "")
	_ = set.String("repo", "desktop", "")
	c := cli.NewContext(&cli.App{}, set, nil)
	assert.NoError(t, MainUpdater(c))

	data, err := os.ReadFile(output)
	require.NoError(t, err)
	assert.Contains(t, string(data), "multimedia")
}

func TestMainFunction(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"lastore-tools"}
	main()
}
