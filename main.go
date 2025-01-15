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

const (
	DefaultPort     = "8989"
	maxConnections  = 10
)

var (
	clients   = make(map[net.Conn]string) // Map to store client connections and names
	mutex     sync.Mutex                 // Mutex to protect access to the clients map
	messages  []string                   // Slice to store chat messages
	connCount int                        // Counter for active connections
)

// GetClients returns a copy of the clients map for testing purposes
func GetClients() map[net.Conn]string {
	mutex.Lock()
	defer mutex.Unlock()
	
	// Create a new map and copy all entries
	clientsCopy := make(map[net.Conn]string)
	for k, v := range clients {
		clientsCopy[k] = v
	}
	return clientsCopy
}

func main() {
	port := DefaultPort
	if len(os.Args) > 1 {
		if len(os.Args) != 2 {
			fmt.Println("[USAGE]: ./TCPChat $port")
			os.Exit(1)
		}
		port = os.Args[1]
	}

	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
	defer ln.Close()

	fmt.Println("Listening on the port :" + port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}

		// Validate connection by checking first bytes
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil || !strings.HasPrefix(string(buf[:n]), "CHAT/1.0") {
			conn.Write([]byte("Invalid protocol. Please use TCP chat client.\n"))
			conn.Close()
			continue
		}

		mutex.Lock()
		if connCount >= maxConnections {
			mutex.Unlock()
			conn.Write([]byte("Server is full. Please try again later.\n"))
			conn.Close()
			continue
		}
		connCount++
		mutex.Unlock()
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		mutex.Lock()
		connCount--
		mutex.Unlock()
		if _, ok := clients[conn]; ok {
			clientName := clients[conn]
			delete(clients, conn)
			broadcastMessage(fmt.Sprintf("%s has left our chat...", clientName), conn)
			log.Printf("Client disconnected: %s", clientName)
		}
	}()

	fmt.Println("New connection from:", conn.RemoteAddr())
	reader := bufio.NewReader(conn)

	// Send welcome messages
	_, err := conn.Write([]byte("Welcome to TCP-Chat!\n"))
	if err != nil {
		log.Printf("Error sending welcome message: %v", err)
		return
	}

	// Send the ASCII art logo
	logo := []string{
		"         _nnnn_",
		"        dGGGGMMb",
		"       @p~qp~~qMb",
		"       M|@||@) M|",
		"       @,----.JM|",
		"      JS^\\__/  qKL",
		"     dZP        qKRb",
		"    dZP          qKKb",
		"   fZP            SMMb",
		"   HZM            MMMM",
		"   FqM            MMMM",
		" __| \".        |\\dS\"qML",
		" |    `.       | `' \\Zq",
		"_)      \\.___.,|     .'",
		"\\____   )MMMMMP|   .'",
		"     `-'       `--'",
	}
	
	for _, line := range logo {
		_, err := conn.Write([]byte(line + "\n"))
		if err != nil {
			log.Printf("Error sending logo line: %v", err)
			return
		}
	}

	// Prompt for the client's name
	_, err = conn.Write([]byte("[ENTER YOUR NAME]: "))
	if err != nil {
		log.Printf("Error sending name prompt: %v", err)
		return
	}

	// Read client name
	clientName, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("Error reading client name: %v", err)
		return
	}
	clientName = strings.TrimSpace(clientName)

	// Validate name
	if clientName == "" {
		_, err := conn.Write([]byte("Name cannot be empty. Please reconnect.\n"))
		if err != nil {
			log.Printf("Error sending empty name message: %v", err)
		}
		conn.Close()
		return
	}

	// Add client to map with proper synchronization
	mutex.Lock()
	clients[conn] = clientName
	mutex.Unlock()

	// Send confirmation message and wait for it to complete
	_, err = conn.Write([]byte(fmt.Sprintf("Welcome, %s!\n", clientName)))
	if err != nil {
		log.Printf("Error sending welcome message: %v", err)
		mutex.Lock()
		delete(clients, conn)
		mutex.Unlock()
		return
	}

	// Send previous messages to the new client
	for _, msg := range messages {
		_, err := conn.Write([]byte(msg + "\n"))
		if err != nil {
			log.Printf("Error sending previous message: %v", err)
		}
	}

	// Notify other clients about the new connection
	broadcastMessage(fmt.Sprintf("%s has joined our chat...", clientName), conn)

	log.Printf("Client connected: %s", clientName)

	// Handle incoming messages from the client
	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			// Handle client disconnection
			mutex.Lock()
			delete(clients, conn)
			mutex.Unlock()
			broadcastMessage(fmt.Sprintf("%s has left our chat...", clientName), conn)
			log.Printf("Client disconnected: %s", clientName)
			return
		}
		message = strings.TrimSpace(message)
		if strings.HasPrefix(message, "/msg ") {
			parts := strings.SplitN(message, " ", 3)
			if len(parts) == 3 {
				recipientName := parts[1]
				privateMessage := parts[2]

				mutex.Lock()
				recipientConn := findConnectionByName(recipientName)
				mutex.Unlock()

				if recipientConn != nil {
					// Send the private message
					_, err := recipientConn.Write([]byte(fmt.Sprintf("[Private from %s]: %s\n", clientName, privateMessage)))
					if err != nil {
						log.Printf("Error sending private message to %s: %v", recipientName, err)
						// Optionally notify the sender about the failure
						conn.Write([]byte(fmt.Sprintf("Could not send private message to %s\n", recipientName)))
					} else {
						// Notify the sender about successful delivery
						conn.Write([]byte(fmt.Sprintf("Private message sent to %s\n", recipientName)))
					}
				} else {
					// Notify the sender if the recipient is not found
					conn.Write([]byte(fmt.Sprintf("User %s not found\n", recipientName)))
				}
				continue
			} else {
				conn.Write([]byte("Invalid private message format. Use /msg <username> <message>\n"))
				continue
			}
		} else if message == "/list" {
			// Handle /list command
			mutex.Lock()
			var userList strings.Builder
			userList.WriteString("Connected users:\n")
			for _, name := range clients {
				userList.WriteString(name + "\n")
			}
			mutex.Unlock()
			conn.Write([]byte(userList.String()))
			continue
		}
		if message != "" {
			formattedMessage := fmt.Sprintf("[%s][%s]:%s", time.Now().Format("2006-01-02 15:04:05"), clientName, message)
			messages = append(messages, formattedMessage)
			broadcastMessage(formattedMessage, conn)
		}
	}
}

func findConnectionByName(name string) net.Conn {
	for conn, clientName := range clients {
		if clientName == name {
			return conn
		}
	}
	return nil
}

func broadcastMessage(message string, sender net.Conn) {
	mutex.Lock()
	defer mutex.Unlock()
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	formattedMessage := fmt.Sprintf("[%s] %s", timestamp, message)
	for conn, name := range clients {
		if conn != sender {
			_, err := conn.Write([]byte(formattedMessage + "\n"))
			if err != nil {
				log.Printf("Error broadcasting message to %s: %v", name, err)
			}
		}
	}
}