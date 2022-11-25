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
