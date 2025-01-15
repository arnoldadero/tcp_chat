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
	connStatus := make(chan bool, 1)
	done := make(chan struct{})
	defer close(done)

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
				_, err := conn.Write([]byte{})
				if err != nil {
					select {
					case connStatus <- false:
					case <-done:
						return
					}
					return
				}
				select {
				case connStatus <- true:
				case <-done:
					return
				}
			case <-shutdownChan:
				return
			case <-done:
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

	// Handle receiving messages from the server
	go func() {
		reader := bufio.NewReader(conn)
		for {
			select {
			case status := <-connStatus:
				if !status {
					fmt.Println("\nConnection lost. Please restart the client.")
					conn.Close()
					return
				}
			default:
				message, err := reader.ReadString('\n')
				if err != nil {
					if err == io.EOF {
						fmt.Println("\nServer closed the connection")
					} else {
						fmt.Printf("\nConnection error: %v\n", err)
					}
					return
				}

				// Enforce message size limit
				if len(message) > maxMessageSize {
					fmt.Println("\nMessage too large, skipping")
					continue
				}

				if strings.HasPrefix(message, "Connected users:") {
					fmt.Print(message)
					continue
				}

				// Handle ASCII art lines (they won't have timestamps)
				if strings.HasPrefix(message, "Welcome to TCP-Chat!") ||
				   strings.HasPrefix(message, "         _nnnn_") ||
				   strings.HasPrefix(message, "[ENTER YOUR NAME]:") ||
				   strings.HasPrefix(message, "        dGGGGMMb") ||
				   strings.HasPrefix(message, "       @p~qp~~qMb") {
					fmt.Print(message)
					continue
				}
				
				// Parse and display message with timestamp
				parts := strings.SplitN(message, "] ", 2)
				if len(parts) == 2 {
					timestamp := strings.TrimPrefix(parts[0], "[")
					fmt.Printf("[%s] %s", timestamp, parts[1])
				} else {
					fmt.Print(message)
				}
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
