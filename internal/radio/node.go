// Adapted (stolen) from: https://github.com/yggdrasil-network/yggdrasil-go/pull/1109

package radio

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"regexp"

	"github.com/gologme/log"

	"github.com/yggdrasil-network/yggdrasil-go/src/admin"
	"github.com/yggdrasil-network/yggdrasil-go/src/config"
	"github.com/yggdrasil-network/yggdrasil-go/src/core"
	"github.com/yggdrasil-network/yggdrasil-go/src/multicast"
)

type Node struct {
	core      *core.Core
	ctx       context.Context
	cancel    context.CancelFunc
	logger    *log.Logger
	config    *config.NodeConfig
	multicast *multicast.Multicast
	admin     *admin.AdminSocket
}

func NewNode(logger *log.Logger) *Node {
	ctx, cancel := context.WithCancel(context.Background())
	return &Node{
		ctx:       ctx,
		cancel:    cancel,
		logger:    logger,
		multicast: &multicast.Multicast{},
		admin:     &admin.AdminSocket{},
	}
}

func (n *Node) Close() {
	n.cancel()
	_ = n.admin.Stop()
	_ = n.multicast.Stop()
	_ = n.core.Close()
}

func (n *Node) Done() <-chan struct{} {
	return n.ctx.Done()
}

func (n *Node) Address() (net.IP, net.IPNet) {
	address, subnet := n.core.Address(), n.core.Subnet()
	return address, subnet
}

func (n *Node) Admin() *admin.AdminSocket {
	return n.admin
}

func (n *Node) GenerateConfig(listen []string, peers []string, debugSocket string) {
	// Get the defaults for the platform.
	defaults := config.GetDefaults()

	// Create a node configuration and populate it.
	cfg := new(config.NodeConfig)
	cfg.NewPrivateKey()
	cfg.Listen = listen
	cfg.AdminListen = debugSocket
	cfg.Peers = peers
	cfg.InterfacePeers = map[string][]string{}
	cfg.AllowedPublicKeys = []string{}
	cfg.MulticastInterfaces = defaults.DefaultMulticastInterfaces
	cfg.IfName = "none"
	cfg.IfMTU = defaults.DefaultIfMTU
	cfg.NodeInfoPrivacy = true
	if err := cfg.GenerateSelfSignedCertificate(); err != nil {
		panic(err)
	}

	n.config = cfg
}

func (n *Node) Config() config.NodeConfig {
	return *n.config
}

func (n *Node) Run() error {
	var err error = nil
	// Have we got a working configuration? Stop if we don't.
	if n.config == nil {
		return fmt.Errorf("no configuration supplied")
	}

	// Setup the Yggdrasil node.
	{
		options := []core.SetupOption{
			core.NodeInfo(n.config.NodeInfo),
			core.NodeInfoPrivacy(n.config.NodeInfoPrivacy),
		}
		for _, addr := range n.config.Listen {
			options = append(options, core.ListenAddress(addr))
		}
		for _, peer := range n.config.Peers {
			options = append(options, core.Peer{URI: peer})
		}
		for intf, peers := range n.config.InterfacePeers {
			for _, peer := range peers {
				options = append(options, core.Peer{URI: peer, SourceInterface: intf})
			}
		}
		for _, allowed := range n.config.AllowedPublicKeys {
			k, err := hex.DecodeString(allowed)
			if err != nil {
				return err
			}
			options = append(options, core.AllowedPublicKey(k[:]))
		}
		if n.core, err = core.New(n.config.Certificate, n.logger, options...); err != nil {
			return err
		}
	}

	// Setup the admin socket.
	{
		options := []admin.SetupOption{
			admin.ListenAddress(n.config.AdminListen),
		}
		if n.config.LogLookups {
			options = append(options, admin.LogLookups{})
		}
		if n.admin, err = admin.New(n.core, n.logger, options...); err != nil {
			return err
		}
		if n.admin != nil {
			n.admin.SetupAdminHandlers()
		}
	}

	// Setup the multicast module.
	{
		options := []multicast.SetupOption{}
		for _, intf := range n.config.MulticastInterfaces {
			options = append(options, multicast.MulticastInterface{
				Regex:    regexp.MustCompile(intf.Regex),
				Beacon:   intf.Beacon,
				Listen:   intf.Listen,
				Port:     intf.Port,
				Priority: uint8(intf.Priority),
				Password: intf.Password,
			})
		}
		if n.multicast, err = multicast.New(n.core, n.logger, options...); err != nil {
			return err
		}
		if n.admin != nil && n.multicast != nil {
			n.multicast.SetupAdminHandlers(n.admin)
		}
	}

	return nil
}
