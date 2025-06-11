package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"ccache-backend-client/internal/app"
	"ccache-backend-client/internal/com"
	. "ccache-backend-client/internal/logger"
)

var BACKEND_TYPE string

func StartServer() {
	server, err := app.NewServer(com.SOCKET_PATH, com.FIXED_BUF_SIZE, BACKEND_TYPE)
	if err != nil {
		WARN("Error starting server %v\n", err.Error())
		return
	}

	defer server.Cleanup()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		WARN("Signal Shutdown... Exiting!")
		server.Cleanup()
		os.Exit(0)
	}()

	server.Start()
}

func parseArgs() error {
	if len(os.Args) < 4 {
		log.Println("Usage: ccache-backend-client --url=<string> --socket=<string> --bufsize=<uint>",
			" [optional: --debug]")
		return fmt.Errorf("Incorrect usage!\n")
	}

	flag.StringVar(&com.SOCKET_PATH, "socket", "", "Domain socket path for ccache")
	flag.IntVar(&com.FIXED_BUF_SIZE, "bufsize", 8192, "Size of socket buffer")
	com.PACK_SIZE = com.FIXED_BUF_SIZE / 2
	flag.StringVar(&BACKEND_TYPE, "url", "", "Backend's url")
	flag.BoolVar(&DEBUG_ENABLED, "debug", false, "Debug flag")
	flag.Parse()

	if com.SOCKET_PATH == "" || BACKEND_TYPE == "" {
		log.Println("Usage: ccache-backend-client --url=<string> --socket=<string> --bufsize=<uint>",
			" [optional: --debug]")
		return fmt.Errorf("Incorrect usage!\n")
	}

	// create log file if debug flag is set
	if DEBUG_ENABLED {
		err := OpenLogFile()
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	if err := parseArgs(); err != nil {
		log.Fatal(err)
	}

	execDir, err := os.Executable()
	if err != nil {
		TERM(err)
	}

	err = os.Chdir(filepath.Dir(execDir))
	if err != nil {
		TERM(err)
	}
	LOG("Start server!\n")
	StartServer()
}
