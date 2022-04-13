package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func init() {
	InitClientRateLimiterMap()
}

func main() {
	fmt.Println("Hello!")
	defer fmt.Println("Goodbye.")

	// start the http server
	fmt.Println("Starting server")
	srv := InitServer(nil)
	go StartServer(srv)
	fmt.Println("...done")

	// create some clients
	fmt.Println("Creating client rate limiters")
	for i := 1; i < 11; i++ {
		clientRateLimiterMap[fmt.Sprintf("client-%v", i)] = NewLimiter(i, i)
	}
	fmt.Println("...done")

	// block for shutdown
	fmt.Println("Application started, waiting for shutdown")
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM) // sigterm is what kubernetes uses to shutdown pods
	<-done
	fmt.Println("\nReceived shutdown")
	fmt.Println("Closing server")
	ShutdownServer(srv, context.Background())
	fmt.Println("...done")
	fmt.Println("Closing clients")
	for _, client := range clientRateLimiterMap {
		client.Shutdown()
	}
	fmt.Println("...done")
	return
}
