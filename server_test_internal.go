package main_test

import (
	"fmt"
	"net"
	"testing"
	"time"
    "tcp_chat"
)

func TestServerConnection(t *testing.T) {
	// Start the server in a separate Goroutine
	go tcp_chat.main()

	// Give the server some time to start
	time.Sleep(1 * time.Second)

	// Try to connect to the server
	conn, err := net.Dial("tcp", ":"+tcp_chat.defaultPort)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Check if the server accepts the connection
	if conn == nil {
		t.Errorf("Server did not accept connection")
	}
}

func TestHandleConnection(t *testing.T) {
	// Start the server in a separate Goroutine
	go tcp_chat.main()

	// Give the server some time to start
	time.Sleep(1 * time.Second)

	// Connect to the server
	conn, err := net.Dial("tcp", ":"+tcp_chat.defaultPort)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Send a name to the server
	testName := "testUser"
	fmt.Fprintf(conn, "%s\n", testName)

    time.Sleep(1 * time.Second)

	// Check if the server received the name correctly
    if _, ok := tcp_chat.clients[conn]; !ok {
        t.Errorf("Client not found in clients map")
    } else if tcp_chat.clients[conn] != testName {
		t.Errorf("Server did not receive name correctly. Expected: %s, Got: %s", testName, tcp_chat.clients[conn])
	}
}