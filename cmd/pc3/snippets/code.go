package snippets

import (
	"fmt"
	"strings"
	"time"

	"dev.l1qu1d.net/wraith-labs/wraith_module_comosum/cmd/pc3/lib"
)

func snippetCode(ctx lib.CommandContext, arg string) (string, error) {
	target, code, _ := strings.Cut(arg, " ")
	res, err := sendReqAwaitResponse(ctx, target, []byte(code), time.Minute)
	if err != nil {
		return "", fmt.Errorf("failed to execute `code` snippet for client `%s` due to error: %s", target, err.Error())
	}

	return fmt.Sprintf("client response from `%s`: %s", target, string(res.Payload)), nil
}
