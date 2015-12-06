/*
This tools implement SmartMirrorDetector.

Run with `detector OfficialSite MirrorSite filename`
The result will be
	OfficialHit Hit = 1
	MirrorHit       = 2
	NotFoundHit     = 3

*/
package main

import (
	"fmt"
	"os"
	"time"
)

type Hit int

const (
	OfficialHit Hit = 1
	MirrorHit       = 2
	NotFoundHit     = 3
)

func (h Hit) String() string {
	switch h {
	case OfficialHit:
		return "Official Hit"
	case MirrorHit:
		return "Mirror Hit"
	case NotFoundHit:
		return "Not Found Any"
	default:
		return "Unknown Hit"
	}
}

func GetResultNow(ch chan Hit, defaultHit Hit, timeout time.Duration) Hit {
	select {
	case v := <-ch:
		return v
	case <-time.After(timeout):
		return defaultHit
	}
}

func SmartMirrorDetector(official, mirror string) Hit {
	checker := MakeChecker(official, mirror)

	officialResult, mirrorResult := make(chan Hit), make(chan Hit)
	go func() {
		if checker.CheckOfficial() {
			officialResult <- OfficialHit
		} else {
			officialResult <- NotFoundHit
		}
	}()

	go func() {
		if checker.CheckMirror() {
			mirrorResult <- MirrorHit
		} else {
			mirrorResult <- NotFoundHit

		}
	}()

	select {
	case v := <-mirrorResult:
		if v == MirrorHit {
			return v
		}
		return GetResultNow(officialResult, NotFoundHit, time.Second*2)
	case <-time.After(time.Second * 2):
		return GetResultNow(officialResult, NotFoundHit, 0)
	}
}

func main() {
	if len(os.Args) != 3 {
		fmt.Printf("Usage %s OfficialURL MirrorURL\n", os.Args[0])
		os.Exit(-1)
	}
	hit := SmartMirrorDetector(
		os.Args[1],
		os.Args[2],
	)
	fmt.Println(hit)
	os.Exit(int(hit))
}
