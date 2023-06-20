package snippets

import (
	"fmt"
	"strings"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/cmd/pc3/lib"
)

func snippetSendto(ctx lib.CommandContext, arg string) (string, error) {
	targetsArg, otherArgs, _ := strings.Cut(arg, " ")
	snippetName, otherArgs, _ := strings.Cut(otherArgs, " ")
	targets := strings.Split(targetsArg, ",")

	snippet, exists := Snippets[snippetName]
	if !exists {
		return "", fmt.Errorf("snippet `%s` not found", snippetName)
	}

	clients := make([]lib.Client, len(targets))
	for _, target := range targets {
		client, err := ctx.State.ClientGet(target)
		if err != nil {
			return "", fmt.Errorf("could not get client `%s` from the database: %s", target, err.Error())
		}
		clients = append(clients, client)
	}

	errCount := 0
	for _, client := range clients {
		_, err := snippet(ctx, fmt.Sprintf("%s %s", client.ID, otherArgs))
		if err != nil {
			errCount++
		}
	}

	var err error
	if errCount > 0 {
		err = fmt.Errorf("%d sends errored", errCount)
	}

	return fmt.Sprintf("sent to %d clients", len(clients)), err
}
