package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var FIXED_BUF_SIZE int
var isShuttingDown bool

const inactivityTimeout = 30 * time.Second

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: daemon <socket_path> <buffer_size> [optional.. attributes]")
		os.Exit(1)
	}

	socketPath := os.Args[1]
	FIXED_BUF_SIZE, _ = strconv.Atoi(os.Args[2])
	fmt.Println(FIXED_BUF_SIZE)

	defer func() {
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			fmt.Printf("Error removing socket: %v\n", err)
		}
	}()

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		fmt.Printf("Error listening on socket: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	fmt.Printf("Listening on %s\n", socketPath)

	// Handle SIGINT and SIGTERM for graceful shutdown
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)

	lastActivity := time.Now()

	go func() {
		// Start routine for listening
		if isShuttingDown {
			return
		}
		for {
			conn, err := listener.Accept()
			if err != nil {
				fmt.Printf("Error accepting connection: %v\n", err)
				return
			}
			lastActivity = time.Now()
			go handleConnection(conn, &lastActivity)
		}
	}()

	// Monitor inactivity
	for {
		select {
		case <-stopChan:
			fmt.Println("Shutting down...")
			isShuttingDown = true
			return
		default:
			if time.Since(lastActivity) >= inactivityTimeout {
				fmt.Println("Socket closed due to inactivity.")
				return
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func handleConnection(conn net.Conn, lastActivity *time.Time) {
	defer conn.Close()

	fmt.Println("ACK Running handler.#")
	buf := make([]byte, FIXED_BUF_SIZE)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Printf("Connection error: %v\n", err)
			return
		}
		// conn.Write([]byte("NEW CONNECTION!"))

		// Echo back the data or send "ACK" based on the received message
		if n > 0 {
			// Update last activity time
			*lastActivity = time.Now()
			receivedMessage := string(buf[:n])
			fmt.Printf("Received: %s\n", receivedMessage)

			// Check for the "setup" message
			if strings.HasPrefix(receivedMessage, "!SETUP") {
				_, err := conn.Write([]byte("ACK!")) // Send ACK for "setup"
				if err != nil {
					fmt.Printf("Error writing to connection: %v\n", err)
					return
				}
			}
		}
	}
}
