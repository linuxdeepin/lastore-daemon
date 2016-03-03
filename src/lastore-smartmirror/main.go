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
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("Usage %s URL Official\n", os.Args[0])
		os.Exit(-1)
	}

	origin := os.Args[1]
	official := appendSuffix(os.Args[2], "/")

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
		EndAndReport(filename, official)
	}

	// Try serving this file from mirrors
	if strings.HasPrefix(filename, "pool") ||
		strings.HasPrefix(filename, "dists") && strings.Contains(filename, "/by-hash") {
		EndAndReport(filename, ChooseMirror(filename, official))
	}

	EndImmediately(origin)
}

// EndImmediately print the choice and exit immediately
func EndImmediately(url string) {
	fmt.Printf(url)
	os.Exit(0)
}

// EndAndReport print the choice and report it to official before exit
func EndAndReport(filename string, server string) {
	fmt.Printf(appendSuffix(server, "/") + filename)
	cmd := exec.Command("lastore-tools", "smartmirror",
		"report", "--server", server, filename)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Start()
	os.Exit(0)
}

// ChooseMirror use lastore-tools to detect dynamically
// the best server
func ChooseMirror(filename string, official string) string {
	cmd := exec.Command("lastore-tools", "smartmirror",
		"--official", official, "choose", filename)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	out, err := cmd.StdoutPipe()
	cmd.Start()

	r := bufio.NewReader(out)
	line, err := r.ReadString('\n')
	if err != nil {

		return official
	}
	go cmd.Wait()
	return strings.TrimSpace(strings.Replace(line, filename, "", 1))
}

func isSchema(url string, schema string) bool {
	return strings.HasPrefix(url, schema+"://")
}

func appendSuffix(r string, suffix string) string {
	if strings.HasSuffix(r, suffix) {
		return r
	}
	return r + suffix
}
