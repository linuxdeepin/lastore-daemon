/*
This tools implement SmartMirrorDetector.

Run with `detector url OfficialSite MirrorSite`
The result will be the right url.
*/
package main

import (
	"fmt"
	"os"
	"strings"
)

// MirrorURL return whether the url need be fetched by mirror
func MirrorURL(url string, official string, mirror string) (string, bool) {
	if strings.HasPrefix(url, official+"/pool") {
		return strings.Replace(url, official, mirror, 1), true
	} else if strings.HasPrefix(url, official+"/dists") && strings.Contains(url, "/by-hash/") {
		return strings.Replace(url, official, mirror, 1), true
	}
	return url, false
}

func main() {
	if len(os.Args) != 4 {
		fmt.Printf("Usage %s URL OfficialSite MirrorSite\n", os.Args[0])
		os.Exit(-1)
	}
	rawURL := os.Args[1]

	mirrorURL, ok := MirrorURL(rawURL, os.Args[2], os.Args[3])

	if !ok || mirrorURL == rawURL {
		fmt.Print(rawURL)
	} else {
		c := MakeChecker(rawURL, mirrorURL)
		fmt.Print(c.Result())
	}
}
