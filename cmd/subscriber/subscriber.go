package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"github.com/nats-io/nats.go"
)

func main() {
	// Connect to NATS server
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	// Subscribe to subject
	log.Printf("Listening on all subjects")
	sub, err := nc.Subscribe("interface-counters", func(msg *nats.Msg) {
		log.Printf("Received message on [%s]: %s", msg.Subject, string(msg.Data))
	})
	if err != nil {
		log.Fatal(err)
	}

	// Handle SIGINT and SIGTERM signals to gracefully close the application.
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	// Wait until receiving a termination signal.
	<-c

	// Unsubscribe and Drain the connection.
	if err := sub.Unsubscribe(); err != nil {
		log.Fatal(err)
	}
	if err := nc.Drain(); err != nil {
		log.Fatal(err)
	}
}
