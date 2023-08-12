package snippets

import (
	"fmt"
	"strings"
	"time"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/cmd/pc3/lib"
)

func snippetInfo(ctx lib.CommandContext, arg string) (string, error) {
	target, _, _ := strings.Cut(arg, " ")
	res, err := sendRRToClientAwaitResponse(ctx, target, []byte(`
package main

import (
	"wmp/libwraith"
	"wmp/module"
)

func Y(m *module.ModulePinecomms, w *libwraith.Wraith) []byte {
	return []byte("testing!")
}
`), time.Second*2)
	if err != nil {
		return "", fmt.Errorf("failed to execute `info` snippet for client `%s` due to error: %s", target, err.Error())
	}

	return fmt.Sprintf("client response from `%s`: %s", target, string(res.Payload)), nil
}
