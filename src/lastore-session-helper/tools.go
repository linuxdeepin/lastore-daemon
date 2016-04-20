/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package main

import (
	"dbus/org/freedesktop/login1"
	"internal/system"
	"os"
	"path"
	"pkg.deepin.io/lib/dbus"
	"strings"
)

// QueryLang return user lang.
// the rule is document at man gettext(3)
func QueryLang() string {
	return QueryLangs()[0]
}

// QueryLangs return array of user lang, split by ":".
// the rule is document at man gettext(3)
func QueryLangs() []string {
	LC_ALL := os.Getenv("LC_ALL")
	LC_MESSAGE := os.Getenv("LC_MESSAGE")
	LANGUAGE := os.Getenv("LANGUAGE")
	LANG := os.Getenv("LANG")

	cutoff := func(s string) string {
		for i, c := range s {
			if c == '.' {
				return s[:i]
			}
		}
		return s
	}

	if LC_ALL != "C" && LANGUAGE != "" {
		var r []string
		for _, lang := range strings.Split(LANGUAGE, ":") {
			r = append(r, cutoff(lang))
		}
		return r
	}

	if LC_ALL != "" {
		return []string{cutoff(LC_ALL)}
	}
	if LC_MESSAGE != "" {
		return []string{cutoff(LC_MESSAGE)}
	}
	if LANG != "" {
		return []string{cutoff(LANG)}
	}
	return []string{""}
}

func PackageName(pkg string, lang string) string {
	names := make(map[string]struct {
		Id         string            `json:"id"`
		Name       string            `json:"name"`
		LocaleName map[string]string `json:"locale_name"`
	})

	system.DecodeJson(path.Join(system.VarLibDir, "applications.json"), &names)

	info, ok := names[pkg]
	if !ok {
		return pkg
	}

	name := info.LocaleName[lang]
	if name == "" {
		name = info.Name
	}
	return name
}

func Inhibitor(what, who, why string) (dbus.UnixFD, error) {
	m, err := login1.NewManager("org.freedesktop.login1", "/org/freedesktop/login1")
	if err != nil {
		return -1, err
	}
	defer login1.DestroyManager(m)
	return m.Inhibit(what, who, why, "block")
}

func FileExist(fpath string) bool {
	_, err := os.Stat(fpath)
	return err == nil || os.IsExist(err)
}
