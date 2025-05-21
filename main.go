package main

import (
	"ccache-backend-client/com"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

var BACKEND_TYPE string

const inactivityTimeout = 300 * time.Second

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
	flag.IntVar(&com.FIXED_BUF_SIZE, "bufsize", 8192, "Size of socket buffer")
	com.PACK_SIZE = com.FIXED_BUF_SIZE / 2
	flag.StringVar(&BACKEND_TYPE, "url", "", "Backend's url")
	flag.Parse()

	if com.SOCKET_PATH == "" || BACKEND_TYPE == "" {
		fmt.Println("Usage: ccache-backend-client --url=<string> --socket=<string> --bufsize=<uint>",
			" [optional.. attributes]")
		os.Exit(1)
	}

	execDir, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}

	err = os.Chdir(filepath.Dir(execDir))
	if err != nil {
		log.Fatal(err)
	}

	startServer()
}
