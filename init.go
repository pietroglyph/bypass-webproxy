/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

type configuration struct { // The configuration type holds configuration data
	Host                     string // Host string for the webserver to listen on
	Port                     string // Port string for the webserver to listen on
	PublicDir                string // Path string to the directory to serve static files from
	CacheStatic              bool   // Boolean to enable or disable file caching
	StripCORS                bool   // Boolean to strip CORS headers
	StripIntegrityAttributes bool   // Boolean to strip 'integrity' attributes in HTML
	StripFrameOptions        bool   // Boolean to strip X-Frame-Options headers
	ModifyHTML               bool   // Boolean to modify HTML
	ModifyCSS                bool   // Boolean to modify CSS
	ExternalURL              string // External URL string for formatting proxied HTML
	EnableTLS                bool   // Boolean to serve with TLS
	Verbose                  bool   // Boolean to disable logs of 404 errors
	TLSCertPath              string // Path to SSL Certificate
	TLSKeyPath               string // Path to private key for certificate
}

type reqHandler func(http.ResponseWriter, *http.Request) *reqError

var config configuration        // configuration for the entire program
var fileCache map[string][]byte // Files cached in the memory, stored as byte slices in a map that takes strings for the file names

func init() {
	// Configuration flags
	flag.BoolVar(&config.CacheStatic, "cachestatic", true, "cache specific heavily used static files")
	flag.BoolVar(&config.StripCORS, "cors", true, "strip Cross Origin Resource Policy headers")
	flag.BoolVar(&config.EnableTLS, "tls", false, "enable serving with TLS (https)")
	flag.BoolVar(&config.Verbose, "verbose", false, "enable printing 404 errors to STDOUT")
	flag.BoolVar(&config.StripFrameOptions, "frameoptions", true, "strip Frame Options headers to allow framing (if disabled this will break pub/index.html")
	flag.BoolVar(&config.StripIntegrityAttributes, "integrity", true, "strip 'integrity' attributes in HTML")
	flag.BoolVar(&config.ModifyCSS, "css", true, "modify CSS to pass URLs through the webproxy")
	flag.BoolVar(&config.ModifyHTML, "HTML", true, "modify HTML to pass URLs through the webproxy")
	flag.StringVar(&config.Host, "host", "localhost", "host to listen on for the webserver")
	flag.StringVar(&config.Port, "port", "8000", "port to listen on for the webserver")
	flag.StringVar(&config.PublicDir, "pubdir", "pub", "path to the static files the webserver should serve")
	flag.StringVar(&config.TLSCertPath, "tls-cert", "", "path to certificate file")
	flag.StringVar(&config.TLSKeyPath, "tls-key", "", "path to private key for certificate")
	flag.StringVar(&config.ExternalURL, "exturl", "", "external URL for formatting proxied HTML files to link back to the webproxy")
	flag.Parse() // Parse the rest of the flags
}

func main() { // Main function

	var err error

	if config.ExternalURL == "" {
		config.ExternalURL = "http://" + config.Host + ":" + config.Port // If nothing is specified, use the default host and port
	}

	fileCache = make(map[string][]byte) // Make the map for caching files
	if config.CacheStatic == true {     // Cache certain static files if they exist and if config.CacheStatic is set to true
		fileCache["index"], err = ioutil.ReadFile(config.PublicDir + "/index.html")
		if err != nil {
			fileCache["index"] = nil
		}
		fileCache["404"], err = ioutil.ReadFile(config.PublicDir + "/404.html")
		if err != nil {
			fileCache["404"] = nil
		}
	}
	// Create a HTTP Server, and handle requests and errors
	http.Handle("/", reqHandler(static))
	http.Handle("/p/", reqHandler(proxyHandler))
	bind := fmt.Sprintf("%s:%s", config.Host, config.Port)
	fmt.Printf("Bypass listening on %s...\n", bind)
	if !config.EnableTLS {
		err = http.ListenAndServe(bind, nil)
		if err != nil {
			panic(err)
		}
	} else if config.EnableTLS {
		if config.TLSCertPath == "" {
			panic(errors.New("tls-cert flag must not be empty"))
		}
		if config.TLSKeyPath == "" {
			panic(errors.New("tls-key flag must not be empty"))
		}
		fmt.Println("Serving with TLS...")
		err = http.ListenAndServeTLS(bind, config.TLSCertPath, config.TLSKeyPath, nil)
		if err != nil {
			panic(err)
		}
	}
}

func (fn reqHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) { // Allows us to pass errors back through our http handling functions
	if e := fn(w, r); e != nil { // e is *appError, not os.Error
		if e.Code == 404 { // Serve a pretty (potentially cached) file for 404 errors, if it exists
			w.WriteHeader(404)
			if config.Verbose && e.Error != nil {
				fmt.Println(e.Error.Error(), "\n", e.Message) // Print the error message
			}
			if fileCache["404"] != nil && config.CacheStatic { // Serve the cached file if one exists
				io.WriteString(w, string(fileCache["404"]))
			} else { // Read a non-cached file from disk and serve it because there isn't a cached one
				file, err := ioutil.ReadFile(config.PublicDir + "/404.html")
				if err != nil {
					if e.Error == nil { // Is there an included Error type
						http.Error(w, e.Message, e.Code) // Serve a generic error message if the file isn't cached and doesn't exist
					} else {
						http.Error(w, e.Message+"\n"+e.Error.Error(), e.Code) // Serve a generic error message if the file isn't cached and doesn't exist
					}
					return
				}
				io.WriteString(w, string(file))
			}
		} else { // If it's not a 404 error just serve a generic message
			if e.Error == nil {
				fmt.Println(e.Message)
				http.Error(w, e.Message, e.Code) // Serve a generic error message if the file isn't cached and doesn't exist
			} else {
				fmt.Println(e.Error.Error(), "\n", e.Message)         // Print the error message
				http.Error(w, e.Message+"\n"+e.Error.Error(), e.Code) // Serve a generic error message if the file isn't cached and doesn't exist
			}
		}

	}
}
