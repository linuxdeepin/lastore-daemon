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

	log "github.com/cihub/seelog"
	dbus "pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"

	"internal/system"
	"internal/utils"
)

var (
	qualityDataFilepath = "smartmirror_quality.json"
	configDataFilepath  = "smartmirror_config.json"
)

// SmartMirror handle core smart mirror data
type SmartMirror struct {
	Enable bool

	config        *config
	service       *dbusutil.Service
	mirrorQuality MirrorQuality
	sources       []system.MirrorSource
	sourcesURL    []string
	taskCount     int // TODO: need a lock???

	methods *struct { //nolint
		Query     func() `in:"origin, official, mirror" out:"url"`
		SetEnable func() `in:"enable"`
	}
}

// GetInterfaceName export dbus interface name
func (s *SmartMirror) GetInterfaceName() string {
	return "com.deepin.lastore.Smartmirror"
}

// newSmartMirror return a object with dbus
func newSmartMirror(service *dbusutil.Service) *SmartMirror {
	s := &SmartMirror{
		service:   service,
		taskCount: 0,
		config:    newConfig(path.Join(system.VarLibDir, configDataFilepath)),
		mirrorQuality: MirrorQuality{
			QualityMap:   make(QualityMap),
			adjustDelays: make(map[string]int),
			reportList:   make(chan []Report),
		},
	}

	s.Enable = s.config.Enable

	err := system.DecodeJson(path.Join(system.VarLibDir, qualityDataFilepath), &s.mirrorQuality.QualityMap)
	if nil != err {
		log.Info("load quality.json failed", err)
	}

	err = system.DecodeJson(path.Join(system.VarLibDir, "mirrors.json"), &s.sources)
	if nil != err {
		_ = log.Error(err)
	}

	for _, source := range s.sources {
		s.sourcesURL = append(s.sourcesURL, source.Url)
		s.mirrorQuality.adjustDelays[source.Url] = source.AdjustDelay
	}

	go func() {
		for reportList := range s.mirrorQuality.reportList {
			for _, r := range reportList {
				s.mirrorQuality.updateQuality(r)
				s.taskCount--
			}
			_ = utils.WriteData(path.Join(system.VarLibDir, qualityDataFilepath), s.mirrorQuality.QualityMap)
		}
	}()
	return s
}

// SetEnable the best source
func (s *SmartMirror) SetEnable(enable bool) *dbus.Error {
	changed := (s.Enable != enable)

	s.Enable = enable
	err := s.config.setEnable(enable)
	if nil != err {
		_ = log.Errorf("save config failed: %v", err)
		return dbus.NewError(err.Error(), nil)
	}

	if changed {
		err := s.service.EmitPropertyChanged(s, "Enable", enable)
		if nil != err {
			return dbus.NewError(err.Error(), nil)
		}
	}
	return nil
}

// Query the best source
func (s *SmartMirror) Query(original, officialMirror, mirrorHost string) (string, *dbus.Error) {
	if !s.Enable {
		source := strings.Replace(original, officialMirror, mirrorHost, 1)
		if utils.ValidURL(source) {
			return source, nil
		}
		return source, nil
	}
	result := s.route(original, officialMirror)
	return result, nil
}

func (s *SmartMirror) canQuit() bool {
	fmt.Println("canQuit", s.taskCount)
	return s.taskCount <= 0
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

	if 0 == len(mirrorHosts) {
		return original
	}

	for _, mirrorHost := range mirrorHosts {
		s.taskCount++
		go func(mirror string) {
			b := time.Now()
			urlMirror := strings.Replace(original, officialMirror, mirror, 1)
			v, statusCode := handleRequest(buildRequest(header, "HEAD", urlMirror))
			report := Report{
				Mirror:     mirror,
				URL:        v,
				Delay:      time.Since(b),
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
			r := <-detectReport
			reportList = append(reportList, r)
			if !r.Failed && !send {
				send = true
				result <- r
			}
			count++
			if count >= len(mirrorHosts) {
				end = true
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
		log.Info("begin -----------------------")
		log.Info("query", original)
		for i, v := range reportList {
			if 0 == i {
				log.Info("select", v.String())
			} else {
				log.Info("detect", v.String())
			}
		}
		// TODO: send an report
		log.Info("end -----------------------\n")
		s.mirrorQuality.reportList <- reportList
		header := makeReportHeader(reportList)
		handleRequest(buildRequest(header, "HEAD", original))
		close(detectReport)
	}()

	r := <-result
	close(result)
	if r.URL != "" {
		return r.URL
	}

	fmt.Println("error", "fallback", original)
	return original
}
