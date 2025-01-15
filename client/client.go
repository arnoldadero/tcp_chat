package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	connectionTimeout = 10 * time.Second
	reconnectDelay    = 5 * time.Second
	maxMessageSize    = 1024
)

var shutdownChan = make(chan struct{})

func main() {
	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	if len(os.Args) != 3 {
		fmt.Println("Usage: ./client <server_address> <port>")
		fmt.Println("Example: ./client localhost 8989")
		return
	}

	// Handle shutdown signals
	go func() {
		<-sigChan
		close(shutdownChan)
	}()

	serverAddress := os.Args[1]
	port := os.Args[2]

	// Validate server address (can be IP or hostname)
	// Validate port first
	portNum, portErr := strconv.Atoi(port)
	if portErr != nil || portNum < 1 || portNum > 65535 {
		fmt.Println("Invalid port number")
		return
	}

	// Then validate server address
	if _, addrErr := net.ResolveTCPAddr("tcp", serverAddress+":"+port); addrErr != nil {
		fmt.Println("Invalid server address")
		return
	}

	var conn net.Conn
	var err error

	// Connection loop with retry logic
	connected := false
	retryCount := 0
	maxRetries := 3

	for !connected && retryCount < maxRetries {
		conn, err = net.DialTimeout("tcp", serverAddress+":"+port, connectionTimeout)
		if err != nil {
			retryCount++
			fmt.Printf("Unable to connect to server: %v\n", err)
			if retryCount < maxRetries {
				fmt.Printf("Retrying in %v... (attempt %d/%d)\n", reconnectDelay, retryCount, maxRetries)
				time.Sleep(reconnectDelay)
				continue
			}
			fmt.Println("Max connection attempts reached")
			return
		}
		connected = true
	}

	if !connected {
		fmt.Println("Failed to establish connection")
		return
	}

	// Setup connection status monitoring
	connStatus := make(chan bool)
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_, err := conn.Write([]byte{})
				if err != nil {
					connStatus <- false
					close(connStatus)
					return
				}
				connStatus <- true
			case <-shutdownChan:
				close(connStatus)
				return
			}
		}
	}()

	// Send protocol handshake
	_, err = conn.Write([]byte("CHAT/1.0\n"))
	if err != nil {
		log.Fatalf("Error sending handshake: %v", err)
		return
	}

	defer conn.Close()

	fmt.Println("Connected to the server!")

	// handleIncomingMessages processes incoming messages from the server
	func() {
		reader := bufio.NewReader(conn)
		for {
			message, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					fmt.Println("Error reading message:", err)
				}
				break
			}

			// Process and print the message
			message = strings.TrimSpace(message)
			if message != "" {
				fmt.Println(message)
			}
		}
	}()

	// Handle sending messages to the server
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		message := scanner.Text()
		trimmedMessage := strings.TrimSpace(message)
		if trimmedMessage == "/list" {
			_, err := conn.Write([]byte("/list\n"))
			if err != nil {
				fmt.Println("Error sending list command:", err)
				return
			}
			continue // Don't send the /list command as a regular message
		} else if strings.HasPrefix(trimmedMessage, "/msg ") {
			parts := strings.SplitN(trimmedMessage, " ", 3)
			if len(parts) == 3 {
				recipient := parts[1]
				privateMessage := parts[2]
				_, err := conn.Write([]byte(fmt.Sprintf("/msg %s %s\n", recipient, privateMessage)))
				if err != nil {
					fmt.Println("Error sending private message:", err)
					return
				}
				continue // Don't send the /msg command as a regular message
			} else {
				fmt.Println("Invalid private message format. Use /msg <username> <message>")
				continue
			}
		} else if trimmedMessage != "" {
			_, err := conn.Write([]byte(trimmedMessage + "\n"))
			if err != nil {
				fmt.Println("Error sending message:", err)
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading input:", err)
	}
}
