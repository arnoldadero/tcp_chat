package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	connectionTimeout = 10 * time.Second
	reconnectDelay    = 5 * time.Second
	maxMessageSize    = 1024
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: ./client <server_address> <port>")
		fmt.Println("Example: ./client localhost 8989")
		return
	}

	serverAddress := os.Args[1]
	port := os.Args[2]

	// Validate server address (can be IP or hostname)
	if _, err := net.ResolveTCPAddr("tcp", serverAddress+":"+port); err != nil {
		fmt.Println("Invalid server address")
		return
	}
	if _, err := strconv.Atoi(port); err != nil {
		fmt.Println("Invalid port number")
		return
	}

	var conn net.Conn
	var err error

	// Connection loop with retry logic
	for {
		conn, err = net.DialTimeout("tcp", serverAddress+":"+port, connectionTimeout)
		if err != nil {
			fmt.Printf("Unable to connect to server: %v\n", err)
			fmt.Printf("Retrying in %v...\n", reconnectDelay)
			time.Sleep(reconnectDelay)
			continue
		}
		break
	}

	// Setup connection status monitoring
	connStatus := make(chan bool)
	go func() {
		for {
			time.Sleep(1 * time.Second)
			_, err := conn.Write([]byte{})
			if err != nil {
				connStatus <- false
				return
			}
			connStatus <- true
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
					fmt.Println("\nConnection lost. Attempting to reconnect...")
					conn.Close()
					main() // Restart client
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