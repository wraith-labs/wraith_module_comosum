package snippets

import "dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/cmd/pc3/lib"

var Snippets map[string]func(ctx lib.CommandContext, arg string)

func init() {
	Snippets = map[string]func(ctx lib.CommandContext, arg string){
		"sysinfo":    snippetSysinfo,
		"screenshot": snippetScreenshot,
	}
}
