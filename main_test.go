package main

import (
	"net"
	"sync"
	"testing"
)

var mutex sync.Mutex
var clients = make(map[net.Conn]string)

func TestGetClients(t *testing.T) {
	// Clear existing clients before test
	mutex.Lock()
	clients = make(map[net.Conn]string)
	mutex.Unlock()

	// Add some mock clients
	mockConn1 := &net.TCPConn{}
	mockConn2 := &net.TCPConn{}

	mutex.Lock()
	clients[mockConn1] = "Client1"
	clients[mockConn2] = "Client2"
	mutex.Unlock()

	// Test GetClients function
	retrievedClients := GetClients()

	if len(retrievedClients) != 2 {
		t.Errorf("Expected 2 clients, got %d", len(retrievedClients))
	}

	// Check if specific clients are present
	_, exists1 := retrievedClients[mockConn1]
	_, exists2 := retrievedClients[mockConn2]

	if !exists1 || !exists2 {
		t.Error("Expected mock clients not found in retrieved clients")
	}
}
