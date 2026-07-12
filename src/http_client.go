package main

import (
	"net/http"
	"time"
)

func newAlbumHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errTooManyRedirects
			}
			if !isBunkrHost(req.URL.Hostname()) {
				return errUnsupportedRedirectHost
			}
			return nil
		},
	}
}

func newDownloadHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 15 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errTooManyRedirects
			}
			return nil
		},
	}
}
