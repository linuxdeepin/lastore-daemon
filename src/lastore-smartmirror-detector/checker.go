package main

import (
	"net/http"
)

func result(statusCode int) bool {
	switch statusCode / 100 {
	case 4, 5:
		return false
	case 2, 1, 3:
		return true
	default:
		return false
	}

}

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

func (c *Checker) CheckOfficial() bool {
	r, err := c.makeRequest("HEAD", c.official)
	if err != nil {
		return false
	}
	resp, err := c.client.Do(r)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return result(resp.StatusCode)
}

// CheckURL check whether the remote url is valid
func (c *Checker) CheckMirror() bool {
	r, err := c.makeRequest("GET", c.mirror)
	if err != nil {
		return false
	}
	resp, err := c.client.Do(r)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return result(resp.StatusCode)
}
