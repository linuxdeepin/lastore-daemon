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

Run with `lastore-smartmirror rawUrl officialServer preferenceServer`

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

// AutoChoose use preference server no matter the latency,
// unless the request file on the server is unreachable.
//
// If the request is unreachable in preference mirror
// then it will auto choose one according the recently
// request quality records.
// It will force choose official server either
// due to the 5s timeout or there hasn't any reachable server.
func AutoChoose(raw string, official string, preference string) string {
	official = utils.AppendSuffix(official, "/")
	if preference != "" {
		preference = utils.AppendSuffix(preference, "/")
	}

	// invliad url schema, e.g. cd://
	if !strings.HasPrefix(raw, "http://") ||
		!strings.HasPrefix(official, "http://") {
		return show(raw)
	}

	filename := strings.Replace(raw, official, "", 1)

	// Can't serve this file from mirrors
	// because the origin url isn't from official server
	if filename == raw {
		return show(raw)
	}

	// Just touch file in official
	if strings.HasPrefix(filename, "dists") && strings.Contains(filename, "Release") {
		show(raw)
		utils.ReportChoosedServer(official, filename, official)
		return raw
	}

	// Try serving this file from mirrors
	if strings.HasPrefix(filename, "pool") ||
		strings.HasPrefix(filename, "dists") && strings.Contains(filename, "/by-hash") {
		return show(chooseMirror(filename, official, preference))
	}

	return show(raw)
}

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("Usage %s URL Official mirror\n", os.Args[0])
		os.Exit(-1)
	}

	origin := os.Args[1]

	official := os.Args[2]

	var preference string
	if len(os.Args) >= 4 {
		preference = os.Args[3]
	}

	AutoChoose(origin, official, preference)
}

// show the choosed server
func show(url string) string {
	fmt.Printf(url)
	return url
}

// chooseMirror use lastore-tools to detect dynamically
// the best server
func chooseMirror(filename string, official string, mirror string) string {
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
