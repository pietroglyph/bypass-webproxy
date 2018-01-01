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

	"golang.org/x/net/html"

	"github.com/PuerkitoBio/goquery"
	"github.com/lukasbob/srcset"
	goenc "github.com/mattn/go-encoding"
)

type reqError struct {
	Error   error
	Message string
	Code    int
}

type proxy struct { // The Proxy type holds request and response details for the proxy request
	RawURL        string            // Raw URL that is formatted into URL
	ReqURL        *url.URL          // Formatted URL as the URL type
	FinalURL      string            // Formatted URL as a string
	Body          []byte            // The request Body as a byte slice
	ConType       *contentType      // Content type as parsed into the ContentType type
	Document      *goquery.Document // The body parsed into the Document type
	FormattedBody string            // The final formatted body, converted into the a string form Document
}

var err error

func proxyHandler(resWriter http.ResponseWriter, reqHTTP *http.Request) *reqError { // Handle requests to /p/
	defer func() { // Recover from a panic if one occurred
		if err := recover(); err != nil {
			fmt.Println(err)
			fmt.Fprint(resWriter, err)
		}
	}()

	var prox proxy
	urlRegexp := regexp.MustCompile(`url(?:\(['"]?)(.*?)(?:['"]?\))`) // Regular expression for matching "url()" contents in CSS

	replFunc := func(origURI string) string {
		submatch := urlRegexp.FindStringSubmatch(origURI)[1]                // This is how we get the regex's capture group (we get google.com out of url("google.com)
		fURI, err := formatURI(submatch, prox.FinalURL, config.ExternalURL) // Fully format the URI
		if err != nil {
			fmt.Println(err)
			return origURI // If we can't format it just return the original
		}
		return "url('" + fURI + "')" // We also need to add the url() part back in
	}

	urldec, err := base64.StdEncoding.DecodeString(reqHTTP.URL.Query().Get("u"))
	if err != nil {
		return &reqError{err, "Couldn't decode provided URL parameter.", 400}
	}

	prox.RawURL = string(urldec) // Get the value from the url key of a posted form

	prox.ReqURL, err = url.Parse(prox.RawURL) // Parse the raw URL value we were given into somthing we can work with
	if err != nil {
		return &reqError{err, "Couldn't parse provided URL.", 400}
	}

	if !prox.ReqURL.IsAbs() { // Is our URL absolute or not?
		prox.ReqURL.Scheme = "http"
	} else { // If our URL is absolute, make sure the protocol is http(s)
		if !strings.HasPrefix(prox.ReqURL.Scheme, "http") {
			prox.ReqURL.Scheme = "http"
		}
	}

	if prox.ReqURL.Port() != "80" && prox.ReqURL.Port() != "443" && prox.ReqURL.Port() != "" {
		return &reqError{nil, "Requests on ports other than 80 and 443 are forbidden to mitigate the possibility of port scanning as a result of the SSRF vulnerability inherent in this application's design.", 403}
	}

	err = isAllowedURL(prox.ReqURL)
	if err != nil {
		return &reqError{err, "You cannot request certain special IPs to mitigate the SSRF vulnerability inherent in this application's design.", 403}
	}

	client := &http.Client{} // Make a new http client

	request, err := http.NewRequest("GET", prox.ReqURL.String(), nil) // Make a new http GET request
	if err != nil {
		return &reqError{err, "Couldn't make a new http request with provided URL.", 400}
	}

	// Use the client's User-Agent
	request.Header.Set("User-Agent", reqHTTP.Header.Get("User-Agent"))

	httpCliResp, err := client.Do(request) // Actually do the http request
	if err != nil {
		return &reqError{err, "Invalid URL, or server connectivity issue.", 400}
	}
	prox.Body, err = ioutil.ReadAll(httpCliResp.Body) // Read the response into another variable
	if err != nil {
		return &reqError{err, "Couldn't read returned body.", 400}
	}

	prox.FinalURL = httpCliResp.Request.URL.String() // This accounts for redirects, and gives us the *final* URL

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

	//  Copy headers to the proxy's response, making modifications along the way
	for curHeader := range httpCliResp.Header {
		switch curHeader {
		case "Content-Security-Policy":
			if config.StripCORS {
				continue
			}
		case "X-Frame-Options":
			if config.StripFrameOptions {
				continue
			}
		case "Content-Type":
			if prox.ConType.Type == "text" && prox.ConType.Subtype == "html" {
				resWriter.Header().Set(curHeader, "text/html; charset=utf-8")
				continue
			}
		case "Content-Length":
			// This will automatically be written for our modified page by net/http, and we don't want to copy it
			continue
		}
		resWriter.Header().Set(curHeader, httpCliResp.Header.Get(curHeader))
	}
	resWriter.Header().Set("Access-Control-Allow-Origin", "*") // This always needs to be set

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
				fmt.Println(err.Error(), prox.ReqURL)
				return nil
			}
		} else {
			prox.Document, err = goquery.NewDocumentFromReader(resReader)
			if err != nil { // Looks like we can't parse this, let's just spit out the raw response
				fmt.Fprint(resWriter, string(prox.Body))
				fmt.Println(err.Error(), prox.ReqURL)
				return nil
			}
		}
		prox.Document.Find("*[href]").Each(func(i int, s *goquery.Selection) { // Modify all href attributes
			if len(s.Parent().Nodes) > 0 && s.Parent().Nodes[0].Type == html.ElementNode {
				if s.Parent().Nodes[0].Data == "svg" { // hrefs are different in SVGs
					return
				}
			}
			origlink, exists := s.Attr("href")
			if exists {
				formattedurl, err := formatURI(origlink, prox.FinalURL, config.ExternalURL)
				if err == nil {
					s.SetAttr("href", formattedurl)
					s.SetAttr("data-bypass-modified", "true")
				}
			}
		})
		prox.Document.Find("*[src]").Each(func(i int, s *goquery.Selection) { // Modify all src attributes
			origlink, exists := s.Attr("src")
			if exists {
				formattedurl, err := formatURI(origlink, prox.FinalURL, config.ExternalURL)
				if err == nil {
					s.SetAttr("src", formattedurl)
					s.SetAttr("data-bypass-modified", "true")
				}
			}
		})
		prox.Document.Find("*[srcset]").Each(func(i int, s *goquery.Selection) { // Modify all srcset attributes
			origlink, exists := s.Attr("srcset")
			if exists {
				srcset := srcset.Parse(origlink)
				replacedurl := origlink
				for i := range srcset {
					formattedurl, err := formatURI(srcset[i].URL, prox.FinalURL, config.ExternalURL)
					if err == nil {
						fmt.Println(formattedurl)
						replacedurl = strings.Replace(replacedurl, srcset[i].URL, formattedurl, 1)
						s.SetAttr("srcset", replacedurl)
						s.SetAttr("data-bypass-modified", "true")
					}
				}
			}
		})
		prox.Document.Find("*[style]").Each(func(i int, s *goquery.Selection) { // Modify all srcset attributes
			style, exists := s.Attr("style")
			if exists {
				replacedStyle := urlRegexp.ReplaceAllStringFunc(style, replFunc)
				s.SetAttr("style", replacedStyle)
				s.SetAttr("data-bypass-modified", "true")
			}
		})
		prox.Document.Find("*[poster]").Each(func(i int, s *goquery.Selection) { // Modify all poster attributes
			origlink, exists := s.Attr("poster")
			if exists {
				formattedurl, err := formatURI(origlink, prox.FinalURL, config.ExternalURL)
				if err == nil {
					s.SetAttr("poster", formattedurl)
					s.SetAttr("data-bypass-modified", "true")
				}
			}
		})

		if config.StripIntegrityAttributes {
			prox.Document.Find("*[integrity]").Each(func(i int, s *goquery.Selection) { // Remove integrity attributes, because we modify CSS
				s.RemoveAttr("integrity")
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
	} else if prox.ConType.Type == "text" && prox.ConType.Subtype == "css" && config.ModifyCSS {
		replacedBody := urlRegexp.ReplaceAllStringFunc(string(prox.Body), replFunc)
		_, err = fmt.Fprint(resWriter, replacedBody)
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

func static(resWriter http.ResponseWriter, reqHTTP *http.Request) *reqError { // Handle everything else to the root /
	defer func() { // Recover from a panic if one occurred
		if err := recover(); err != nil {
			fmt.Println(err)
			fmt.Fprint(resWriter, err)
		}
	}()

	var file []byte

	if reqHTTP.URL.Path == "/" {
		if fileCache["index"] != nil && config.CacheStatic {
			file = fileCache["index"]
		} else {
			file, err = ioutil.ReadFile(config.PublicDir + "/index.html")
			if err != nil {
				return &reqError{err, "File not found.", 404}
			}
		}
	} else {
		file, err = ioutil.ReadFile(config.PublicDir + reqHTTP.URL.Path)
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
