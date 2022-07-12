package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"git.0x1a8510f2.space/wraith-labs/wraith-module-pinecomms/internal/pineconemanager"
)

const (
	DEFAULT_PANEL_LISTEN_ADDR  = "127.0.0.1:48080"
	DEFAULT_PANEL_ACCESS_TOKEN = "wraith!"
)

func main() {
	// Create a struct to hold any config values
	var c Config

	//
	// Define and parse command-line flags
	//

	c.panelListenAddr = flag.String("panelAddr", DEFAULT_PANEL_LISTEN_ADDR, "the address on which the control panel should listen for connections")
	c.panelAccessToken = flag.String("panelToken", DEFAULT_PANEL_ACCESS_TOKEN, "the token needed to access the control panel")
	c.pineconeId = flag.String("pineconeId", "", "private key to use as identity on the pinecone network")
	c.logPinecone = flag.Bool("logPinecone", false, "whether to print pinecone's internal logs to stdout")
	c.pineconeInboundTcpAddr = flag.String("inboundTcpAddr", "", "the address to listen for inbound pinecone connections on")
	c.pineconeInboundWebAddr = flag.String("inboundWebAddr", "", "the address to listen for inbound HTTP connections on (allows pinecone over websocket and pinecone debug endpoint if enabled)")
	c.pineconeDebugEndpoint = flag.String("pineconeDebugEndpoint", "", "the HTTP path of the pinecone debug endpoint on the pinecone webserver (omit to disable)")
	c.pineconeUseMulticast = flag.Bool("useMulticast", false, "whether to use multicast to discover pinecone peers on the local network")
	c.pineconeStaticPeers = flag.String("staticPeers", "", "comma-delimeted list of static peers to connect to")

	flag.Parse()

	//
	// Validate the config
	//

	if *c.pineconeId == "" {
		fmt.Println("no pineconeId was specified; cannot continue")
		os.Exit(1)
	}
	if *c.pineconeInboundTcpAddr == "" && *c.pineconeInboundWebAddr == "" && !*c.pineconeUseMulticast && *c.pineconeStaticPeers == "" {
		fmt.Println("no way for peers to connect was specified; cannot continue")
		os.Exit(1)
	}
	pineconeIdBytes, err := hex.DecodeString(*c.pineconeId)
	if err != nil {
		fmt.Println("provided pineconeId was not a hex-encoded string; cannot continue")
		os.Exit(1)
	}
	pineconeId := ed25519.PrivateKey(pineconeIdBytes)

	// Get a struct for managing pinecone connections
	pm := pineconemanager.GetInstance()

	//
	// Configure pinecone manager
	//

	pm.SetPineconeIdentity(pineconeId)
	if *c.logPinecone {
		pm.SetLogger(log.Default())
	}
	pm.SetInboundAddr(*c.pineconeInboundTcpAddr)
	pm.SetWebserverAddr(*c.pineconeInboundWebAddr)
	pm.SetWebserverDebugPath(*c.pineconeDebugEndpoint)
	pm.SetUseMulticast(*c.pineconeUseMulticast)
	if *c.pineconeStaticPeers != "" {
		pm.SetStaticPeers(strings.Split(*c.pineconeStaticPeers, ","))
	}

	//
	// Configure signal handler for clean exit
	//

	sigchan := make(chan os.Signal, 2)
	signal.Notify(sigchan, syscall.SIGTERM, syscall.SIGINT)

	//
	// Main body
	//

	go pm.Start()

	//
	// On exit
	//

	<-sigchan
	fmt.Println("exit requested; exiting gracefully")

	go func() {
		<-sigchan
		fmt.Println("exit re-requested; forcing")
		os.Exit(1)
	}()

	pm.Stop()

	os.Exit(0)
}
