package main

import (
	"ccache-backend-client/com"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var BACKEND_TYPE string

const inactivityTimeout = 30 * time.Second

func startServer() {
	server, err := newServer(com.SOCKET_PATH, com.FIXED_BUF_SIZE)
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
	if len(os.Args) < 4 {
		fmt.Println("Usage: ccache-backend-client --url=<string> --socket=<string> --bufsize=<uint>",
			" [optional.. attributes]")
		os.Exit(1)
	}

	flag.StringVar(&com.SOCKET_PATH, "socket", "", "Domain socket path for ccache")
	flag.IntVar(&com.FIXED_BUF_SIZE, "bufsize", 4096, "Size of socket buffer")
	flag.StringVar(&BACKEND_TYPE, "url", "", "Backend's url")
	flag.Parse()

	if com.SOCKET_PATH == "" || BACKEND_TYPE == "" {
		fmt.Println("Usage: ccache-backend-client --url=<string> --socket=<string> --bufsize=<uint>",
			" [optional.. attributes]")
		os.Exit(1)
	}

	startServer()
}
