package main

import (
	"flag"

	"github.com/noahjalex/epoch/internal/database"
	"github.com/noahjalex/epoch/internal/handlers"
	"github.com/noahjalex/epoch/internal/logging"
)

func main() {
	// Parse CLI flags first
	var (
		port      = flag.String("port", "8080", "port to use")
		logLevel  = flag.String("log-level", "", "log level (debug, info, warn, error)")
		logFormat = flag.String("log-format", "", "log format (text, json)")
	)
	flag.Parse()

	// Override config with CLI flags if provided
	logConfig := logging.LoadConfig()
	if *logLevel != "" {
		logConfig.Level = *logLevel
	}
	if *logFormat != "" {
		logConfig.Format = *logFormat
	}

	// Initialize logging with configuration
	log := logging.Init(logConfig)

	db, repo := database.SetupDB(log)
	defer db.Close()

	// Normalize port
	if (*port)[:1] != ":" {
		*port = ":" + *port
	}

	// Run Server
	server, err := handlers.NewServer(repo, log, logConfig)
	if err != nil {
		log.WithError(err).Fatal("Failed to create server")
	}

	log.WithField("port", *port).Info("Starting server")

	if err := server.Run(*port); err != nil {
		log.WithError(err).Fatal("Server failed")
	}
}
