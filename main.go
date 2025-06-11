package main

import (
	"ccache-backend-client/com"
	"ccache-backend-client/utils"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

var BACKEND_TYPE string

const inactivityTimeout = 300 * time.Second

var ( // aliases for LOGS
	LOG  = utils.Inform
	WARN = utils.WarnUser
	TERM = utils.ReportError
)

func startServer() {
	server, err := newServer(com.SOCKET_PATH, com.FIXED_BUF_SIZE)
	if err != nil {
		WARN("Error starting server %v\n", err.Error())
		return
	}

	defer server.cleanup()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		WARN("Signal Shutdown... Exiting!")
		server.cleanup()
		os.Exit(0)
	}()

	server.start()
}

func main() {
	if len(os.Args) < 4 {
		log.Println("Usage: ccache-backend-client --url=<string> --socket=<string> --bufsize=<uint>",
			" [optional: --debug]")
		os.Exit(1)
	}

	flag.StringVar(&com.SOCKET_PATH, "socket", "", "Domain socket path for ccache")
	flag.IntVar(&com.FIXED_BUF_SIZE, "bufsize", 8192, "Size of socket buffer")
	com.PACK_SIZE = com.FIXED_BUF_SIZE / 2
	flag.StringVar(&BACKEND_TYPE, "url", "", "Backend's url")
	flag.BoolVar(&utils.DEBUG_ENABLED, "debug", false, "Debug flag")
	flag.Parse()

	if com.SOCKET_PATH == "" || BACKEND_TYPE == "" {
		log.Println("Usage: ccache-backend-client --url=<string> --socket=<string> --bufsize=<uint>",
			" [optional: --debug]")
		os.Exit(1)
	}

	// create log file if debug flag is set
	if utils.DEBUG_ENABLED {
		err := utils.OpenLogFile()
		if err != nil {
			log.Fatal(err)
		}
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
	startServer()
}
