package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"runtime"
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

type clientsRequest struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
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

func handleClients(r *http.Request, w http.ResponseWriter, s *state) {
	// Pull necessary information out of the request.
	// Get the data from the request body.
	reqbody, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Parse the request body.
	reqdata := clientsRequest{}
	err = json.Unmarshal(reqbody, &reqdata)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Collect necessary information.
	clients, totalClients := s.GetClients(reqdata.Offset, reqdata.Limit)

	// Build response data.
	data, err := json.Marshal(map[string]any{
		"clients": clients,
		"total":   totalClients,
	})
	if err != nil {
		panic(fmt.Sprintf("error while generating `clients` API response: %v", err))
	}

	// Send!
	w.Write(data)
}

func handleAbout(w http.ResponseWriter) {
	// Collect necessary information.
	buildinfo, _ := debug.ReadBuildInfo()

	currentUser, _ := user.Current()
	binaryPath, _ := os.Executable()
	systemInfo := map[string]string{
		"os":              runtime.GOOS,
		"arch":            runtime.GOARCH,
		"currentTime":     time.Now().String(),
		"binaryPath":      binaryPath,
		"runningUserName": currentUser.Username,
		"runningUserId":   currentUser.Uid,
	}

	// Build response data.
	data, err := json.Marshal(map[string]any{
		"build":  buildinfo,
		"system": systemInfo,
	})
	if err != nil {
		panic(fmt.Sprintf("error while generating `about` API response: %v", err))
	}

	// Send!
	w.Write(data)
}
