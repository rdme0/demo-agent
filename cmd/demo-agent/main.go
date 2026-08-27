package main

import (
	"errors"
	"flag"
	"log"
	"os"

	"demo-agent/internal/app"
	"demo-agent/internal/config"

	"github.com/joho/godotenv"
)

func main() {
	configPath := flag.String("config", "config/application.yaml", "application configuration YAML")
	host := flag.String("host", "", "listener host override")
	port := flag.Int("port", 0, "listener port override")
	mode := flag.String("mode", "", "agent mode override: fixture or openai")
	flag.Parse()

	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatal(err)
	}

	configuration, err := config.Load(*configPath, config.Overrides{
		Host: *host, Port: *port, AgentMode: *mode,
	}, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}

	application, err := app.New(configuration, nil)
	if err != nil {
		log.Fatal(err)
	}

	if err := application.Run(configuration.ListenAddress()); err != nil {
		log.Fatal(err)
	}
}
