package config

import (
	"net"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/go-playground/validator/v10"
	"github.com/lmriccardo/synchme/internal/client/consts"
	"github.com/lmriccardo/synchme/internal/utils"
)

// Configuration-releated settings
// WatchConf: The current file is automatically watched
// SynchConf: If the conf changes, remote synch is provided
// Restart  : If the conf changes, the service is automatically restarted
type Config struct {
	WatchConf *bool `toml:"watch_conf" validate:"required,boolean"`
	SynchConf *bool `toml:"synch_conf" validate:"required,boolean"`
}

// FileSystemNotification-releated settings
// Paths        : Absolute paths of file/folder to watch
// Recursive    : Perform recursive search on added folders
// BaseTTL      : [seconds] Base TTL of content in the cache
// MaxTTL       : [seconds] Base TTL of content in the cache
// ExpirationInt: [seconds] Interval of time between two expiration checks
type FS_Notification struct {
	Paths         []string `toml:"paths"`
	BaseTTL       int64    `toml:"caching_base_ttl" validate:"required,numeric,min=1"`
	MaxTTL        int64    `toml:"caching_max_ttl" validate:"required,numeric,gtefield=BaseTTL"`
	ExpirationInt int64    `toml:"expiration_interval" validate:"required,numeric,min=1"`
	SynchInterval float64  `toml:"synch_interval" validate:"required"`
	Filters       []string `toml:"filters" validate:"required,fs_op"`
}

// Network-releated settings
// RelayIP         : The IP address of the relay server
// RelayPort       : The IP port of the relay server
// NetworkInterface: Sender network interface
// SourcePort      : Client source port
type Network struct {
	ServerHost        string `toml:"relay_host" validate:"required,ip_or_hostname"`
	ServerPort        int    `toml:"relay_port" validate:"required,min=1,max=65535"`
	HeartbeatInterval int    `toml:"heartbeat_interval" validate:"required,min=1,max=65535"`
}

// Root struct representing the whole TOML
type ClientConf struct {
	Path            string
	Config          Config          `toml:"Config"`
	FS_Notification FS_Notification `toml:"FileSystemNotification"`
	Network         Network         `toml:"Network"`
}

func IpOrHostname(v *validator.Validate) func(fl validator.FieldLevel) bool {
	return func(fl validator.FieldLevel) bool {
		val := fl.Field().String()
		return v.Var(val, "ip") == nil || v.Var(val, "hostname_rfc1123") == nil
	}
}

func IpOrIface(v *validator.Validate) func(fl validator.FieldLevel) bool {
	return func(fl validator.FieldLevel) bool {
		val := fl.Field().String()

		// Check if it's a valid IP
		if net.ParseIP(val) != nil {
			return true
		}

		// Check if it's a valid network interface
		if ifaces, err := net.Interfaces(); err == nil {
			for _, iface := range ifaces {
				if iface.Name == val {
					return true
				}
			}

			return false
		}

		return false
	}
}

func ValidFilters(v *validator.Validate) func(fl validator.FieldLevel) bool {
	return func(fl validator.FieldLevel) bool {
		filters := fl.Field()

		// Check that it is a slice
		if filters.Kind() != reflect.Slice {
			return false
		}

		possible_values := []string{"WRITE", "CREATE", "RENAME", "REMOVE"}

		for i := 0; i < filters.Len(); i++ {
			elem := filters.Index(i)
			if elem.Kind() != reflect.String || elem.String() == "" {
				return false
			}

			if !slices.Contains(possible_values, elem.String()) {
				return false
			}
		}

		return true
	}
}

type VCallback func(v *validator.Validate) func(fl validator.FieldLevel) bool

func RegisterValidator(v *validator.Validate, name string, callback VCallback) {
	if err := v.RegisterValidation(name, callback(v)); err != nil {
		utils.FATAL("Failed to register validator: ", err)
	}
}

func ReadConf(path string) *ClientConf {
	var synchme_client_conf ClientConf
	if _, err := toml.DecodeFile(path, &synchme_client_conf); err != nil {
		utils.FATAL("Fatal Error: Configuration Error ", err)
	}

	validate := validator.New(validator.WithRequiredStructEnabled())

	// Register a new validator to check if the network relay is either an IP or hostname
	RegisterValidator(validate, "ip_or_hostname", IpOrHostname)
	RegisterValidator(validate, "ip_or_iface", IpOrIface)
	RegisterValidator(validate, "fs_op", ValidFilters)

	if err := validate.Struct(synchme_client_conf); err != nil {
		if errs, ok := err.(validator.ValidationErrors); ok {
			for _, e := range errs {
				utils.WARN("[Conf Field: ", e.Field(), "] Invalid for tag=",
					e.Tag(), " with value=", e.Value())
			}
		}
		utils.ERROR("invalid config")
		return nil
	}

	synchme_client_conf.Path, _ = filepath.Abs(path)
	return &synchme_client_conf
}

func GetDefaultConf() *ClientConf {
	b := func(val bool) *bool {
		return &val
	}

	return &ClientConf{
		Path:   os.Getenv(consts.SYNCHME_DEFAULT_CONFIG_PATH),
		Config: Config{WatchConf: b(true), SynchConf: b(false)},
		FS_Notification: FS_Notification{
			Paths:         []string{},
			BaseTTL:       900,
			MaxTTL:        10800,
			ExpirationInt: 1,
			SynchInterval: 0.01,
			Filters:       []string{},
		},
		Network: Network{
			ServerHost:        "127.0.0.1",
			ServerPort:        50051,
			HeartbeatInterval: 1,
		},
	}
}

// LoadConfiguration either reads or create a default configuration
// and returns it as a `ClientConf` pointer.
func LoadConfiguration(path string) *ClientConf {
	// If the path exists than we can read the configuration
	if utils.Exist(path) && strings.HasSuffix(path, ".toml") {
		return ReadConf(path)
	}

	client_conf := GetDefaultConf() // Generate a default conf
	utils.WARN("Configuration path ", path, " does not exists!")
	utils.INFO("Loading default configuration into ", client_conf.Path)

	// Write the conf into the default destination
	flags := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	file, err := os.OpenFile(client_conf.Path, flags, 0777)
	if err != nil {
		utils.FATAL("Unable to open file ", client_conf.Path, ": ", err)
	}

	defer func() {
		if err := file.Close(); err != nil {
			utils.ERROR("Error when closing file: ", err)
		}
	}()

	if err := toml.NewEncoder(file).Encode(*client_conf); err != nil {
		utils.FATAL("Unable to encode the default configuration: ", err)
	}

	// Set the synchme config environment variable to the new file
	SetEnv(consts.SYNCHME_CONFIG, client_conf.Path)

	return client_conf
}
