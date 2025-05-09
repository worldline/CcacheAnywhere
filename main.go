package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

var FIXED_BUF_SIZE int
var SOCKET_PATH string
var BACKEND_TYPE string

const inactivityTimeout = 30 * time.Second

func startServer() {
	server, err := newServer(SOCKET_PATH, FIXED_BUF_SIZE)
	defer server.cleanup()

	if err != nil {
		fmt.Println("Error starting server", err)
		return
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("Signal Shutdown... Exiting!")
		server.cleanup()
		os.Exit(0)
	}()

	server.start()
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: daemon <socket_path> <buffer_size> <url> [optional.. attributes]")
		os.Exit(1)
	}

	SOCKET_PATH = os.Args[1]
	FIXED_BUF_SIZE, _ = strconv.Atoi(os.Args[2])
	if len(os.Args) > 2 {
		SOCKET_PATH = os.Args[3]
	}
	startServer()
}
