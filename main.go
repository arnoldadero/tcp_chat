package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

const defaultPort = "8989"

var (
	clients   = make(map[net.Conn]string)
	mutex     sync.Mutex
	messages  []string // To store previous messages
)

func main() {
	port := defaultPort
	if len(os.Args) > 1 {
		if len(os.Args) != 2 {
			fmt.Println("[USAGE]: ./TCPChat $port")
			os.Exit(1)
		}
		port = os.Args[1]
	}

	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Error starting server:", err)
		os.Exit(1)
	}
	defer ln.Close()

	fmt.Println("Listening on the port :" + port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	fmt.Println("New connection from:", conn.RemoteAddr())
	logo := `Welcome to TCP-Chat!
         _nnnn_
        dGGGGMMb
       @p~qp~~qMb
       M|@||@) M|
       @,----.JM|
      JS^\__/  qKL
     dZP        qKRb
    dZP          qKKb
   fZP            SMMb
   HZM            MMMM
   FqM            MMMM
 __| ".        |\dS"qML
 |    `.       | `' \Zq
_)      \.___.,|     .'
\____   )MMMMMP|   .'
     `-'       `--'
[ENTER YOUR NAME]: `
	_, err := conn.Write([]byte(logo))
	if err != nil {
		log.Println("Error sending logo:", err)
		return
	}

	reader := bufio.NewReader(conn)
	clientName, err := reader.ReadString('\n')
	if err != nil {
		log.Println("Error reading client name:", err)
		return
	}
	clientName = strings.TrimSpace(clientName)

	mutex.Lock()
	clients[conn] = clientName
	mutex.Unlock()

	fmt.Printf("Client '%s' joined the chat\n", clientName)

	// Send previous messages to the new client
	for _, msg := range messages {
		_, err := conn.Write([]byte(msg + "\n"))
		if err != nil {
			log.Println("Error sending previous message:", err)
			return
		}
	}

	// Keep connection open for messaging
	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Client '%s' disconnected\n", clientName)
			mutex.Lock()
			delete(clients, conn)
			mutex.Unlock()
			return
		}
		message = strings.TrimSpace(message)
		if len(message) > 0 {
			currentTime := time.Now().Format("2006-01-02 15:04:05")
			formattedMessage := fmt.Sprintf("[%s][%s]:%s", currentTime, clientName, message)

			// Add message to the history
			messages = append(messages, formattedMessage)

			// Broadcast message to all other clients
			mutex.Lock()
			for clientConn, client := range clients {
				if clientConn != conn {
					_, err := clientConn.Write([]byte(formattedMessage + "\n"))
					if err != nil {
						log.Printf("Error broadcasting message to %s: %v\n", client, err)
						// Handle disconnection if broadcasting fails
						delete(clients, conn)
						conn.Close()
					}
				}
			}
			mutex.Unlock()
		}
	}
}