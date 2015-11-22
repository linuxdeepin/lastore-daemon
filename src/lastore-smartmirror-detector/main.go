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
	"net/http"
	"os"
	"time"
)

type Hit int

const (
	OfficialHit Hit = 1
	MirrorHit       = 2
	NotFoundHit     = 3
)

// CheckURL check whether the remote url is valid
func CheckURL(url string) bool {
	resp, err := http.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()

	switch resp.StatusCode / 100 {
	case 4, 5:
		return false
	case 2, 1, 3:
		return true
	default:
		return false
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
	officialResult, mirrorResult := make(chan Hit), make(chan Hit)
	go func() {
		if CheckURL(official) {
			officialResult <- OfficialHit
		} else {
			officialResult <- NotFoundHit
		}
	}()

	go func() {
		if CheckURL(mirror) {
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
	if len(os.Args) != 4 {
		os.Exit(-1)
	}
	hit := SmartMirrorDetector(
		os.Args[1]+"/"+os.Args[3],
		os.Args[2]+"/"+os.Args[3],
	)
	os.Exit(int(hit))
}
