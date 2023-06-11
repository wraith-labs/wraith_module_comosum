package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/cmd/pc3/lib"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
	"github.com/traefik/yaegi/stdlib/unsafe"
)

func CmdX(ctx lib.CommandContext, arg string) (response string, errResponse error) {
	defer func() {
		if p := recover(); p != nil {
			errResponse = fmt.Errorf("command panicked: %e", p)
		}
	}()

	i := interp.New(interp.Options{
		Unrestricted: true,
	})

	stdlib.Symbols["wmp/wmp"] = map[string]reflect.Value{
		"CommandContext": reflect.ValueOf((*lib.CommandContext)(nil)),
		"Config":         reflect.ValueOf((*lib.Config)(nil)),
	}
	i.Use(stdlib.Symbols)
	i.Use(unsafe.Symbols)

	_, err := i.Eval(arg)
	if err != nil {
		return "", fmt.Errorf("could not evaluate due to error: %e", err)
	}

	result, err := i.Eval("main.X")
	if err != nil {
		return "", fmt.Errorf("could not find `main.X`: %e", err)
	}

	if !result.IsValid() {
		// The program didn't return anything for us to run, so assume everything
		// is done and notify the user.
		return "program did not return anything", nil
	}

	communicator, ok := result.Interface().(func(lib.CommandContext) (string, error))
	if !ok {
		return "", fmt.Errorf("returned function was of incorrect type (%T)", result.Interface())
	}

	return communicator(ctx)
}

func CmdL(ctx lib.CommandContext, arg string) (string, error) {
	page, err := strconv.Atoi(arg)
	if err != nil {
		return "", fmt.Errorf("could not parse argument: %e", err)
	}
	clients, err := ctx.State.ClientGetPage(page*10, 10)
	if err != nil {
		return "", fmt.Errorf("could not get page from database: %e", err)
	}
	clientListString, _ := json.Marshal(clients)
	return string(clientListString), nil
}

func CmdH(ctx lib.CommandContext, arg string) (string, error) {
	switch strings.ToLower(arg) {
	case "":
		return "", nil
	default:
		return "", fmt.Errorf("no help found for keyword `%s`", arg)
	}
}

func ExecCmd(ctx lib.CommandContext, command string) (string, error) {
	keyword, arg, _ := strings.Cut(command, " ")
	switch strings.ToLower(keyword) {
	case "x":
		return CmdX(ctx, arg)
	case "l":
		return CmdL(ctx, arg)
	case "h":
		return CmdH(ctx, arg)
	}

	return fmt.Sprintf("keyword `%s` not found, try `h` for help", keyword), nil
}
