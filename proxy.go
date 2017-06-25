/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	goenc "github.com/mattn/go-encoding"
)

type reqError struct {
	Error   error
	Message string
	Code    int
}

type proxy struct { // The Proxy type holds request and response details for the proxy request
	RawUrl        string            // Raw URL that is formatted into Url
	Url           *url.URL          // Formatted URL as the URL type
	UrlString     string            // Formatted URL as a string
	Body          []byte            // The request Body as a byte slice
	ConType       *contentType      // Content type as parsed into the ContentType type
	Document      *goquery.Document // The body parsed into the Document type
	FormattedBody string            // The final formatted body, converted into the a string form Document
}

var urlRegexp *regexp.Regexp = regexp.MustCompile(`url(?:\(['"]?)(.*?)(?:['"]?\))`) // Regular expression for matching "url()" contents in CSS

func proxyHandler(resWriter http.ResponseWriter, reqHttp *http.Request) *reqError { // Handle requests to /p/
	defer func() { // Recover from a panic if one occurred
		if err := recover(); err != nil {
			fmt.Println(err)
			fmt.Fprint(resWriter, err)
		}
	}()

	var prox proxy
	var err error

	urldec, err := base64.StdEncoding.DecodeString(reqHttp.URL.Query().Get("u"))
	if err != nil {
		return &reqError{err, "Couldn't decode provided URL parameter.", 400}
	}

	prox.RawUrl = string(urldec) // Get the value from the url key of a posted form

	prox.Url, err = url.Parse(prox.RawUrl) // Parse the raw URL value we were given into somthing we can work with
	if err != nil {
		return &reqError{err, "Couldn't parse provided URL.", 400}
	}

	if !prox.Url.IsAbs() { // Is our URL absolute or not?
		prox.Url.Scheme = "http"
	} else { // If our URL is absolute, make sure the protocol is http(s)
		if !strings.HasPrefix(prox.Url.Scheme, "http") {
			prox.Url.Scheme = "http"
		}
	}
	prox.UrlString = prox.Url.String() // Turn our type URL back into a nice easy string, and store it in a variable

	client := &http.Client{} // Make a new http client

	request, err := http.NewRequest("GET", prox.UrlString, nil) // Make a new http GET request
	if err != nil {
		return &reqError{err, "Couldn't make a new http request with provided URL.", 400}
	}

	// Use the client's User-Agent
	request.Header.Set("User-Agent", reqHttp.Header.Get("User-Agent"))

	httpCliResp, err := client.Do(request) // Actually do the http request
	if err != nil {
		return &reqError{err, "Invalid URL, or server connectivity issue.", 400}
	}
	prox.Body, err = ioutil.ReadAll(httpCliResp.Body) // Read the response into another variable
	if err != nil {
		return &reqError{err, "Couldn't read returned body.", 400}
	}

	prox.ConType, err = parseContentType(httpCliResp.Header.Get("Content-Type")) // Get the MIME type of what we received from the Content-Type header
	if err != nil {
		prox.ConType, err = parseContentType(http.DetectContentType(prox.Body)) // Looks like we couldn't parse the Content-Type header, so we'll have to detect content type from the actual response body
		if err != nil {
			return &reqError{err, "Couldn't parse provided or detected content-type of document.", 400}
		}
	}

	if prox.ConType.Parameters["charset"] == "" { // Make sure that we have a charset if the website doesn't provide one (which is fairly common)
		tempConType, err := parseContentType(http.DetectContentType(prox.Body))
		if err != nil {
			fmt.Println(err.Error()) // Instead of failing we will just give the user a non-formatted page and print the error
		} else {
			prox.ConType.Parameters["charset"] = tempConType.Parameters["charset"]
		}
	}

	if prox.ConType.Type == "text" && prox.ConType.Subtype == "html" && prox.ConType.Parameters["charset"] != "" && config.ModifyHTML { // Does it say it's html with a valid charset
		resReader := strings.NewReader(string(prox.Body))
		if prox.ConType.Parameters["charset"] != "utf-8" {
			encoding := goenc.GetEncoding(prox.ConType.Parameters["charset"])
			if encoding == nil {
				return &reqError{nil, prox.ConType.Parameters["charset"] + " is an invalid encoding.", 400}
			}
			fmt.Println(encoding, prox.ConType.Parameters["charset"])
			decoder := encoding.NewDecoder()
			prox.Document, err = goquery.NewDocumentFromReader(decoder.Reader(resReader)) // Parse the response from our target website whose body has been freshly utf-8 encoded
			if err != nil {                                                               // Looks like we can't parse this, let's just spit out the raw response
				fmt.Fprint(resWriter, string(prox.Body))
				fmt.Println(err.Error(), prox.Url)
				return nil
			}
		} else {
			prox.Document, err = goquery.NewDocumentFromReader(resReader)
			if err != nil { // Looks like we can't parse this, let's just spit out the raw response
				fmt.Fprint(resWriter, string(prox.Body))
				fmt.Println(err.Error(), prox.Url)
				return nil
			}
		}
		prox.Document.Find("*[href]").Each(func(i int, s *goquery.Selection) { // Modify all href attributes
			origlink, exists := s.Attr("href")
			if exists {
				formattedurl, err := formatUri(origlink, prox.UrlString, config.ExternalURL)
				if err == nil {
					s.SetAttr("href", formattedurl)
					s.SetAttr("data-bypass-modified", "true")
				}
			}
		})
		prox.Document.Find("*[src]").Each(func(i int, s *goquery.Selection) { // Modify all src attributes
			origlink, exists := s.Attr("src")
			if exists {
				formattedurl, err := formatUri(origlink, prox.UrlString, config.ExternalURL)
				if err == nil {
					s.SetAttr("src", formattedurl)
					s.SetAttr("data-bypass-modified", "true")
				}
			}
		})

		if config.StripIntegrityAttributes {
			prox.Document.Find("*[integrity]").Each(func(i int, s *goquery.Selection) { // Modify all src attributes
				s.RemoveAttr("integrity")
			})
		}

		parsedhtml, err := goquery.OuterHtml(prox.Document.Selection)
		if err != nil {
			return &reqError{err, "Couldn't convert parsed document back to HTML.", 500}
		}
		prox.FormattedBody = parsedhtml

		for header := range httpCliResp.Header { // Copy over headers from the http response to our http response writer
			switch header {
			case "Content-Security-Policy":
				if !config.StripCORS {
					resWriter.Header().Del(header)
				}
			case "X-Frame-Options":
				if !config.StripFrameOptions {
					resWriter.Header().Del(header)
				}
			case "Content-Type":
				if prox.ConType.Type == "text" && prox.ConType.Subtype == "html" {
					resWriter.Header().Set(header, "text/html; charset=utf-8")
				}
			case "Content-Length":
				resWriter.Header().Set(header, string(len([]byte(prox.FormattedBody)))) // Get length in bytes of the formatted converted body
			default:
				resWriter.Header().Set(header, httpCliResp.Header.Get(header))
			}
		}

		_, err = fmt.Fprint(resWriter, prox.FormattedBody)
		if err != nil {
			return &reqError{err, "Couldn't write content to response.", 500}
		}
	} else if prox.ConType.Type == "text" && prox.ConType.Subtype == "css" && config.ModifyCSS {
		for header := range httpCliResp.Header { // Copy over headers from the http response to our http response writer
			switch header {
			case "Content-Security-Policy":
				if !config.StripCORS {
					resWriter.Header().Del(header)
				}
			case "X-Frame-Options":
				if !config.StripFrameOptions {
					resWriter.Header().Del(header)
				}
			case "Content-Type":
				resWriter.Header().Set(header, httpCliResp.Header.Get(header))
			case "Content-Length":
				resWriter.Header().Set(header, string(len(prox.Body))) // Get length in bytes of the formatted converted body
			default:
				resWriter.Header().Set(header, httpCliResp.Header.Get(header))
			}
		}

		replFunc := func(origUri string) string {
			submatch := urlRegexp.FindStringSubmatch(origUri)[1]                 // This is how we get the regex's capture group (we get google.com out of url("google.com)
			fUri, err := formatUri(submatch, prox.UrlString, config.ExternalURL) // Fully format the URI
			fmt.Println(origUri)
			if err != nil {
				fmt.Println(err)
				return origUri // If we can't format it just return the original
			}
			return "url('" + fUri + "')" // We also need to add the url() part back in
		}
		replacedBody := urlRegexp.ReplaceAllStringFunc(string(prox.Body), replFunc)
		_, err = fmt.Fprint(resWriter, replacedBody)
		if err != nil {
			return &reqError{err, "Couldn't write content to response.", 500}
		}

	} else { // It's not html apparently, just give the raw response
		for header := range httpCliResp.Header { // Copy over headers from the http response to our http response writer
			switch header {
			case "Content-Security-Policy":
				if !config.StripCORS {
					resWriter.Header().Del(header)
				}
			case "X-Frame-Options":
				if !config.StripFrameOptions {
					resWriter.Header().Del(header)
				}
			case "Content-Type":
				resWriter.Header().Set(header, httpCliResp.Header.Get(header))
			case "Content-Length":
				resWriter.Header().Set(header, string(len(prox.Body))) // Get length in bytes of the formatted converted body
			default:
				resWriter.Header().Set(header, httpCliResp.Header.Get(header))
			}
		}

		_, err = fmt.Fprint(resWriter, string(prox.Body))
		if err != nil {
			return &reqError{err, "Couldn't write content to response.", 500}
		}
	}

	return nil
}

func static(resWriter http.ResponseWriter, reqHttp *http.Request) *reqError { // Handle everything else to the root /
	defer func() { // Recover from a panic if one occurred
		if err := recover(); err != nil {
			fmt.Println(err)
			fmt.Fprint(resWriter, err)
		}
	}()

	var file []byte
	var err error

	if reqHttp.URL.Path == "/" {
		if fileCache["index"] != nil {
			file = fileCache["index"]
		} else {
			file, err = ioutil.ReadFile(config.PublicDir + "/index.html")
			if err != nil {
				return &reqError{err, "File not found.", 404}
			}
		}
	} else {
		file, err = ioutil.ReadFile(config.PublicDir + reqHttp.URL.Path)
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
