package consts

import "time"

const (
	// Environment Variables Names
	SYNCHME_FOLDER              string = "SYNCHME_FOLDER"
	SYNCHME_API_KEY             string = "SYNCHME_API_KEY"
	SYNCHME_CONFIG              string = "SYNCHME_CONFIG"
	SYNCHME_ROOT_FOLDER         string = "SYNCHME_ROOT_FOLDER"
	SYNCHME_ENV_FILE            string = "SYNCHME_ENV_FILE"
	SYNCHME_DEFAULT_CONFIG_PATH string = "SYNCHME_DEFAULT_CONFIG_PATH"

	SYNCHME_DEFAULT_FOLDER  string        = ".synchme"
	SYNCHME_DEFAULT_API_KEY string        = ""
	SYNCHME_DEFAULT_CONFIG  string        = "config.toml"
	ENV_FILE_NAME           string        = ".env"
	CHUNK_SIZE              uint32        = 64 * 1024             // 65536
	THRESHOLD               time.Duration = 50 * time.Millisecond // 50 millisec

	ENV_CONTENT_HEADER string = "# .env file\n" +
		"# Secrets and configuration for the local environment\n\n" +
		"# Existing environment variables can be used to construct new ones.\n" +
		"# They can appear in the format: $<VAR> (other formats are not recognised).\n" +
		"# On the other hand, created variables can be referenced using $VAR or ${VAR}.\n\n" +
		"# SYNCHME_FOLDER  = The path to the root folder of the synchme client-side application\n" +
		"# SYNCHME_API_KEY = The API token obtained from the server (used for authorization)\n" +
		"# SYNCHME_CONFIG  = There the configuration file resides\n\n"
)
