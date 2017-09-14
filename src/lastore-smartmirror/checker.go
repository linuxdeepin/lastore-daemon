/*
 * Copyright (C) 2015 ~ 2017 Deepin Technology Co., Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"strings"
)

func validURL(url string) bool {
	return strings.HasPrefix(url, "http")
}

func Route(original, officialHost, mirrorHost string) string {
	urlMirror := strings.Replace(original, officialHost, mirrorHost, 1)

	if !validURL(original) || !validURL(officialHost) || !validURL(mirrorHost) {
		// Just return raw url if there has any invalid input
		return original
	}

	if strings.HasPrefix(original, officialHost+"/pool") {
		return MakeChoice(original, urlMirror)
	} else if strings.HasPrefix(original, officialHost+"/dists") && strings.HasSuffix(original, "Release") {
		return HandleRequest(BuildRequest(MakeHeader(mirrorHost), "HEAD", original))
	} else if strings.HasPrefix(original, officialHost+"/dists") && strings.Contains(original, "/by-hash/") {
		return MakeChoice(original, urlMirror)
	}
	return original
}

func MakeChoice(original, mirror string) string {
	header := MakeHeader(mirror)
	officialResult := make(chan string)

	go func() {
		v := HandleRequest(BuildRequest(header, "HEAD", original))
		officialResult <- v
	}()

	if r := HandleRequest(BuildRequest(header, "GET", mirror)); r != "" {
		return r
	}

	select {
	case r := <-officialResult:
		if r != "" {
			return r
		}
	}
	return original
}
