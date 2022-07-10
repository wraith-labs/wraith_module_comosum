package main

import (
	"fmt"

	"git.0x1a8510f2.space/wraith-labs/wraith-module-pinecomms/internal/pineconemanager"
)

func main() {
	pm := pineconemanager.GetInstance()

	fmt.Println(pm.GetPineconeIdentity())

	pm.Start()
}
