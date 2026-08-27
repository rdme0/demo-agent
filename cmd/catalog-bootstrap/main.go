package main

import (
	"context"
	"flag"
	"log"

	"demo-agent/catalog"
	"demo-agent/internal/bootstrap"
)

func main() {
	agentStoreBaseURL := flag.String("agent-store-base-url", "", "required AgentStore API origin")
	demoAgentBaseURL := flag.String("demo-agent-base-url", "", "required demo-agent origin")
	flag.Parse()
	if *agentStoreBaseURL == "" || *demoAgentBaseURL == "" {
		log.Fatal("--agent-store-base-url and --demo-agent-base-url are required")
	}
	source, err := catalog.LoadEmbedded()
	if err != nil {
		log.Fatal(err)
	}
	client, err := bootstrap.New(*agentStoreBaseURL, *demoAgentBaseURL)
	if err != nil {
		log.Fatal(err)
	}
	if err := client.Bootstrap(context.Background(), source); err != nil {
		log.Fatal(err)
	}
}
