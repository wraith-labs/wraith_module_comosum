package snippets

import (
	"fmt"
	"strings"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/cmd/pc3/lib"
)

func snippetInfo(ctx lib.CommandContext, arg string) (string, error) {
	target, _, _ := strings.Cut(arg, " ")
	_, err := sendRRToClientAwaitResponse(ctx, target, []byte("package main\nfunc main() {println(1)}")) // TODO: Actually make the packet.
	if err != nil {
		return "", fmt.Errorf("failed to execute info snippet for client `%s` due to error: %s", target, err.Error())
	}

	// TODO: Parse response and send the screenshot.
	return "", nil
}
