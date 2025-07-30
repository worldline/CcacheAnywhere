package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"ccache-backend-client/internal/app"
	//lint:ignore ST1001 do want nice LOG operations
	. "ccache-backend-client/internal/logger"
	storage "ccache-backend-client/internal/storage"
	"ccache-backend-client/internal/tlv"
)

var BACKEND_TYPE string

func StartServer() {
	server, err := app.NewServer(tlv.SOCKET_PATH, tlv.FIXED_BUF_SIZE, BACKEND_TYPE)
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

func parseArgs() (err error) {
	tlv.SOCKET_PATH = os.Getenv("_CCACHE_SOCKET_PATH")
	tlv.FIXED_BUF_SIZE, err = strconv.Atoi(os.Getenv("_CCACHE_BUFFER_SIZE"))
	BACKEND_TYPE = os.Getenv("_CCACHE_REMOTE_URL")

	countAttrs := os.Getenv("_CCACHE_NUM_ATTR")

	for i := range countAttrs {
		key := os.Getenv("_CCACHE_ATTR_KEY_" + strconv.Itoa(i))
		value := os.Getenv("_CCACHE_ATTR_VALUE_" + strconv.Itoa(i))
		storage.BackendAttributes = append(storage.BackendAttributes, storage.Attribute{Key: key, Value: value})
	}

	flag.BoolVar(&DEBUG_ENABLED, "debug", false, "Debug flag")
	flag.Parse()

	if tlv.SOCKET_PATH == "" || BACKEND_TYPE == "" || err != nil {
		log.Println("Make sure you are passing the environment variables!")
		return fmt.Errorf("incorrect usage")
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
		log.Fatal("Parsing error!", err)
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
