# bypass-webproxy [![Build Status](https://travis-ci.org/pietroglyph/bypass-webproxy.svg?branch=master)](https://travis-ci.org/pietroglyph/bypass-webproxy) [![License](https://img.shields.io/badge/license-MPL--2.0-orange.svg)](https://github.com/pietroglyph/bypass-webproxy/blob/master/LICENSE) [![Go Report Card](https://goreportcard.com/badge/github.com/pietroglyph/bypass-webproxy)](https://goreportcard.com/report/github.com/pietroglyph/bypass-webproxy)

A simple webproxy written in Go that uses Goquery to parse and modify proxied HTML pages so that links, images, and other resources are fed back through the proxy. Bypass also serves static files.

## Dependencies

+ [goquery](https://github.com/PuerkitoBio/goquery)
+ [osext](https://github.com/kardianos/osext)
+ [iconv-go](https://github.com/djimenez/iconv-go)
+ [go-encoding](https://github.com/mattn/go-encoding)

## Building

[Install Go](https://golang.org/doc/install) and [set the `$GOPATH` environment variable](https://golang.org/doc/code.html#GOPATH).

Run the following commands (assumes that you're using BASH):
1. ` $ go get github.com/pietroglyph/bypass-webproxy`
2. ` $ cd $GOPATH/src/github.com/pietroglyph/bypass-webproxy`
3. ` $ go get -d ./...`
4. ` $ go install`

## Usage

With the defualt arguments, your working directory when running Bypass needs to contain a directory called `pub`, if you want to have a working web interface. You probably want the `pub` folder in Bypass' source directory.

The following should get Bypass running if you've already built it:
1. ` $ cd $GOPATH/src/github.com/pietroglyph/bypass-webproxy`
2. ` $ $GOPATH/bin/bypass-webproxy`
3. Open http://localhost:8000 in your favorite browser

To see all the possible arguments type ` $ $GOPATH/bin/bypass-webproxy -h`.
