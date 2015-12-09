package main

import (
	"net/http"
	"time"
)

type Checker struct {
	official string
	mirror   string
	header   map[string]string
	client   http.Client
}

func MakeChecker(official, mirror string) Checker {
	return Checker{
		header:   MakeHeader(mirror),
		official: official,
		mirror:   mirror,
	}
}

func GetResultNow(ch chan string, defaultValue string, timeout time.Duration) string {
	select {
	case v := <-ch:
		if v != "" {
			return v
		}
	case <-time.After(timeout):
	}
	return defaultValue
}

func (c *Checker) Result() string {
	officialResult, mirrorResult := make(chan string), make(chan string)
	go func() {
		v := c.CheckOfficial()
		officialResult <- v
	}()

	go func() {
		v := c.CheckMirror()
		mirrorResult <- v
	}()

	select {
	case v := <-mirrorResult:
		if v != "" {
			return v
		}
		r := GetResultNow(officialResult, c.official, time.Second*2)
		return r
	case <-time.After(time.Second * 2):
		return GetResultNow(officialResult, c.official, 0)
	}
}

func (c *Checker) makeRequest(method string, url string) (*http.Request, error) {
	r, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range c.header {
		r.Header.Set(k, v)
	}
	return r, nil
}

func (c *Checker) CheckOfficial() string {
	r, err := c.makeRequest("HEAD", c.official)
	if err != nil {
		return ""
	}
	resp, err := c.client.Do(r)
	if err != nil {
		return ""
	}
	resp.Body.Close()

	switch resp.StatusCode / 100 {
	case 4, 5:
		return ""
	case 3:
		u, err := resp.Location()
		if err != nil {
			return c.official
		}
		return u.String()
	case 2, 1:
		return c.official
	default:
		return ""
	}
}

// CheckURL check whether the remote url is valid
func (c *Checker) CheckMirror() string {
	r, err := c.makeRequest("GET", c.mirror)
	if err != nil {
		return ""
	}
	resp, err := c.client.Do(r)
	if err != nil {
		return ""
	}
	resp.Body.Close()

	switch resp.StatusCode / 100 {
	case 4, 5:
		return ""
	case 3:
		u, err := resp.Location()
		if err != nil {
			return c.mirror
		}
		return u.String()
	case 2, 1:
		return c.mirror
	default:
		return ""
	}
}
