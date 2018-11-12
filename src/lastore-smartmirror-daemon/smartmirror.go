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
	"fmt"
	"path"
	"strings"
	"time"

	"pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"

	"internal/mirrors"
	"internal/system"
	"internal/utils"
)

// SmartMirror handle core smart mirror data
type SmartMirror struct {
	service       *dbusutil.Service
	mirrorQuality MirrorQuality
	sources       []system.MirrorSource
	sourcesURL    []string

	methods *struct {
		Query func() `in:"origin, official" out:"url"`
	}
}

// GetInterfaceName export dbus interface name
func (s *SmartMirror) GetInterfaceName() string {
	return "com.deepin.lastore.Smartmirror"
}

// newSmartMirror return a object with dbus
func newSmartMirror(service *dbusutil.Service) *SmartMirror {
	s := &SmartMirror{
		service: service,
		mirrorQuality: MirrorQuality{
			QualityMap: make(QualityMap, 0),
			report:     make(chan Report),
		},
	}
	system.DecodeJson(path.Join(system.VarLibDir, "quality.json"), s.mirrorQuality.QualityMap)

	var err error
	s.sources, err = mirrors.LoadMirrorSources("")
	if nil != err {
		panic(err)
	}
	for _, source := range s.sources {
		s.sourcesURL = append(s.sourcesURL, source.Url)
	}

	go func() {
		for {
			select {
			case r := <-s.mirrorQuality.report:
				s.mirrorQuality.updateQuality(r)
			}
		}
	}()
	return s
}

// Query the best source
func (s *SmartMirror) Query(original, officialMirror string) (string, *dbus.Error) {
	fmt.Print("query", original)
	result := s.route(original, officialMirror)
	s.mirrorQuality.mux.Lock()
	utils.WriteData(path.Join(system.VarLibDir, "quality.json"), s.mirrorQuality.QualityMap)
	s.mirrorQuality.mux.Unlock()
	return result, nil
}

// route select new url by file path
func (s *SmartMirror) route(original, officialMirror string) string {
	if !utils.ValidURL(original) || !utils.ValidURL(officialMirror) {
		// Just return raw url if there has any invalid input
		return original
	}

	if strings.HasPrefix(original, officialMirror+"/pool") {
		return s.makeChoice(original, officialMirror)
	} else if strings.HasPrefix(original, officialMirror+"/dists") && strings.HasSuffix(original, "Release") {
		// Get Release from Release
		url, _ := handleRequest(buildRequest(makeHeader(), "HEAD", original))
		return url
	} else if strings.HasPrefix(original, officialMirror+"/dists") && strings.Contains(original, "/by-hash/") {
		return s.makeChoice(original, officialMirror)
	}
	return original
}

// makeChoice select best mirror by http request
func (s *SmartMirror) makeChoice(original, officialMirror string) string {
	header := makeHeader()
	detectReport := make(chan Report)
	result := make(chan Report)

	mirrorHosts := s.mirrorQuality.detectSelectMirror(s.sourcesURL)

	for _, mirrorHost := range mirrorHosts {
		go func(mirror string) {
			b := time.Now()
			urlMirror := strings.Replace(original, officialMirror, mirror, 1)
			v, statusCode := handleRequest(buildRequest(header, "HEAD", urlMirror))
			report := Report{
				Mirror:     mirror,
				URL:        v,
				Delay:      time.Now().Sub(b),
				Failed:     !utils.ValidURL(v),
				StatusCode: statusCode,
			}
			detectReport <- report
		}(mirrorHost)
	}

	go func() {
		count := 0
		send := false
		end := false
		reportList := []Report{}
		for {
			select {
			case r := <-detectReport:
				// fmt.Println("result", r)
				reportList = append(reportList, r)
				if !r.Failed && !send {
					send = true
					result <- r
				}
				s.mirrorQuality.report <- r
				count++
				if count >= len(mirrorHosts) {
					end = true
				}
			}
			if end {
				break
			}
		}
		if !send {
			result <- Report{
				URL:   "",
				Delay: 5 * time.Second,
			}
		}
		// dump report
		fmt.Println("\nbegin -----------------------")
		for i, v := range reportList {
			if 0 == i {
				fmt.Println("select", v)
			} else {
				fmt.Println("detect", v)
			}
		}
		// TODO: send an report
		fmt.Println("end -----------------------")
		header := makeReportHeader(reportList)
		handleRequest(buildRequest(header, "HEAD", original))
		close(detectReport)
	}()

	select {
	case r := <-result:
		close(result)
		if r.URL != "" {
			return r.URL
		}
	}
	fmt.Println("error", "fallback", original)
	return original
}
