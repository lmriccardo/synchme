package main

import (
	"github.com/lmriccardo/synchme/internal/client"
	"github.com/lmriccardo/synchme/internal/utils"
)

func main() {
	utils.INFO("Starting SynchMe Client ...")
	client.Run()
}
