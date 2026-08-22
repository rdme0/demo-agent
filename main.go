package main

import (
	"log"

	"demo-agent/internal/agent"
	"demo-agent/internal/config"
	"demo-agent/internal/runtime"
	"demo-agent/internal/server"
)

func main() {
	configuration, err := config.LoadFromEnvironment()
	if err != nil {
		log.Fatal(err)
	}

	agents, err := agent.NewRegistry(agent.NewFixtureAgents()...)
	if err != nil {
		log.Fatal(err)
	}
	callbackClient := runtime.NewCallbackClient()

	application, err := server.New(configuration, agents, callbackClient, nil)
	if err != nil {
		log.Fatal(err)
	}

	if err := application.Run(configuration.ListenAddress()); err != nil {
		log.Fatal(err)
	}
}
