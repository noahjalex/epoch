package main

import (
	"flag"

	"github.com/noahjalex/epoch/internal/database"
	"github.com/noahjalex/epoch/internal/handlers"
	"github.com/noahjalex/epoch/internal/logging"
)

func main() {

	log := logging.Init()

	db, repo := database.SetupDB(log)
	defer db.Close()

	// Port / Flag parsing
	pport := flag.String("port", "8080", "port to use")
	flag.Parse()

	var port string
	if pport == nil {
		port = "8080"
	} else {
		port = *pport
	}
	if port[:1] != ":" {
		port = ":" + port
	}

	// Run Server
	server, err := handlers.NewServer(repo, log)
	if err != nil {
		log.Fatalf("error starting server %v", err)
	}

	server.Run(port)
}
