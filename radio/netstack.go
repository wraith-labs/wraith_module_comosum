// Adapted (stolen) from: https://github.com/yggdrasil-network/yggdrasil-go/pull/1109

package radio

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"

	"github.com/yggdrasil-network/yggdrasil-go/src/core"
	"github.com/yggdrasil-network/yggdrasil-go/src/ipv6rwc"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
)

type YggdrasilNIC struct {
	stack      *YggdrasilNetstack
	ipv6rwc    *ipv6rwc.ReadWriteCloser
	dispatcher stack.NetworkDispatcher
	readBuf    []byte
	writeBuf   []byte
}

func (s *YggdrasilNetstack) NewYggdrasilNIC(ygg *core.Core) tcpip.Error {
	rwc := ipv6rwc.NewReadWriteCloser(ygg)
	mtu := rwc.MTU()
	nic := &YggdrasilNIC{
		ipv6rwc:  rwc,
		readBuf:  make([]byte, mtu),
		writeBuf: make([]byte, mtu),
	}
	if err := s.stack.CreateNIC(1, nic); err != nil {
		return err
	}
	go func() {
		var rx int
		var err error
		for {
			rx, err = nic.ipv6rwc.Read(nic.readBuf)
			if err != nil {
				log.Println(err)
				break
			}
			pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{
				Payload: buffer.MakeWithData(nic.readBuf[:rx]),
			})
			nic.dispatcher.DeliverNetworkPacket(ipv6.ProtocolNumber, pkb)
		}
	}()
	_, snet, err := net.ParseCIDR("0200::/7")
	if err != nil {
		return &tcpip.ErrBadAddress{}
	}
	subnet, err := tcpip.NewSubnet(
		tcpip.AddrFromSlice(snet.IP.To16()),
		tcpip.MaskFrom(string(snet.Mask)),
	)
	if err != nil {
		return &tcpip.ErrBadAddress{}
	}
	s.stack.AddRoute(tcpip.Route{
		Destination: subnet,
		NIC:         1,
	})
	if s.stack.HandleLocal() {
		ip := ygg.Address()
		if err := s.stack.AddProtocolAddress(
			1,
			tcpip.ProtocolAddress{
				Protocol:          ipv6.ProtocolNumber,
				AddressWithPrefix: tcpip.AddrFromSlice(ip.To16()).WithPrefix(),
			},
			stack.AddressProperties{},
		); err != nil {
			return err
		}
	}
	return nil
}

func (e *YggdrasilNIC) Attach(dispatcher stack.NetworkDispatcher) { e.dispatcher = dispatcher }

func (e *YggdrasilNIC) IsAttached() bool { return e.dispatcher != nil }

func (e *YggdrasilNIC) MTU() uint32 { return uint32(e.ipv6rwc.MTU()) }

func (*YggdrasilNIC) Capabilities() stack.LinkEndpointCapabilities { return stack.CapabilityNone }

func (*YggdrasilNIC) MaxHeaderLength() uint16 { return 40 }

func (*YggdrasilNIC) LinkAddress() tcpip.LinkAddress { return "" }

func (*YggdrasilNIC) Wait() {}

func (e *YggdrasilNIC) WritePackets(
	list stack.PacketBufferList,
) (int, tcpip.Error) {
	var i int = 0
	for i, pkt := range list.AsSlice() {
		vv := pkt.ToView()
		n, err := vv.Read(e.writeBuf)
		if err != nil {
			log.Println(err)
			return i - 1, &tcpip.ErrAborted{}
		}
		_, err = e.ipv6rwc.Write(e.writeBuf[:n])
		if err != nil {
			log.Println(err)
			return i - 1, &tcpip.ErrAborted{}
		}
	}

	return i, nil
}

func (e *YggdrasilNIC) WriteRawPacket(*stack.PacketBuffer) tcpip.Error {
	panic("not implemented")
}

func (*YggdrasilNIC) ARPHardwareType() header.ARPHardwareType {
	return header.ARPHardwareNone
}

func (e *YggdrasilNIC) AddHeader(*stack.PacketBuffer) {
}

func (e *YggdrasilNIC) ParseHeader(*stack.PacketBuffer) bool {
	return true
}

func (e *YggdrasilNIC) Close() error {
	e.stack.stack.RemoveNIC(1)
	e.dispatcher = nil
	return nil
}

////////////////////////

type YggdrasilNetstack struct {
	stack *stack.Stack
}

func CreateYggdrasilNetstack(ygg *Node) (*YggdrasilNetstack, error) {
	s := &YggdrasilNetstack{
		stack: stack.New(stack.Options{
			NetworkProtocols:   []stack.NetworkProtocolFactory{ipv6.NewProtocol},
			TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, icmp.NewProtocol6},
			HandleLocal:        true,
		}),
	}
	if s.stack.HandleLocal() {
		s.stack.AllowICMPMessage()
	} else if err := s.stack.SetForwardingDefaultAndAllNICs(ipv6.ProtocolNumber, true); err != nil {
		panic(err)
	}
	if err := s.NewYggdrasilNIC(ygg.core); err != nil {
		return nil, fmt.Errorf("s.NewYggdrasilNIC: %s", err.String())
	}
	return s, nil
}

func convertToFullAddr(ip net.IP, port int) (tcpip.FullAddress, tcpip.NetworkProtocolNumber, error) {
	addr := tcpip.Address{}
	ip16 := ip.To16()
	if ip16 != nil {
		addr = tcpip.AddrFromSlice(ip16)
	}
	return tcpip.FullAddress{
		NIC:  1,
		Addr: addr,
		Port: uint16(port),
	}, ipv6.ProtocolNumber, nil
}

func convertToFullAddrFromString(endpoint string) (tcpip.FullAddress, tcpip.NetworkProtocolNumber, error) {
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return tcpip.FullAddress{}, 0, fmt.Errorf("net.SplitHostPort: %w", err)
	}
	pn := 80
	if port != "" {
		if pn, err = strconv.Atoi(port); err != nil {
			return tcpip.FullAddress{}, 0, fmt.Errorf("strconv.Atoi: %w", err)
		}
	}
	return convertToFullAddr(net.ParseIP(host), pn)
}

func (s *YggdrasilNetstack) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	fa, pn, err := convertToFullAddrFromString(address)
	if err != nil {
		return nil, fmt.Errorf("convertToFullAddrFromString: %w", err)
	}
	switch network {
	case "tcp", "tcp6":
		return gonet.DialContextTCP(ctx, s.stack, fa, pn)
	case "udp", "udp6":
		conn, err := gonet.DialUDP(s.stack, nil, &fa, pn)
		if err != nil {
			return nil, fmt.Errorf("gonet.DialUDP: %w", err)
		}
		return conn, nil
	default:
		return nil, fmt.Errorf("not supported")
	}
}

func (s *YggdrasilNetstack) DialTCP(addr *net.TCPAddr) (net.Conn, error) {
	fa, pn, _ := convertToFullAddr(addr.IP, addr.Port)
	return gonet.DialTCP(s.stack, fa, pn)
}

func (s *YggdrasilNetstack) DialUDP(addr *net.UDPAddr) (net.PacketConn, error) {
	fa, pn, _ := convertToFullAddr(addr.IP, addr.Port)
	return gonet.DialUDP(s.stack, nil, &fa, pn)
}

func (s *YggdrasilNetstack) ListenTCP(addr *net.TCPAddr) (net.Listener, error) {
	fa, pn, _ := convertToFullAddr(addr.IP, addr.Port)
	return gonet.ListenTCP(s.stack, fa, pn)
}

func (s *YggdrasilNetstack) ListenUDP(addr *net.UDPAddr) (net.PacketConn, error) {
	fa, pn, _ := convertToFullAddr(addr.IP, addr.Port)
	return gonet.DialUDP(s.stack, &fa, nil, pn)
}
