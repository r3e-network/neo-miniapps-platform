package main

import (
	"io"
	"net/http"
	"strings"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func jsonResponse(req *http.Request, statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:    io.NopCloser(strings.NewReader(body)),
		Request: req,
	}
}
