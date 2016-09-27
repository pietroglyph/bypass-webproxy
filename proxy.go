/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	//"errors"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type reqError struct {
	Error   error
	Message string
	Code    int
}

type Proxy struct {
	RawUrl        string
	Url           *url.URL
	UrlString     string
	Body          []byte
	ConType       *ContentType
	Document      *goquery.Document
	FormattedBody string
	Params        map[string]string
}

func proxy(resWriter http.ResponseWriter, reqHttp *http.Request) *reqError { // Handle requests to /p/
	defer func() { // Recover from a panic if one occured
		if err := recover(); err != nil {
			fmt.Println(err)
			fmt.Fprint(resWriter, err)
		}
	}()

	var prox Proxy
	var err error
	prox.Params = make(map[string]string)

	urldec, err := base64.StdEncoding.DecodeString(reqHttp.URL.Query().Get("u"))
	if err != nil {
		return &reqError{err, "Couldn't decode provided URL parameter.", 400}
	}

	prox.RawUrl = string(urldec)           // Get the value from the url key of a posted form
	prox.Url, err = url.Parse(prox.RawUrl) // Parse the raw URL value we were given into somthing we can work with
	if err != nil {
		return &reqError{err, "Couldn't parse provided URL.", 400}
	}

	if reqHttp.URL.Query().Get("params") != "" {
		rawparams, err := base64.StdEncoding.DecodeString(reqHttp.URL.Query().Get("params"))
		if err == nil {
			paramsl := strings.Split(string(rawparams), ";")
			for i := range paramsl {
				curparam := strings.Split(paramsl[i], ":")
				prox.Params[curparam[0]] = curparam[1]
			}
		}
	}
	fmt.Println(prox.Params)
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

	//copyHeaders(request.Header, reqHttp.Header) // Copy over headers from the actual client to our new http request

	cliResp, err := client.Do(request)
	if err != nil {
		return &reqError{err, "Invalid URL, or server connectivity issue.", 500}
	}
	prox.Body, err = ioutil.ReadAll(cliResp.Body)
	if err != nil {
		return &reqError{err, "Couldn't read returned body.", 500}
	}

	copyHeaders(resWriter.Header(), cliResp.Header) // Copy over headers from the actual client to our new http request

	prox.ConType, err = parseContentType(cliResp.Header.Get("Content-Type"))
	if err != nil {
		prox.ConType, err = parseContentType(http.DetectContentType(prox.Body))
		if err != nil {
			return &reqError{err, "Couldn't parse provided or detected content-type of document.", 500}
		}
	}

	if prox.ConType.Type == "text" && prox.ConType.Subtype == "html" && prox.Params["modify"] != "true" { // Is it html
		r := strings.NewReader(string(prox.Body))
		prox.Document, err = goquery.NewDocumentFromReader(r) // Parse the response from our target website
		if err != nil {                                       // Looks like we can't parse this, let's just spit out the raw response
			fmt.Fprint(resWriter, string(prox.Body))
			return nil
		}
		prox.Document.Find("*[href]").Each(func(i int, s *goquery.Selection) { // Modify all href attributes
			origlink, exists := s.Attr("href")
			if exists {
				formattedurl, err := formatUrl(origlink, prox.UrlString, Config.Host)
				if err == nil {
					s.SetAttr("href", formattedurl)
				}
			}
		})
		prox.Document.Find("*[src]").Each(func(i int, s *goquery.Selection) { // Modify all src attributes
			origlink, exists := s.Attr("src")
			if exists {
				formattedurl, err := formatUrl(origlink, prox.UrlString, Config.Host)
				if err == nil {
					s.SetAttr("src", formattedurl)
				}
			}
		})
		if prox.Params["stripjs"] == "true" {
			prox.Document.Find("script").Each(func(i int, s *goquery.Selection) { // Modify all src attributes
				s.Remove()
			})
		}
		if prox.Params["stripmeta"] == "true" {
			prox.Document.Find("meta").Each(func(i int, s *goquery.Selection) { // Modify all src attributes
				s.Remove()
			})
		}
		parsedhtml, err := goquery.OuterHtml(prox.Document.Selection)
		if err != nil {
			return &reqError{err, "Couldn't convert parsed document back to HTML.", 500}
		}
		prox.FormattedBody = parsedhtml
		_, err = fmt.Fprint(resWriter, prox.FormattedBody)
		if err != nil {
			return &reqError{err, "Couldn't write content to response.", 500}
		}
	} else { // It's not html apparently, just give the raw response
		_, err = fmt.Fprint(resWriter, string(prox.Body))
		if err != nil {
			return &reqError{err, "Couldn't write content to response.", 500}
		}
	}

	return nil
}

func static(resWriter http.ResponseWriter, reqHttp *http.Request) *reqError { // Handle everything else to the root /
	defer func() { // Recover from a panic if one occured
		if err := recover(); err != nil {
			fmt.Println(err)
			fmt.Fprint(resWriter, err)
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
