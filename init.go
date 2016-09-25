/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
)

type Configuration struct {
	Host        string
	Port        string
	PublicDir   string
	CacheStatic bool
}

type reqHandler func(http.ResponseWriter, *http.Request) *reqError

var Config Configuration
var FileCache map[string][]byte

func main() { // Main function
	// Read config
	file, err := os.Open("conf.json")
	if err != nil {
		fmt.Println("Could not read configuration file:", err.Error())
		return
	}
	decoder := json.NewDecoder(file)
	Config = Configuration{}
	err = decoder.Decode(&Config)
	if err != nil {
		fmt.Println("Could not parse configuration:", err.Error())
		return
	}
	FileCache = make(map[string][]byte)
	if Config.CacheStatic == true { // Cache certain static files if they exist and if Config.CacheStatic is set to true
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
	fmt.Printf("Proxy listening on %s...\n", bind)
	err = http.ListenAndServe(bind, nil)
	if err != nil {
		panic(err)
	}
}

func (fn reqHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) { // Allows us to pass errors back through our http handling functions
	if e := fn(w, r); e != nil { // e is *appError, not os.Error.
		fmt.Println(e.Error.Error())
		if e.Code == 404 {
			w.WriteHeader(404)
			if FileCache["404"] != nil {
				io.WriteString(w, string(FileCache["404"]))
			} else {
				file, err := ioutil.ReadFile(Config.PublicDir + "/404.html")
				if err != nil {
					http.Error(w, e.Message, e.Code)
					return
				}
				io.WriteString(w, string(file))
			}
		} else {
			http.Error(w, e.Message+"\n"+e.Error.Error(), e.Code)
		}

	}
}
