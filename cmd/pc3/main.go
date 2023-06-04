package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
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
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
)

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

	// Start pinecone.
	go pr.Start()

	// Connect to Matrix homeserver.
	client, err := mautrix.NewClient(c.homeserver, "", "")
	if err != nil {
		panic(err)
	}

	syncer := client.Syncer.(*mautrix.DefaultSyncer)
	syncer.OnEventType(event.EventMessage, func(source mautrix.EventSource, evt *event.Event) {
		//
	})
	syncer.OnEventType(event.StateMember, func(source mautrix.EventSource, evt *event.Event) {
		if evt.GetStateKey() == client.UserID.String() && evt.Content.AsMember().Membership == event.MembershipInvite {
			_, err := client.JoinRoomByID(evt.RoomID)
			if err == nil {
				//
			} else {
			}
		}
	})

	cryptoHelper, err := cryptohelper.NewCryptoHelper(client, []byte("meow"), "file::memory:")
	if err != nil {
		panic(err)
	}

	// You can also store the user/device IDs and access token and put them in the client beforehand instead of using LoginAs.
	//client.UserID = "..."
	//client.DeviceID = "..."
	//client.AccessToken = "..."
	// You don't need to set a device ID in LoginAs because the crypto helper will set it for you if necessary.
	cryptoHelper.LoginAs = &mautrix.ReqLogin{
		Type:       mautrix.AuthTypePassword,
		Identifier: mautrix.UserIdentifier{Type: mautrix.IdentifierTypeUser, User: c.username},
		Password:   c.password,
	}
	// If you want to use multiple clients with the same DB, you should set a distinct database account ID for each one.
	//cryptoHelper.DBAccountID = ""
	err = cryptoHelper.Init()
	if err != nil {
		panic(err)
	}
	// Set the client crypto helper in order to automatically encrypt outgoing messages
	client.Crypto = cryptoHelper

	syncCtx, cancelSync := context.WithCancel(context.Background())
	var syncStopWait sync.WaitGroup
	syncStopWait.Add(1)

	go func() {
		err = client.SyncWithContext(syncCtx)
		defer syncStopWait.Done()
		if err != nil && !errors.Is(err, context.Canceled) {
			panic(err)
		}
	}()

	cancelSync()
	syncStopWait.Wait()
	_ = cryptoHelper.Close()
	client.JoinedRooms()

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
