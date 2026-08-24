package main

import (
	"errors"
	"log"
	"os"

	"demo-agent/internal/app"
	"demo-agent/internal/config"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatal(err)
	}

	configuration, err := config.LoadFromEnvironment()
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
