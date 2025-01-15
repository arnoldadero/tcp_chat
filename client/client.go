package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: ./client <server_address> <port>")
		return
	}

	serverAddress := os.Args[1]
	port := os.Args[2]

	conn, err := net.Dial("tcp", serverAddress+":"+port)
	if err != nil {
		log.Fatalf("Unable to connect to server: %v", err)
		return
	}

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
			message, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("Disconnected from server.")
				return
			}
			if strings.HasPrefix(message, "Connected users:") {
				fmt.Print(message)
				continue
			}
			// Assuming the server sends messages in the format "[timestamp] message"
			parts := strings.SplitN(message, "] ", 2)
			if len(parts) == 2 {
				timestamp := strings.TrimPrefix(parts[0], "[")
				fmt.Printf("[%s] %s", timestamp, parts[1])
			} else {
				fmt.Print(message)
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