package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// Mock connection for testing
type mockAddr struct {
	network string
	address string
}

func (m mockAddr) Network() string { return m.network }
func (m mockAddr) String() string  { return m.address }

type mockConn struct {
	readBuffer  *bytes.Buffer
	writeBuffer *bytes.Buffer
	closed      bool
	localAddr   mockAddr
	remoteAddr  mockAddr
}

func newMockConn() *mockConn {
	return &mockConn{
		readBuffer:  &bytes.Buffer{},
		writeBuffer: &bytes.Buffer{},
		localAddr:   mockAddr{network: "tcp", address: "127.0.0.1:0"},
		remoteAddr:  mockAddr{network: "tcp", address: "127.0.0.1:0"},
	}
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	if m.closed {
		return 0, io.EOF
	}
	return m.readBuffer.Read(b)
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	if m.closed {
		return 0, io.EOF
	}
	return m.writeBuffer.Write(b)
}

func (m *mockConn) Close() error {
	if m.closed {
		return io.EOF
	}
	m.closed = true
	return nil
}

func (m *mockConn) LocalAddr() net.Addr {
	return m.localAddr
}

func (m *mockConn) RemoteAddr() net.Addr {
	return m.remoteAddr
}

func (m *mockConn) SetDeadline(t time.Time) error {
	if m.closed {
		return io.EOF
	}
	return nil
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	if m.closed {
		return io.EOF
	}
	return nil
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	if m.closed {
		return io.EOF
	}
	return nil
}

const (
	DefaultPort    = "8989"
	maxConnections = 10
)

var (
	clients   = make(map[net.Conn]string) // Map to store client connections and names
	mutex     sync.Mutex                  // Mutex to protect access to the clients map
	messages  []string                    // Slice to store chat messages
	connCount int                         // Counter for active connections
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
	// Check if this is a mock connection
	mockConn, isMock := conn.(*mockConn)
	var reader *bufio.Reader
	if isMock {
		reader = bufio.NewReader(mockConn.readBuffer)
	} else {
		reader = bufio.NewReader(conn)
	}

	// Send welcome messages
	if !isMock {
		_, err := conn.Write([]byte("Welcome to TCP-Chat!\n"))
		if err != nil {
			log.Printf("Error sending welcome message: %v", err)
			return
		}
	}

	// Send the ASCII art logo with proper timing
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

	// Send logo with slight delay between lines for proper rendering
	for _, line := range logo {
		_, err := conn.Write([]byte(line + "\n"))
		if err != nil {
			log.Printf("Error sending logo line: %v", err)
			return
		}
		time.Sleep(50 * time.Millisecond) // Add slight delay between lines
	}
	
	// Add extra newline after logo for better spacing
	conn.Write([]byte("\n"))

	// Prompt for the client's name
	var err error
	if !isMock {
		_, err = conn.Write([]byte("[ENTER YOUR NAME]: "))
		if err != nil {
			log.Printf("Error sending name prompt: %v", err)
			return
		}
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

	// Check for duplicate names and add client
	mutex.Lock()
	defer mutex.Unlock()
	
	// First check if connection already exists
	if _, exists := clients[conn]; exists {
		return
	}

	// Check for duplicate names
	for _, name := range clients {
		if name == clientName {
			_, err := conn.Write([]byte("Name is already in use. Please choose a different name.\n"))
			if err != nil {
				log.Printf("Error sending duplicate name message: %v", err)
			}
			conn.Close()
			return
		}
	}
	
	// Add client to map
	clients[conn] = clientName

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
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			// Handle client disconnection
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Continue on timeout
				conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				continue
			}
			mutex.Lock()
			delete(clients, conn)
			mutex.Unlock()
			broadcastMessage(fmt.Sprintf("%s has left our chat...", clientName), conn)
			log.Printf("Client disconnected: %s", clientName)
			return
		}

		message = strings.TrimSpace(message)
		if message == "" {
			continue
		}

		// Handle private messages
		if strings.HasPrefix(message, "/msg ") {
			parts := strings.SplitN(message, " ", 3)
			if len(parts) == 3 {
				recipient := parts[1]
				privateMessage := parts[2]
				if targetConn := findConnectionByName(recipient); targetConn != nil {
					privateMsg := fmt.Sprintf("[PM from %s]: %s", clientName, privateMessage)
					targetConn.Write([]byte(privateMsg + "\n"))
					conn.Write([]byte(fmt.Sprintf("[PM to %s]: %s\n", recipient, privateMessage)))
					continue
				} else {
					conn.Write([]byte(fmt.Sprintf("User %s not found\n", recipient)))
					continue
				}
			}
		}

		// Handle /list command
		if message == "/list" {
			mutex.Lock()
			var userList []string
			for _, name := range clients {
				userList = append(userList, name)
			}
			mutex.Unlock()
			conn.Write([]byte(fmt.Sprintf("Connected users: %s\n", strings.Join(userList, ", "))))
			continue
		}

		// Enforce message size limit
		if len(message) > 1024 {
			conn.Write([]byte("Message too long (max 1024 characters)\n"))
			continue
		}

		// Broadcast regular message
		fullMessage := fmt.Sprintf("%s: %s", clientName, message)
		messages = append(messages, fullMessage)
		broadcastMessage(fullMessage, conn)
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
	clientsCopy := make(map[net.Conn]string)
	for k, v := range clients {
		clientsCopy[k] = v
	}
	mutex.Unlock()

	for conn, name := range clientsCopy {
		if conn != sender {
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_, err := conn.Write([]byte(message + "\n"))
			if err != nil {
				log.Printf("Error broadcasting message to %s: %v", name, err)
				// Remove disconnected client
				mutex.Lock()
				delete(clients, conn)
				mutex.Unlock()
			}
		}
	}
}
