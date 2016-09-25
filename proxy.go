/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	//"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type reqError struct {
	Error   error
	Message string
	Code    int
}

type Proxy struct {
	RawUrl    string
	Url       *url.URL
	UrlString string
	Body      []byte
}

func proxy(resWriter http.ResponseWriter, reqHttp *http.Request) *reqError { // Handle requests to /p/
	defer func() { // Recover from a panic if one occured
		if err := recover(); err != nil {
			fmt.Println(err)
			//return &reqError{errors.New("main: panic"), "Panic while serving proxy!", 500}
		}
	}()

	var prox Proxy
	var err error

	prox.RawUrl = reqHttp.URL.Query().Get("target") // Get the value from the url key of a posted form
	prox.Url, err = url.Parse(prox.RawUrl)          // Parse the raw URL value we were given into somthing we can work with
	if err != nil {
		return &reqError{err, "Couldn't parse provided URL.", 500}
	}
	if !prox.Url.IsAbs() { // Is our URL absolute or not?
		prox.Url.Scheme = "http"
	} else { // If our URL is absolute, make sure the protocol is http(s)
		if prox.Url.Scheme != "http" || prox.Url.Scheme != "https" {
			prox.Url.Scheme = "http"
		}
	}
	prox.UrlString = prox.Url.String() // Turn our type URL back into a nice easy string, and store it in a variable

	client := &http.Client{} // Make a new http client

	request, err := http.NewRequest("GET", prox.UrlString, nil) // Make a new http GET request
	if err != nil {
		return &reqError{err, "Couldn't make a new http request.", 500}
	}

	copyHeaders(request.Header, reqHttp.Header) // Copy over headers from the actual client to our new http request

	cliResp, err := client.Do(request)
	fmt.Println(prox)
	if err != nil {
		return &reqError{err, "Invalid URL, or server connectivity issue.", 500}
	}
	prox.Body, err = ioutil.ReadAll(cliResp.Body)
	if err != nil {
		return &reqError{err, "Couldn't read returned body.", 500}
	}
	fmt.Fprintf(resWriter, string(prox.Body))
	fmt.Println(prox)
	return nil
}

func static(resWriter http.ResponseWriter, reqHttp *http.Request) *reqError { // Handle everything else to the root /
	defer func() { // Recover from a panic if one occured
		if err := recover(); err != nil {
			fmt.Println(err)
			//return &reqError{errors.New("main: panic"), "Panic while serving static!", 500}
		}
	}()

	var file []byte
	var err error

	if reqHttp.URL.Path == "/" {
		if FileCache["index"] != nil {
			file = FileCache["index"]
		} else {
			file, err = ioutil.ReadFile(Config.PublicDir + "/index.html")
			if err != nil {
				return &reqError{err, "File not found.", 404}
			}
		}
	} else {
		file, err = ioutil.ReadFile(Config.PublicDir + reqHttp.URL.Path)
		if err != nil {
			return &reqError{err, "File not found.", 404}
		}
	}
	_, err = fmt.Fprint(resWriter, string(file))
	if err != nil {
		return &reqError{err, "Couldn't write a response.", 500}
	}
	return nil
}
