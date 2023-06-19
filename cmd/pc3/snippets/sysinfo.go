package snippets

import (
	"context"
	"fmt"
	"net/http"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/cmd/pc3/lib"
	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/proto"
)

func snippetSysinfo(ctx lib.CommandContext, arg string) (string, error) {
	data, err := proto.Marshal(&proto.PacketRR{}, ctx.OwnPrivKey)
	if err != nil {
		return "", fmt.Errorf("could not marshal packet data: %e", err)
	}

	(*ctx.Radio).Send(context.Background(), proto.Packet{
		Peer:   client.Address,
		Method: http.MethodPost,
		Route:  proto.ROUTE_RR,
		Data:   data,
	})
}
