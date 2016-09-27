/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

type ContentType struct {
	Type       string
	Subtype    string
	Parameters map[string]string
}

func parseContentType(rawcontype string) (*ContentType, error) { // Parse a MIME string into a ContentType struct
	var contentType ContentType
	contentType.Parameters = make(map[string]string)
	contype := strings.Split(rawcontype, " ")
	contype[0] = strings.Replace(contype[0], ";", "", -1)
	mimetype := strings.Split(contype[0], "/")
	if len(mimetype) <= 1 {
		return new(ContentType), errors.New("contype: malformed content-type MIME type provided")
	}
	if len(contype) > 1 {
		params := strings.Split(contype[1], ";")
		for it := range params {
			splitparams := strings.Split(params[it], "=")
			contentType.Parameters[splitparams[0]] = splitparams[1]
		}
	}
	contentType.Type = mimetype[0]
	contentType.Subtype = mimetype[1]
	return &contentType, nil
}

func formatUrl(rawurl string, host string, proxyhost string) (string, error) { // Formats a non-absolute URL or one with missing information into a hopefully valid one
	parsedurl, err := url.Parse(rawurl)
	if err != nil {
		return "", errors.New("main: couldn't parse provided URL in order to format it")
	}
	if parsedurl.IsAbs() {
		if parsedurl.Scheme != "http" || parsedurl.Scheme != "https" {
			parsedurl.Scheme = "http"
		}
	} else {
		base, err := url.Parse(host)
		if err != nil {
			return "", errors.New("main: couldn't parse provided host ( \"base\" ) in order to resolve a reference")
		}
		if base.Scheme != "http" || base.Scheme != "https" || base.Scheme == "" {
			base.Scheme = "http"
		}
		parsedurl = base.ResolveReference(parsedurl)
		if parsedurl.Scheme != "http" || parsedurl.Scheme != "https" || parsedurl.Scheme == "" {
			parsedurl.Scheme = "http"
		}
	}
	formattedurl := "http://" + proxyhost + "/p/?target=" + parsedurl.String()
	return formattedurl, nil
}

func copyHeaders(dest http.Header, source http.Header) { // Copy one http.Header to another http.Header
	for header := range source {
		if !Config.DisableCORS {
			dest.Add(header, source.Get(header))
		} else if header != "Content-Security-Policy" {
			dest.Add(header, source.Get(header))
		}
	}
}
