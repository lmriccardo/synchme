package main

import (
	"log"

	"github.com/lmriccardo/synchme/internal/server"
)

func main() {
	log.Println("Starting SynchMe Server ...")
	server.Run()
}
