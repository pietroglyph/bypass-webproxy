/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
)

type contentType struct { // The contentType type holds easily usable information that is normally held as a string for indentifying MIME type and character encoding along with other information
	Type       string            // The first part of the MIME type (eg. "text")
	Subtype    string            // The second part of the MIME type (eg. "html")
	Parameters map[string]string // Any extra information (eg. "charset=utf8") represeted as a map
}

func parseContentType(rawcontype string) (*contentType, error) { // Parse a MIME string into a contentType struct
	rawcontype = strings.ToLower(rawcontype)
	var conType contentType
	conType.Parameters = make(map[string]string)
	splitcontype := strings.Split(rawcontype, " ")
	splitcontype[0] = strings.Replace(splitcontype[0], ";", "", -1)
	mimetype := strings.Split(splitcontype[0], "/")
	if len(mimetype) <= 1 {
		return new(contentType), errors.New("contype: malformed content-type MIME type provided")
	}
	if len(splitcontype) > 1 {
		params := strings.Split(splitcontype[1], ";")
		for it := range params {
			splitparams := strings.Split(params[it], "=")
			conType.Parameters[splitparams[0]] = splitparams[1]
		}
	}
	conType.Type = mimetype[0]
	conType.Subtype = mimetype[1]
	return &conType, nil
}

func formatURI(rawurl string, host string, baseurl string) (string, error) { // Formats a non-absolute URL or one with missing information into a hopefully valid one
	parsedurl, err := url.Parse(rawurl)
	if err != nil {
		return "", errors.New("main: couldn't parse provided URL in order to format it")
	}
	if parsedurl.IsAbs() {
		if !strings.HasPrefix(parsedurl.Scheme, "http") {
			parsedurl.Scheme = "http"
		}
	} else {
		base, err := url.Parse(host)
		if err != nil {
			return "", errors.New("main: couldn't parse provided host ( \"base\" ) in order to resolve a reference")
		}
		if !strings.HasPrefix(parsedurl.Scheme, "http") {
			base.Scheme = "http"
		}
		parsedurl = base.ResolveReference(parsedurl)
		if !strings.HasPrefix(parsedurl.Scheme, "http") {
			parsedurl.Scheme = "http"
		}
	}
	encodedurl := base64.StdEncoding.EncodeToString([]byte(parsedurl.String()))
	parsedProxyHost, err := url.Parse(baseurl)
	if err != nil {
		return "", errors.New("main: couldn't parse provided base url")
	}
	parsedProxyHost.Path += "/p/"
	q := parsedProxyHost.Query()
	q.Add("u", encodedurl)
	parsedProxyHost.RawQuery = q.Encode()
	return parsedProxyHost.String(), nil
}
