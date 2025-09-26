package main

import (
	"flag"
	"fmt"

	"github.com/lmriccardo/synchme/internal/client"
)

func main() {
	fmt.Println("Starting app ...")

	default_file := "./config/client_config.toml"
	conf_file_path := flag.String("conf", default_file, "Client configuration file")
	flag.Parse() // Parse command-line flags

	client.Run(*conf_file_path)
}
