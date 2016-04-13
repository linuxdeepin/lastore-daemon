/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/
/*
This tools implement SmartMirrorDetector.

Run with `detector url Official`
The result will be the best url.
*/
package main

import (
	"bufio"
	"fmt"
	"internal/utils"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("Usage %s URL Official mirror\n", os.Args[0])
		os.Exit(-1)
	}

	origin := os.Args[1]
	official := os.Args[2]
	var mirror string
	if len(os.Args) >= 4 {
		mirror = os.Args[3]
	}

	// invliad url schema, e.g. cd://
	if !isSchema(origin, "http") || !isSchema(official, "http") {
		EndImmediately(origin)
	}

	filename := strings.Replace(origin, official, "", 1)

	// Can't serve this file from mirrors
	// because the origin url isn't from official server
	if filename == origin {
		EndImmediately(origin)
	}

	// Just touch file in official
	if strings.HasPrefix(filename, "dists") && strings.Contains(filename, "Release") {
		fmt.Printf(origin)
		utils.ReportChoosedServer(official, filename, official)
		os.Exit(0)
	}

	// Try serving this file from mirrors
	if strings.HasPrefix(filename, "pool") ||
		strings.HasPrefix(filename, "dists") && strings.Contains(filename, "/by-hash") {
		EndImmediately(ChooseMirror(filename, official, mirror))
	}

	EndImmediately(origin)
}

// EndImmediately print the choice and exit immediately
func EndImmediately(url string) {
	fmt.Printf(url)
	os.Exit(0)
}

// ChooseMirror use lastore-tools to detect dynamically
// the best server
func ChooseMirror(filename string, official string, mirror string) string {
	cmd := exec.Command("lastore-tools", "smartmirror",
		"--official", official, "choose", "-p", mirror, filename)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	out, err := cmd.StdoutPipe()
	cmd.Start()

	r := bufio.NewReader(out)
	line, err := r.ReadString('\n')
	if err != nil {
		return official
	}
	go cmd.Wait()
	server := strings.TrimSpace(line)
	return server + filename
}

func isSchema(url string, schema string) bool {
	return strings.HasPrefix(url, schema+"://")
}
