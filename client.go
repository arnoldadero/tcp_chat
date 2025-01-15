package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: ./client <server_address> <port>")
		return
	}

	serverAddress := os.Args[1]
	port := os.Args[2]

	conn, err := net.Dial("tcp", serverAddress+":"+port)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	fmt.Println("Connected to server:", conn.RemoteAddr())

	// Read logo and prompt
	reader := bufio.NewReader(conn)
	logo, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal("Failed to read logo from server")
	}
	fmt.Print(logo)

	// Get client name from user
	clientNameReader := bufio.NewReader(os.Stdin)
	clientName, _ := clientNameReader.ReadString('\n')
	clientName = strings.TrimSpace(clientName)

	// Send client name to server
	_, err = conn.Write([]byte(clientName + "\n"))
	if err != nil {
		log.Fatal("Failed to send name to server")
	}

	go receiveMessages(conn)

	// Send messages
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		message := scanner.Text()
		if _, err := conn.Write([]byte(message + "\n")); err != nil {
			log.Println("Error sending message:", err)
			return
		}
	}

	if err := scanner.Err(); err != nil {
		log.Println("Error reading input:", err)
	}
}

func receiveMessages(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Println("Connection closed by server.")
			}
			return
		}
		fmt.Print(message)
	}
}