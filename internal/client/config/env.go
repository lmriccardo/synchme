package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
	"github.com/lmriccardo/synchme/internal/client/consts"
	"github.com/lmriccardo/synchme/internal/utils"
)

// StringExpandEnv replaces environment variable references in a string with their values.
// It supports this environment variable syntax $<VAR>. If the detected environment variable
// does not exists in the current environment, that slice of the string remains unchanged and
// warning is raised, otherwise the value of the env variable is replaced.
func StringExpandEnv(value string) string {
	// This regex matches env variables specified either with $VAR or ${VAR}
	re := regexp.MustCompile(`\$<([a-zA-Z0-9_]+)>`)

	// To check if an environment variable exists or not in the current system
	// and also to expand the input value we check for each capture group
	// if the value in that group exists as a variable. If not, raise a warning
	// otherwise replace the actual variable value
	expanded := re.ReplaceAllStringFunc(value,
		func(match string) string {
			// First we need to determine the variable name from the captured
			// groups. If it is 1 then it is $VAR, otherwise it is ${VAR}
			matches := re.FindStringSubmatch(match)

			// There might be no match. If so, returns the input string
			if len(matches) <= 1 {
				return match
			}

			// Once we have the variable name, check if it exists.
			// If it does not, then prints a warning and returns the input string
			env_value, ok := os.LookupEnv(matches[1])
			if !ok {
				utils.WARN("EnvVariable ", matches[1], " does not exists in this environment")
				return match
			}

			return env_value
		},
	)

	return expanded
}

// PrintEnvironment prints all custom environment variables
func PrintEnvironment() {
	vars := []string{consts.SYNCHME_FOLDER,
		consts.SYNCHME_API_KEY,
		consts.SYNCHME_CONFIG}

	_print := func(k string) { fmt.Printf("%v=%v\n", k, os.Getenv(k)) }
	fmt.Println(strings.Repeat("-", 50))
	for _, var_name := range vars {
		_print(var_name)
	}
	fmt.Println(strings.Repeat("-", 50))
}

// LoadEnvironment optionally load the .env in the current working
// folder if it exists. Otherwise, sets the environment variables
// with some default values. Notice that, in production the .env
// file is located in the .synchme subfolder of the home user path.
func LoadEnvironment() {
	// Check if the .synchme folder exists, if it does not then create it
	home_folder, err := os.UserHomeDir()
	if err != nil {
		utils.FATAL("Unable to retrieve user home folder: ", err)
	}

	sync_folder := filepath.Join(home_folder, consts.SYNCHME_DEFAULT_FOLDER)
	if !utils.Exist(sync_folder) {
		if err := os.Mkdir(sync_folder, fs.ModeType); err != nil {
			utils.FATAL("Unable to create ", sync_folder, ": ", err)
		}
	}

	load_result := true
	synchme_env_file := filepath.Join(sync_folder, consts.ENV_FILE_NAME)

	// Load the .env file from the current working folder if it exists
	// otherwise checks if the .env file is in the synchme folder.
	// If not, load some default values and when the application exit
	// write those values into the .env file.
	if err := godotenv.Overload(); err != nil {
		err := godotenv.Overload(synchme_env_file)
		if err != nil {
			utils.INFO("Any of the .env file in the pwd or .synchme root folder exist")
			load_result = false
		}
	} else {
		// Set the env file to be the one in the current folder
		curr_folder, _ := os.Getwd()
		synchme_env_file = filepath.Join(curr_folder, consts.ENV_FILE_NAME)
	}

	// For facility sets also the synchme root folder as env variable as well
	// as the absolute path to the .env file to write the content when exiting
	SetEnv(consts.SYNCHME_ROOT_FOLDER, sync_folder)
	SetEnv(consts.SYNCHME_ENV_FILE, synchme_env_file)

	// If none of the .env file exist then we need to set default values
	if !load_result {
		utils.INFO("Loading default environment")

		// Set default values into the environment variables
		default_config_file := filepath.Join(sync_folder, consts.SYNCHME_DEFAULT_CONFIG)

		SetEnv(consts.SYNCHME_FOLDER, sync_folder)
		SetEnv(consts.SYNCHME_API_KEY, consts.SYNCHME_DEFAULT_API_KEY)
		SetEnv(consts.SYNCHME_CONFIG, default_config_file)
	} else {
		// If the synchme folder is empty, set it to the default value
		if len(os.Getenv(consts.SYNCHME_FOLDER)) == 0 {
			SetEnv(consts.SYNCHME_FOLDER, sync_folder)
		}

		// At this point, some variables might use existing environment variable.
		// Therefore, we need to expand them using the custom expand function
		SetEnv(consts.SYNCHME_FOLDER, StringExpandEnv(os.Getenv(consts.SYNCHME_FOLDER)))
		SetEnv(consts.SYNCHME_CONFIG, StringExpandEnv(os.Getenv(consts.SYNCHME_CONFIG)))
	}

	utils.INFO("Loading environment from ", synchme_env_file)
	PrintEnvironment()
}

// SetEnv tries to set an environment variable and fatal if any error
func SetEnv(key string, value string) {
	if err := os.Setenv(key, value); err != nil {
		utils.FATAL("Unable to set environment variable: ", err)
	}
}

// WriteEnvironment writes the environments into the .env file
func WriteEnvironment() {
	env_file := os.Getenv(consts.SYNCHME_ENV_FILE)

	// Create the content to be written
	content := consts.ENV_CONTENT_HEADER
	content += fmt.Sprintf("%v=%v\n", consts.SYNCHME_FOLDER, os.Getenv(consts.SYNCHME_FOLDER))
	content += fmt.Sprintf("%v=%v\n", consts.SYNCHME_API_KEY, os.Getenv(consts.SYNCHME_API_KEY))
	content += fmt.Sprintf("%v=%v\n", consts.SYNCHME_CONFIG, os.Getenv(consts.SYNCHME_CONFIG))

	// Write the entire environment into the .env file
	if err := os.WriteFile(env_file, []byte(content), os.ModePerm); err != nil {
		utils.ERROR("Unable to write environment: ", err)
		return
	}

	utils.INFO("Written environment into: ", env_file)
}
