package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/maragudk/lightsail"
)

func main() {
	os.Exit(start())
}

func start() int {
	log := log.New(os.Stdout, "", 0)

	args := os.Args[1:]
	if len(args) == 0 {
		return printUsage(log, "deploy")
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Println("Error loading AWS config:", err)
		return 1
	}
	l := lightsail.New(lightsail.NewOptions{
		Config: cfg,
		Log:    log,
	})

	switch args[0] {
	case "deploy":
		if len(args) < 2 {
			return printUsage(log, "deploy <service name>")
		}
		err = l.Deploy(args[1])
	default:
		return printUsage(log, "deploy")
	}

	if err != nil {
		log.Printf("Error running %v: %v\n", os.Args[1], err)
		return 1
	}
	return 0
}

func printUsage(log *log.Logger, text string) int {
	log.Println("Usage: lightsail " + text)
	return 1
}
