package main

import (
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

var serverProcess *os.Process
var testMutex sync.Mutex

func startServer(port string) {
	os.Args = []string{"", port}
	go main()
	time.Sleep(1 * time.Second) // Give server time to start
}

func stopServer() {
	if serverProcess != nil {
		serverProcess.Kill()
	}
}

func TestServerConnection(t *testing.T) {
	port := "8989"
	startServer(port)
	defer stopServer()

	// Try to connect to the server
	conn, err := net.Dial("tcp", ":"+port)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Send protocol header
	_, err = conn.Write([]byte("CHAT/1.0\n"))
	if err != nil {
		t.Fatalf("Failed to send protocol header: %v", err)
	}

	// Check if the server accepts the connection
	if conn == nil {
		t.Errorf("Server did not accept connection")
	}
}

func TestHandleConnection(t *testing.T) {
	port := "8990"
	startServer(port)
	defer stopServer()

	// Connect to the server
	conn, err := net.Dial("tcp", ":"+port)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Send protocol header
	_, err = conn.Write([]byte("CHAT/1.0\n"))
	if err != nil {
		t.Fatalf("Failed to send protocol header: %v", err)
	}

	// Send a name to the server
	testName := "testUser\n"
	_, err = conn.Write([]byte(testName))
	if err != nil {
		t.Fatalf("Failed to send name: %v", err)
	}

	time.Sleep(1 * time.Second)

	// Wait for server to process the connection
	time.Sleep(2 * time.Second)
	
	// Wait for server to process the connection
	time.Sleep(2 * time.Second)
	
	// Get a copy of the clients map
	clients := GetClients()
	
	// Check if the server received the name correctly
	found := false
	for _, name := range clients {
		if name == strings.TrimSpace(testName) {
			found = true
			break
		}
	}
	
	if !found {
		t.Errorf("Client not found in clients map")
	}
}

func TestMaxConnections(t *testing.T) {
	port := "8991"
	startServer(port)
	defer stopServer()

	var connections []net.Conn
	defer func() {
		for _, conn := range connections {
			conn.Close()
		}
	}()

	// Create maxConnections connections
	for i := 0; i < maxConnections; i++ {
		conn, err := net.Dial("tcp", ":"+port)
		if err != nil {
			t.Fatalf("Failed to connect to server: %v", err)
		}
		connections = append(connections, conn)
		
		// Send protocol header
		_, err = conn.Write([]byte("CHAT/1.0\n"))
		if err != nil {
			t.Fatalf("Failed to send protocol header: %v", err)
		}
	}

	// Try to create one more connection
	conn, err := net.Dial("tcp", ":"+port)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Send protocol header
	_, err = conn.Write([]byte("CHAT/1.0\n"))
	if err != nil {
		t.Fatalf("Failed to send protocol header: %v", err)
	}

	// Read server response
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read server response: %v", err)
	}

	response := string(buf[:n])
	if !strings.Contains(response, "Server is full") {
		t.Errorf("Expected server full message, got: %s", response)
	}
}

func TestInvalidProtocol(t *testing.T) {
	port := "8992"
	startServer(port)
	defer stopServer()

	conn, err := net.Dial("tcp", ":"+port)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Send invalid protocol
	_, err = conn.Write([]byte("INVALID/1.0\n"))
	if err != nil {
		t.Fatalf("Failed to send invalid protocol: %v", err)
	}

	// Read server response
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read server response: %v", err)
	}

	response := string(buf[:n])
	if !strings.Contains(response, "Invalid protocol") {
		t.Errorf("Expected invalid protocol message, got: %s", response)
	}
}

func TestEmptyName(t *testing.T) {
	port := "8993"
	startServer(port)
	defer stopServer()

	conn, err := net.Dial("tcp", ":"+port)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Send protocol header
	_, err = conn.Write([]byte("CHAT/1.0\n"))
	if err != nil {
		t.Fatalf("Failed to send protocol header: %v", err)
	}

	// Send empty name
	_, err = conn.Write([]byte("\n"))
	if err != nil {
		t.Fatalf("Failed to send empty name: %v", err)
	}

	// Read server response
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read server response: %v", err)
	}

	response := string(buf[:n])
	if !strings.Contains(response, "Name cannot be empty") {
		t.Errorf("Expected empty name message, got: %s", response)
	}
}

func TestClientDisconnect(t *testing.T) {
	port := "8994"
	startServer(port)
	defer stopServer()

	conn, err := net.Dial("tcp", ":"+port)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}

	// Send protocol header
	_, err = conn.Write([]byte("CHAT/1.0\n"))
	if err != nil {
		t.Fatalf("Failed to send protocol header: %v", err)
	}

	// Send name
	_, err = conn.Write([]byte("testUser\n"))
	if err != nil {
		t.Fatalf("Failed to send name: %v", err)
	}

	// Close connection abruptly
	conn.Close()

	// Wait for server to handle disconnect
	time.Sleep(1 * time.Second)

	// Verify client was removed from map
	testMutex.Lock()
	defer testMutex.Unlock()
	if _, ok := clients[conn]; ok {
		t.Errorf("Client was not removed from clients map after disconnect")
	}
}