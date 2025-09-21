package main

import (
	"fmt"

	"github.com/lmriccardo/synchme/internal/server"
)

func main() {
	fmt.Println("Starting app ...")
	server.Run()
}
