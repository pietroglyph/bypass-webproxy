/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"fmt"
	"github.com/kardianos/osext"
	"io"
	"io/ioutil"
	"net/http"
)

type Configuration struct { // The Configuration type holds configuration data
	Host        string // Host string for the webserver to listen on
	Port        string // Port string for the webserver to listen on
	PublicDir   string // Path string to the directory to serve static files from
	CacheStatic bool   // Boolean to enable or disable file caching
	DisableCORS bool   // Boolean to strip CORS headers
	ExternalURL string // External URL string for formatting proxied HTML
	EnableTLS   bool   // Boolean to serve with TLS
	Verbose     bool   // Boolean to disable logs of 404 errors
	TLSCertPath string // Path to SSL Certificate
	TLSKeyPath  string // Path to private key for certificate
}

type reqHandler func(http.ResponseWriter, *http.Request) *reqError

var Config Configuration        // Configuration for the entire program
var FileCache map[string][]byte // Files cached in the memory, stored as byte slices in a map that takes strings for the file names

func init() { // Init function
	folderPath, err := osext.ExecutableFolder() // Figure out where we are in the filesystem to make specifying the location of the public directory easier
	if err != nil {
		folderPath = ""          // If this doesn't work it's not a huge deal and we can just set the folder path to an empty string and print an error message
		fmt.Println(err.Error()) // Print an error message but don't do anything else
	}
	// Configuration flags
	flag.StringVar(&Config.Host, "host", "localhost", "host to listen on for the webserver")
	flag.StringVar(&Config.Port, "port", "8000", "port to listen on for the webserver")
	flag.StringVar(&Config.PublicDir, "pubdir", folderPath+"/pub", "path to the static files the webserver should serve")
	flag.BoolVar(&Config.CacheStatic, "cachestatic", true, "cache specific heavily used static files")
	flag.BoolVar(&Config.DisableCORS, "cors", true, "strip Cross Origin Resource Policy headers")
	flag.BoolVar(&Config.EnableTLS, "tls", false, "enable serving with TLS (https)")
	flag.BoolVar(&Config.Verbose, "verbose", false, "enable printing 404 errors to STDOUT")
	flag.StringVar(&Config.TLSCertPath, "tls-cert", folderPath+"/cert.pem", "path to certificate file")
	flag.StringVar(&Config.TLSKeyPath, "tls-key", folderPath+"/key.pem", "path to private key for certificate")
	flag.StringVar(&Config.ExternalURL, "exturl", "", "external URL for formatting proxied HTML files to link back to the webproxy")
	flag.Parse() // Parse the rest of the flags

}

func main() { // Main function

	var err error

	if Config.ExternalURL == "" {
		Config.ExternalURL = "http://" + Config.Host + ":" + Config.Port // If nothing is specified, use the defualt host and port
	}

	FileCache = make(map[string][]byte) // Make the map for caching files
	if Config.CacheStatic == true {     // Cache certain static files if they exist and if Config.CacheStatic is set to true
		FileCache["index"], err = ioutil.ReadFile(Config.PublicDir + "/index.html")
		if err != nil {
			FileCache["index"] = nil
		}
		FileCache["404"], err = ioutil.ReadFile(Config.PublicDir + "/404.html")
		if err != nil {
			FileCache["404"] = nil
		}
	}
	// Create a HTTP Server, and handle requests and errors
	http.Handle("/", reqHandler(static))
	http.Handle("/p/", reqHandler(proxy))
	bind := fmt.Sprintf("%s:%s", Config.Host, Config.Port)
	fmt.Printf("Bypass listening on %s...\n", bind)
	if !Config.EnableTLS {
		err = http.ListenAndServe(bind, nil)
		if err != nil {
			panic(err)
		}
	} else if Config.EnableTLS {
		fmt.Println("Serving with TLS...")
		err = http.ListenAndServeTLS(bind, Config.TLSCertPath, Config.TLSKeyPath, nil)
		if err != nil {
			panic(err)
		}
	}
}

func (fn reqHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) { // Allows us to pass errors back through our http handling functions
	if e := fn(w, r); e != nil { // e is *appError, not os.Error
		if e.Code == 404 { // Serve a pretty (potentially cached) file for 404 errors, if it exists
			w.WriteHeader(404)
			if Config.Verbose && e.Error != nil {
				fmt.Println(e.Error.Error(), "\n", e.Message) // Print the error message
			}
			if FileCache["404"] != nil { // Serve the cached file if one exists
				io.WriteString(w, string(FileCache["404"]))
			} else { // Read a non-cached file from disk and serve it because there isn't a cached one
				file, err := ioutil.ReadFile(Config.PublicDir + "/404.html")
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
