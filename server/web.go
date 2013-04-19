/*
	This file supports the DVID REST API, breaking down URLs into
	commands and massaging attached data into appropriate data types. 
*/

package server

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/janelia-flyem/dvid/datastore"
	"github.com/janelia-flyem/dvid/dvid"
)

func badRequest(w http.ResponseWriter, r *http.Request, err string) {
	errorMsg := fmt.Sprintf("ERROR using REST API: %s (%s).", err, r.URL.Path)
	errorMsg += "  Use 'dvid help' to get proper API request format.\n"
	dvid.Error(errorMsg)
	http.Error(w, errorMsg, http.StatusBadRequest)
}

const webClientUnavailableMessage = `
DVID Web Client Unavailable!  To make the web client available, you have two choices:

1) Invoke the DVID server using the full path to the DVID executable to use
   the built-in web client.

2) Specify a path to web pages that implement a web client via the "-webclient=PATH"
   option to dvid.
   Example: dvid -webclient=/path/to/html/files serve dir=/path/to/db"
`

// Index file redirection.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/index.html", http.StatusMovedPermanently)
}

// Handler for presentation files
func mainHandler(w http.ResponseWriter, r *http.Request) {
	if runningService.WebClientPath != "" {
		path := "index.html"
		if r.URL.Path != "/" {
			path = r.URL.Path
		}
		filename := filepath.Join(runningService.WebClientPath, path)
		dvid.Log(dvid.Debug, "http request: %s -> %s\n", r.URL.Path, filename)
		http.ServeFile(w, r, filename)
	} else {
		fmt.Fprintf(w, webClientUnavailableMessage)
	}
}

//       GET /api/data
//       GET /api/data/versions
//       GET /api/data/datasets
//       (POST will add a named data set for a given data type.)
//       POST /api/data/<data type>/<data set name>
func handleDataRequest(w http.ResponseWriter, r *http.Request) {
	// Break URL request into arguments
	const lenPath = len(RestApiPath)
	url := r.URL.Path[lenPath:]
	parts := strings.Split(url, "/")
	action := strings.ToLower(r.Method)
	if action == "post" {
		// Handle setting of data sets
		if len(parts) != 3 {
			msg := fmt.Sprintf("Bad data set creation format (%s).  Try something like '%s' instead.",
				url, "POST /api/data/grayscale8/grayscale")
			badRequest(w, r, msg)
			return
		}
		dataType := parts[1]
		dataSetName := datastore.DataSetString(parts[2])
		err := runningService.NewDataSet(dataSetName, dataType)
		if err != nil {
			msg := fmt.Sprintf("Could not add data set '%s' of type '%s': %s",
				dataSetName, dataType, err.Error())
			badRequest(w, r, msg)
			return
		}
	} else {
		jsonStr, err := runningService.ConfigJSON()
		if err != nil {
			badRequest(w, r, err.Error())
		} else {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, jsonStr)
		}
	}
}

// Handler for API commands.
// We assume all DVID API commands target the URLs /api/<command or data set name>/... 
// Built-in commands are:
//    
//    data  -- Datastore volume and data set configuration.
//    versions -- Datastore versions DAG including UUIDs for each node.
//    load  -- Load (# of pending block requests) on block handlers for each data set.
//    cache -- returns LRU cache status
//    
func apiHandler(w http.ResponseWriter, r *http.Request) {
	// Break URL request into arguments
	const lenPath = len(RestApiPath)
	url := r.URL.Path[lenPath:]
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		badRequest(w, r, "Poorly formed request")
		return
	}

	// Handle the requests
	switch parts[0] {
	case "cache":
		fmt.Fprintf(w, "<p>TODO -- return LRU Cache statistics</p>\n")
	case "data":
		handleDataRequest(w, r)
	case "versions":
		jsonStr, err := runningService.VersionsJSON()
		if err != nil {
			badRequest(w, r, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, jsonStr)
	case "load":
		jsonStr, err := datastore.BlockLoadJSON()
		if err != nil {
			badRequest(w, r, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, jsonStr)
	default:
		// Pass type-specific requests to the type service
		dataSetName := datastore.DataSetString(parts[0])
		typeService, err := runningService.DataSetService(dataSetName)
		if err != nil {
			badRequest(w, r, fmt.Sprintf("Could not find data set '%s' in datastore [%s]",
				dataSetName, err.Error()))
			return
		}
		typeService.DoHTTP(w, r, runningService.Service, RestApiPath)
	}
}
