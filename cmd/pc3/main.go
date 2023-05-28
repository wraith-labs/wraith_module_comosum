package main

import (
	"context"
	"crypto/ed25519"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/proto"
	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/radio"
)

//go:embed ui/dist/*
var ui embed.FS

func main() {
	//
	// Create a struct to hold any config values.
	//

	c := Config{}
	c.Setup()

	//
	// Validate the config.
	//

	if c.pineconeId == "" {
		fmt.Println("no pineconeId was specified; cannot continue")
		os.Exit(1)
	}
	if c.pineconeInboundTcpAddr == "" && c.pineconeInboundWebAddr == "" && !c.pineconeUseMulticast && c.pineconeStaticPeers == "" {
		fmt.Println("no way for peers to connect was specified; cannot continue")
		os.Exit(1)
	}
	pineconeIdBytes, err := hex.DecodeString(c.pineconeId)
	if err != nil {
		fmt.Println("provided pineconeId was not a hex-encoded string; cannot continue")
		os.Exit(1)
	}
	pineconeId := ed25519.PrivateKey(pineconeIdBytes)

	// Get a struct for managing pinecone connections.
	pr := radio.GetInstance()

	//
	// Configure pinecone manager.
	//

	pr.SetPineconeIdentity(pineconeId)
	if c.logPinecone {
		pr.SetLogger(log.Default())
	}
	pr.SetInboundAddr(c.pineconeInboundTcpAddr)
	pr.SetWebserverAddr(c.pineconeInboundWebAddr)
	pr.SetWebserverDebugPath(c.pineconeDebugEndpoint)
	pr.SetUseMulticast(c.pineconeUseMulticast)
	if c.pineconeStaticPeers != "" {
		pr.SetStaticPeers(strings.Split(c.pineconeStaticPeers, ","))
	}

	//
	// Configure signal handler for clean exit.
	//

	sigchan := make(chan os.Signal, 2)
	signal.Notify(sigchan, syscall.SIGTERM, syscall.SIGINT)

	//
	// Main body.
	//

	// Create a state storage struct.
	s := MkState()

	// Use pmanager non-pinecone webserver to host web UI and an API to communicate with it.
	ui, err := fs.Sub(ui, "ui/dist")
	if err != nil {
		panic(err)
	}

	pr.SetWebserverHandlers([]radio.WebserverHandler{
		{
			Path: "/X/",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path := strings.TrimPrefix(r.URL.EscapedPath(), "/X/")

				switch path {
				case "checkauth":
					if !StatusInGroup(AuthStatus(r), AUTH_STATUS_A, AUTH_STATUS_V) {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					w.WriteHeader(http.StatusNoContent)
				case "clients":
					// Require auth.
					if !StatusInGroup(AuthStatus(r), AUTH_STATUS_A, AUTH_STATUS_V) {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}

					handleClients(r, w, s)
				case "about":
					// Require auth.
					if !StatusInGroup(AuthStatus(r), AUTH_STATUS_A, AUTH_STATUS_V) {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}

					handleAbout(w)
				case "send":
					// Require auth as admin.
					if !StatusInGroup(AuthStatus(r), AUTH_STATUS_A) {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}

					// Get the data from the request body.
					reqbody, err := io.ReadAll(r.Body)
					if err != nil {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}

					// Parse the request body.
					reqdata := sendRequest{}
					err = json.Unmarshal(reqbody, &reqdata)
					if err != nil {
						w.WriteHeader(http.StatusBadRequest)
						return
					}

					// Prepare the packet to be sent to client.
					req := s.Request(reqdata.Target, proto.PacketReq{
						Payload: struct {
							Read    []string
							Write   map[string]interface{}
							ListMem bool
						}{
							Read:    reqdata.Payload.Read,
							Write:   reqdata.Payload.Write,
							ListMem: reqdata.Payload.ListMem,
						},
						Conditions: reqdata.Conditions,
					})
					packetData, err := proto.Marshal(&req, pineconeId)
					if err != nil {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}

					// Send the packet to client.
					pr.Send(context.Background(), proto.Packet{
						Peer:   reqdata.Target,
						Method: http.MethodPost,
						Route:  proto.ROUTE_REQUEST,
						Data:   packetData,
					})

					w.WriteHeader(http.StatusNoContent)
				case "auth":
					// Make sure we haven't exceeded the limit for failed logins.
					if c.attemptsUntilLockout.Load() <= 0 {
						w.WriteHeader(http.StatusTeapot)
						return
					}

					// Get the data from the request body.
					reqbody, err := io.ReadAll(r.Body)
					if err != nil {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}

					reqdata := authRequest{}
					err = json.Unmarshal(reqbody, &reqdata)
					if err != nil {
						w.WriteHeader(http.StatusBadRequest)
						return
					}

					// Validate credential.
					outtoken, expiry, status, ok := TradeTokens(c, reqdata)
					if !ok {
						c.attemptsUntilLockout.Add(-1)
						w.WriteHeader(http.StatusUnauthorized)
						return
					}

					// Reset failed attempts counter on successful login.
					c.attemptsUntilLockout.Store(STARTING_ATTEMPTS_UNTIL_LOCKOUT)

					// Create a response.
					response, err := json.Marshal(authSuccessResponse{
						Token:  string(outtoken),
						Expiry: expiry,
						Access: status,
					})

					// Not much we can do.
					if err != nil {
						w.WriteHeader(http.StatusInternalServerError)
					}

					// 200.
					w.Write(response)
				default:
					// If someone makes an API call we don't recognise, we're a teapot.
					w.WriteHeader(http.StatusTeapot)
				}
			}),
		},
		{
			Path:    "/",
			Handler: http.FileServer(http.FS(ui)),
		},
	})

	// Start pinecone.
	go pr.Start()

	// Start receiving messages.
	// Background context is okay because the channel will be closed
	// when the manager exits further down anyway.
	recv := pr.RecvChan(context.Background())

mainloop:
	for {
		select {
		// Exit if requested.
		case <-sigchan:
			break mainloop
		// Clean up state.
		case <-time.After(STATE_CLEANUP_INTERVAL):
			s.Prune()
		// Process incoming packets.
		case packet := <-recv:
			peerPublicKey, err := hex.DecodeString(packet.Peer)
			if err != nil {
				// This shouldn't happen, but if the peer public key is
				// malformed then we have no choice but to ignore the
				// packet.
				continue mainloop
			}

			switch packet.Route {
			case proto.ROUTE_HEARTBEAT:
				packetData := proto.PacketHeartbeat{}
				err = proto.Unmarshal(&packetData, peerPublicKey, packet.Data)
				if err != nil {
					// The packet data is malformed, there is nothing more we
					// can do.
					continue mainloop
				}
				go s.Heartbeat(packet.Peer, packetData)
			case proto.ROUTE_RESPONSE:
				packetData := proto.PacketRes{}
				err = proto.Unmarshal(&packetData, peerPublicKey, packet.Data)
				if err != nil {
					// The packet data is malformed, there is nothing more we
					// can do.
					continue mainloop
				}
				go s.Response(packet.Peer, packetData)
			}
		}
	}

	//
	// On exit.
	//

	fmt.Println("exit requested; exiting gracefully")

	go func() {
		<-sigchan
		fmt.Println("exit re-requested; forcing")
		os.Exit(1)
	}()

	pr.Stop()

	os.Exit(0)
}
