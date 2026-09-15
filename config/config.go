package config

import (
	"encoding/json"
	"io/ioutil"

	log "github.com/gophish/gophish/logger"
)

// AdminServer represents the Admin server configuration details
type AdminServer struct {
	ListenURL            string   `json:"listen_url"`
	UseTLS               bool     `json:"use_tls"`
	CertPath             string   `json:"cert_path"`
	KeyPath              string   `json:"key_path"`
	CSRFKey              string   `json:"csrf_key"`
	AllowedInternalHosts []string `json:"allowed_internal_hosts"`
	TrustedOrigins       []string `json:"trusted_origins"`
}

// PhishServer represents the Phish server configuration details
type PhishServer struct {
	ListenURL string `json:"listen_url"`
	UseTLS    bool   `json:"use_tls"`
	CertPath  string `json:"cert_path"`
	KeyPath   string `json:"key_path"`
}

// Reports represents the report generation configuration
type Reports struct {
	StoragePath string `json:"storage_path"`
}

// Config represents the configuration information.
type Config struct {
	AdminConf       AdminServer `json:"admin_server"`
	PhishConf       PhishServer `json:"phish_server"`
	DBName          string      `json:"db_name"`
	DBPath          string      `json:"db_path"`
	DBSSLCaPath     string      `json:"db_sslca_path"`
	MigrationsPath  string      `json:"migrations_prefix"`
	TestFlag        bool        `json:"test_flag"`
	ContactAddress  string      `json:"contact_address"`
	Logging         *log.Config `json:"logging"`
	ReportsConf     Reports     `json:"reports"`
	// DBMaxOpenConns sets the maximum number of open connections to the DB.
	// A value of 0 means unlimited. Default: 1 (preserves historical behaviour).
	DBMaxOpenConns int `json:"db_max_open_conns"`
	// DBMaxIdleConns sets the maximum number of idle connections in the pool.
	// A value of 0 disables idle connections. Default: 1.
	DBMaxIdleConns int `json:"db_max_idle_conns"`
	// AttachmentTemplateTypes lists MIME types (beyond Office docs) that have
	// Go template variables rendered before being attached. Supports HTA, PS1,
	// BAT, etc. for payload delivery simulations (3.9).
	AttachmentTemplateTypes []string `json:"attachment_template_types"`
}

// Version contains the current gophish version
var Version = ""

// ServerName is the server type that is returned in the transparency response.
const ServerName = "postfix"

// LoadConfig loads the configuration from the specified filepath
func LoadConfig(filepath string) (*Config, error) {
	// Get the config file
	configFile, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	config := &Config{}
	err = json.Unmarshal(configFile, config)
	if err != nil {
		return nil, err
	}
	if config.Logging == nil {
		config.Logging = &log.Config{}
	}
	// Choosing the migrations directory based on the database used.
	config.MigrationsPath = config.MigrationsPath + config.DBName
	// Explicitly set the TestFlag to false to prevent config.json overrides
	config.TestFlag = false
	return config, nil
}
