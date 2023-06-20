package snippets

import (
	"fmt"
	"strings"
	"sync/atomic"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/cmd/pc3/lib"
)

func snippetSendall(ctx lib.CommandContext, arg string) (string, error) {
	snippetName, snippetArg, _ := strings.Cut(arg, " ")

	snippet, exists := Snippets[snippetName]
	if !exists {
		return "", fmt.Errorf("snippet `%s` not found", snippetName)
	}

	clients, err := ctx.State.ClientGetAll()
	if err != nil {
		return "", fmt.Errorf("could not get a list of all clients from db: %s", err.Error())
	}

	errCount := new(uint64)
	for _, client := range clients {
		go func(client lib.Client) {
			_, err := snippet(ctx, fmt.Sprintf("%s %s", client.ID, snippetArg))
			if err != nil {
				atomic.AddUint64(errCount, 1)
			}
		}(client)
	}

	if *errCount > 0 {
		err = fmt.Errorf("%d sends errored", errCount)
	}

	return fmt.Sprintf("sent to %d clients", len(clients)), err
}
