package main

import (
	"flag"

	"github.com/lmriccardo/synchme/internal/client"
	"github.com/lmriccardo/synchme/internal/utils"
)

func main() {
	utils.INFO("Starting SynchMe Client ...")

	default_file := "./config/client_config.toml"
	conf_file_path := flag.String("conf", default_file, "Client configuration file")
	flag.Parse() // Parse command-line flags

	client.Run(*conf_file_path)
}
