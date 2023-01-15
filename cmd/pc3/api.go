package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"
)

type authRequest struct {
	Token string `json:"token"`
	Time  int64  `json:"time"`
}

type authSuccessResponse struct {
	Token  string     `json:"token"`
	Expiry time.Time  `json:"expiry"`
	Access authStatus `json:"access"`
}

type sendRequest struct {
	Target  string `json:"target"`
	Payload struct {
		Read    []string               `json:"read"`
		Write   map[string]interface{} `json:"write"`
		ListMem bool                   `json:"listMem"`
	} `json:"payload"`
	Conditions struct{} `json:"conditions"`
}

func handleAbout(w http.ResponseWriter) {
	// Collect necessary information.
	buildinfo, _ := debug.ReadBuildInfo()

	// Build response data.
	data, err := json.Marshal(map[string]any{
		"build": buildinfo,
	})
	if err != nil {
		panic(fmt.Sprintf("error while generating `about` API response: %v", err))
	}

	// Send!
	w.Write(data)
}
