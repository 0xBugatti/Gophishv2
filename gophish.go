package main

/*
gophish - Open-Source Phishing Framework

The MIT License (MIT)

Copyright (c) 2013 Jordan Wright

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/gophish/gophish/config"
	"github.com/gophish/gophish/controllers"
	"github.com/gophish/gophish/controllers/api"
	"github.com/gophish/gophish/crypto"
	"github.com/gophish/gophish/dialer"
	"github.com/gophish/gophish/imap"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/middleware"
	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/webhook"
)

const (
	modeAll   string = "all"
	modeAdmin string = "admin"
	modePhish string = "phish"
)

var (
	configPath            = kingpin.Flag("config", "Location of config.json.").Default("./config.json").String()
	disableMailer         = kingpin.Flag("disable-mailer", "Disable the mailer (for use with multi-system deployments)").Bool()
	mode                  = kingpin.Flag("mode", fmt.Sprintf("Run the binary in one of the modes (%s, %s or %s)", modeAll, modeAdmin, modePhish)).Default("all").Enum(modeAll, modeAdmin, modePhish)
	generateEncryptionKey = kingpin.Flag("generate-encryption-key", "Generate a new random encryption key and exit").Bool()
	migrateEncryption     = kingpin.Flag("migrate-encryption", "Encrypt all plaintext sensitive fields in the database and exit").Bool()
	migrateDecryption     = kingpin.Flag("migrate-decryption", "Decrypt all encrypted sensitive fields in the database and exit").Bool()
	showEncryptionStatus  = kingpin.Flag("encryption-status", "Show the encryption status of the database and exit").Bool()
	dryRun                = kingpin.Flag("dry-run", "Preview migration changes without applying them (use with --migrate-encryption or --migrate-decryption)").Bool()
	// workDir allows operators to change the process working directory so that
	// relative paths in config.json and template paths resolve correctly (8.5).
	workDir = kingpin.Flag("workdir", "Working directory for Gophish files (default: current directory).").Default(".").String()
)

func main() {
	// Load the version

	version, err := ioutil.ReadFile("./VERSION")
	if err != nil {
		log.Fatal(err)
	}
	kingpin.Version(string(version))

	// Parse the CLI flags and load the config
	kingpin.CommandLine.HelpFlag.Short('h')
	kingpin.Parse()

	// Change working directory if --workdir was specified (8.5).
	if *workDir != "." {
		if err := os.Chdir(*workDir); err != nil {
			log.Fatal(err)
		}
	}

	// Handle encryption key generation early (no config or database needed)
	if *generateEncryptionKey {
		key, err := crypto.GenerateEncryptionKey()
		if err != nil {
			fmt.Printf("Error generating encryption key: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s=%s\n", crypto.EncryptionKeyEnvVar, key)
		return
	}

	// Load the config
	conf, err := config.LoadConfig(*configPath)
	// Just warn if a contact address hasn't been configured
	if err != nil {
		log.Fatal(err)
	}
	if conf.ContactAddress == "" {
		log.Warnf("No contact address has been configured.")
		log.Warnf("Please consider adding a contact_address entry in your config.json")
	}
	config.Version = string(version)

	// Configure our various upstream clients to make sure that we restrict
	// outbound connections as needed.
	dialer.SetAllowedHosts(conf.AdminConf.AllowedInternalHosts)
	webhook.SetTransport(&http.Transport{
		DialContext: dialer.Dialer().DialContext,
	})

	err = log.Setup(conf.Logging)
	if err != nil {
		log.Fatal(err)
	}

	// Provide the option to disable the built-in mailer
	// Setup the global variables and settings
	err = models.Setup(conf)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize encryption system (reads ANGLERPHISH_ENCRYPTION_KEY env var)
	encryptionEnabled, err := crypto.InitEncryption()
	if err != nil {
		log.Fatal("Error initializing encryption: ", err)
	}
	if encryptionEnabled {
		log.Info("Database encryption is enabled")
	}

	// Handle encryption CLI commands (require initialized database)
	if *showEncryptionStatus {
		report, err := models.GetEncryptionStatus()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(report)
		return
	}
	if *migrateEncryption {
		results, err := models.MigrateToEncrypted()
		if err != nil {
			log.Fatal(err)
		}
		for _, r := range results {
			fmt.Println(r)
		}
		return
	}
	if *migrateDecryption {
		results, err := models.MigrateToDecrypted()
		if err != nil {
			log.Fatal(err)
		}
		for _, r := range results {
			fmt.Println(r)
		}
		return
	}

	// Initialize report generation services
	api.InitReportServices(&conf.ReportsConf)

	// Unlock any maillogs and smslogs that may have been locked for processing
	// when Gophish was last shutdown.
	err = models.UnlockAllMailLogs()
	if err != nil {
		log.Fatal(err)
	}
	err = models.UnlockAllSMSLogs()
	if err != nil {
		log.Fatal(err)
	}

	// Create our servers
	adminOptions := []controllers.AdminServerOption{}
	if *disableMailer {
		adminOptions = append(adminOptions, controllers.WithWorker(nil))
	}
	adminConfig := conf.AdminConf
	adminServer := controllers.NewAdminServer(adminConfig, adminOptions...)
	middleware.Store.Options.Secure = adminConfig.UseTLS

	phishConfig := conf.PhishConf
	phishServer := controllers.NewPhishingServer(phishConfig)

	imapMonitor := imap.NewMonitor()
	if *mode == "admin" || *mode == "all" {
		go adminServer.Start()
		go imapMonitor.Start()
	}
	if *mode == "phish" || *mode == "all" {
		go phishServer.Start()
	}

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	log.Info("CTRL+C Received... Gracefully shutting down servers")
	api.StopReportServices()
	if *mode == modeAdmin || *mode == modeAll {
		adminServer.Shutdown()
		imapMonitor.Shutdown()
	}
	if *mode == modePhish || *mode == modeAll {
		phishServer.Shutdown()
	}

}
