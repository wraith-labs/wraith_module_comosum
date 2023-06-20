package snippets

import (
	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/cmd/pc3/lib"
)

func snippetCode(ctx lib.CommandContext, arg string) (string, error) {
	/*target, _, _ := strings.Cut(arg, " ")
	response, err := sendRRToClientAwaitResponse(ctx, target, &proto.PacketRR{}) // TODO: Actually make the packet.
	if err != nil {
		return "", fmt.Errorf("failed to execute code snippet for client `%s` due to error: %s", target, err.Error())
	}*/

	// TODO: Parse response and send the screenshot.
	return "", nil
}
