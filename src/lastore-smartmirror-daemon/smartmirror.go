// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"

	"github.com/linuxdeepin/lastore-daemon/src/internal/system"
	"github.com/linuxdeepin/lastore-daemon/src/internal/utils"
)

var (
	qualityDataFilepath = "smartmirror_quality.json"
	configDataFilepath  = "smartmirror_config.json"
)

var stateDirectory = os.Getenv("STATE_DIRECTORY")

// SmartMirror handle core smart mirror data
type SmartMirror struct {
	Enable bool

	config        *config
	service       *dbusutil.Service
	mirrorQuality MirrorQuality
	sources       []system.MirrorSource
	sourcesURL    []string
	taskCount     int // TODO: need a lock???
}

// GetInterfaceName export dbus interface name
func (s *SmartMirror) GetInterfaceName() string {
	return "org.deepin.dde.Lastore1.Smartmirror"
}

// newSmartMirror return a object with dbus
func newSmartMirror(service *dbusutil.Service) *SmartMirror {
	s := &SmartMirror{
		service:   service,
		taskCount: 0,
		config:    newConfig(path.Join(stateDirectory, configDataFilepath)),
		mirrorQuality: MirrorQuality{
			QualityMap:   make(QualityMap),
			adjustDelays: make(map[string]int),
			reportList:   make(chan []Report),
		},
	}

	s.Enable = s.config.Enable

	err := system.DecodeJson(path.Join(stateDirectory, qualityDataFilepath), &s.mirrorQuality.QualityMap)
	if nil != err {
		logger.Info("load quality.json failed", err)
	}

	err = system.DecodeJson(path.Join(stateDirectory, "mirrors.json"), &s.sources)
	if nil != err {
		logger.Error(err)
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
			_ = utils.WriteData(path.Join(stateDirectory, qualityDataFilepath), s.mirrorQuality.QualityMap)
		}
	}()
	return s
}

// SetEnable the best source
func (s *SmartMirror) SetEnable(enable bool) *dbus.Error {
	changed := s.Enable != enable

	s.Enable = enable
	err := s.config.setEnable(enable)
	if nil != err {
		logger.Errorf("save config failed: %v", err)
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
func (s *SmartMirror) Query(original, officialMirror, mirrorHost string) (url string, busErr *dbus.Error) {
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

	if !strings.HasPrefix(original, officialMirror) {
		return original
	}

	if strings.Contains(original, "/pool/") {
		return s.makeChoice(original, officialMirror)
	} else if strings.Contains(original, "/dists/") && strings.HasSuffix(original, "Release") {
		// Get Release from Release
		url, _ := handleRequest(buildRequest(makeHeader(), "HEAD", original))
		return url
	} else if strings.Contains(original, "/dists/") && strings.Contains(original, "/by-hash/") {
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
		logger.Info("begin -----------------------")
		logger.Info("query", original)
		for i, v := range reportList {
			if 0 == i {
				logger.Info("select", v.String())
			} else {
				logger.Info("detect", v.String())
			}
		}
		// TODO: send an report
		logger.Info("end -----------------------\n")
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
