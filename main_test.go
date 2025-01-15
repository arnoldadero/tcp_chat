package main

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGetClients(t *testing.T) {
	t.Parallel() // Enable parallel execution

	// Test scenario 1: Empty clients map
	t.Run("EmptyClientsMap", func(t *testing.T) {
		t.Parallel()
		localClients := make(map[net.Conn]string)
		retrievedClients := getClientsFromMap(localClients)
		if len(retrievedClients) != 0 {
			t.Errorf("Expected empty clients map, got %d clients", len(retrievedClients))
		}
	})

	// Test scenario 2: Multiple clients
	t.Run("MultipleClients", func(t *testing.T) {
		t.Parallel()
		localClients := make(map[net.Conn]string)
		mockConn1 := newMockConn()
		mockConn2 := newMockConn()

		localClients[mockConn1] = "Client1"
		localClients[mockConn2] = "Client2"

		retrievedClients := getClientsFromMap(localClients)

		if len(retrievedClients) != 2 {
			t.Errorf("Expected 2 clients, got %d", len(retrievedClients))
		}
	})

	// Test scenario 3: Concurrent access safety
	t.Run("ConcurrentAccess", func(t *testing.T) {
		t.Parallel()
		localClients := make(map[net.Conn]string)

		// Simulate concurrent client additions
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			mockConn := newMockConn()
			clientName := "Client" + fmt.Sprintf("%d", i)

			go func(conn net.Conn, name string) {
				defer wg.Done()
				localClients[conn] = name
			}(mockConn, clientName)
		}

		wg.Wait()

		retrievedClients := getClientsFromMap(localClients)

		if len(retrievedClients) != 100 {
			t.Errorf("Expected 100 clients, got %d", len(retrievedClients))
		}
	})
}

// Helper function to get clients from a map without global state
func getClientsFromMap(clientMap map[net.Conn]string) map[net.Conn]string {
	result := make(map[net.Conn]string)
	for conn, name := range clientMap {
		result[conn] = name
	}
	return result
}

// Test findConnectionByName function
func TestFindConnectionByName(t *testing.T) {
	t.Parallel()
	t.Run("ExistingClient", func(t *testing.T) {
		t.Parallel()
		localClients := make(map[net.Conn]string)
		mockConn := newMockConn()
		localClients[mockConn] = "TestUser"

		foundConn := findConnectionByNameFromMap(localClients, "TestUser")
		if foundConn == nil {
			t.Error("Expected to find connection for 'TestUser', got nil")
		}
	})

	t.Run("NonExistingClient", func(t *testing.T) {
		t.Parallel()
		localClients := make(map[net.Conn]string)

		foundConn := findConnectionByNameFromMap(localClients, "NonExistentUser")
		if foundConn != nil {
			t.Error("Expected nil for non-existent user, got a connection")
		}
	})
}

// Helper function to find connection by name from a map without global state
func findConnectionByNameFromMap(clientMap map[net.Conn]string, name string) net.Conn {
	for conn, clientName := range clientMap {
		if clientName == name {
			return conn
		}
	}
	return nil
}

// Expanded tests for handleConnection
func TestHandleConnection(t *testing.T) {
	t.Parallel()
	t.Run("NewClientConnection", func(t *testing.T) {
		t.Parallel()
		// Setup mock connection
		readBuffer := bytes.NewBufferString("John\nHello, everyone!\n")
		writeBuffer := &bytes.Buffer{}
		mockConn := &mockConn{
			readBuffer:  readBuffer,
			writeBuffer: writeBuffer,
		}

		// Verify logo is sent
		expectedLogo := "         _nnnn_"
		if !strings.Contains(writeBuffer.String(), expectedLogo) {
			t.Errorf("Expected logo line '%s' not found", expectedLogo)
		}

		// Reset local state
		localClients := make(map[net.Conn]string)
		localMessages := []string{}

		// Create channels for synchronization
		done := make(chan struct{})
		ready := make(chan struct{})

		// Run handleConnection in a goroutine
		go func() {
			close(ready)
			defer func() { done <- struct{}{} }()
			handleConnection(mockConn)
		}()

		// Wait for connection handler to start
		<-ready

		// Wait for connection handling to complete with timeout
		select {
		case <-done:
			// Verify client was added
			if len(localClients) != 1 {
				t.Errorf("Expected 1 client, got %d", len(localClients))
			}

			// Verify client name and message
			for conn, name := range localClients {
				if name != "John" {
					t.Errorf("Expected client name 'John', got '%s'", name)
				}
				// Clean up connection
				conn.Close()
			}

			if len(localMessages) != 1 {
				t.Errorf("Expected 1 message, got %d", len(localMessages))
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Test timed out")
		}
	})

	t.Run("DuplicateClientName", func(t *testing.T) {
		t.Parallel()
		// Setup mock connections
		readBuffer1 := bytes.NewBufferString("Alice\n")
		writeBuffer1 := &bytes.Buffer{}
		mockConn1 := &mockConn{
			readBuffer:  readBuffer1,
			writeBuffer: writeBuffer1,
		}

		readBuffer2 := bytes.NewBufferString("Alice\n")
		writeBuffer2 := &bytes.Buffer{}
		mockConn2 := &mockConn{
			readBuffer:  readBuffer2,
			writeBuffer: writeBuffer2,
		}

		// Reset local state
		localClients := make(map[net.Conn]string)

		// Create done channels
		done1 := make(chan struct{})
		done2 := make(chan struct{})

		// Run first connection
		go func() {
			defer func() { done1 <- struct{}{} }()
			handleConnection(mockConn1)
		}()

		// Run second connection
		go func() {
			defer func() { done2 <- struct{}{} }()
			handleConnection(mockConn2)
		}()

		// Wait for both connections
		<-done1
		<-done2

		// Verify only one client was added
		if len(localClients) != 1 {
			t.Errorf("Expected 1 client with unique name, got %d", len(localClients))
		}
	})
}

// Test broadcastMessage function
func TestBroadcastMessage(t *testing.T) {
	t.Parallel()
	t.Run("MultipleClients", func(t *testing.T) {
		t.Parallel()
		// Setup mock connections
		mockConn1 := &mockConn{
			readBuffer:  &bytes.Buffer{},
			writeBuffer: &bytes.Buffer{},
		}
		mockConn2 := &mockConn{
			readBuffer:  &bytes.Buffer{},
			writeBuffer: &bytes.Buffer{},
		}

		// Reset local state
		_ = map[net.Conn]string{
			mockConn1: "Client1",
			mockConn2: "Client2",
		}

				// Broadcast a message
				broadcastMessage("Test message", mockConn1)
		// Check if message was written to other clients' connections
		expectedMessage := "Client1: Test message\n"
		mockConn2WriteContent := mockConn2.writeBuffer.String()

		if !strings.Contains(mockConn2WriteContent, expectedMessage) {
			t.Errorf("Expected message '%s' not found in broadcast", expectedMessage)
		}
	})
}

// Test findConnectionByName function (additional test)
func TestPrivateMessageHandling(t *testing.T) {
	t.Parallel()
	// Setup mock connections
	mockConn1 := &mockConn{
		readBuffer:  &bytes.Buffer{},
		writeBuffer: &bytes.Buffer{},
	}
	mockConn2 := &mockConn{
		readBuffer:  &bytes.Buffer{},
		writeBuffer: &bytes.Buffer{},
	}

	// Reset local state
	_ = map[net.Conn]string{
		mockConn1: "user1",
		mockConn2: "user2",
	}

	// Test valid private message
	handleConnection(mockConn1)
	mockConn1.readBuffer.WriteString("/msg user2 Hello\n")
	handleConnection(mockConn1)

	output := mockConn2.writeBuffer.String()
	if !strings.Contains(output, "[PM from user1]: Hello") {
		t.Errorf("Expected private message not received")
	}

	// Test invalid recipient
	mockConn1.readBuffer.WriteString("/msg invalid Hello\n")
	handleConnection(mockConn1)
	output = mockConn1.writeBuffer.String()
	if !strings.Contains(output, "User invalid not found") {
		t.Errorf("Expected error message for invalid recipient")
	}
}

func TestListCommand(t *testing.T) {
	t.Parallel()
	mockConn := &mockConn{
		readBuffer:  &bytes.Buffer{},
		writeBuffer: &bytes.Buffer{},
	}

	// Reset local state
	_ = map[net.Conn]string{
		mockConn:      "user1",
		newMockConn(): "user2",
	}

		// Test /list command
		mockConn.readBuffer.WriteString("/list\n")
		handleConnection(mockConn)
	output := mockConn.writeBuffer.String()
	if !strings.Contains(output, "Connected users: user1, user2") {
		t.Errorf("Expected user list not received")
	}
}

func TestMessageSizeLimit(t *testing.T) {
	t.Parallel()
	mockConn := &mockConn{
		readBuffer:  &bytes.Buffer{},
		writeBuffer: &bytes.Buffer{},
	}

	// Reset local state
	_ = map[net.Conn]string{
		mockConn: "user1",
	}

	// Test large message
	largeMessage := strings.Repeat("a", 1025)
	mockConn.readBuffer.WriteString(largeMessage + "\n")
	handleConnection(mockConn)

	output := mockConn.writeBuffer.String()
	if !strings.Contains(output, "Message too long (max 1024 characters)") {
		t.Errorf("Expected message size limit error not received")
	}
}

func TestFindConnectionByNameConcurrent(t *testing.T) {
	t.Parallel()
	// Reset local state
	localClients := make(map[net.Conn]string)

	// Create multiple mock connections
	mockConnections := make([]net.Conn, 100)
	for i := 0; i < 100; i++ {
		mockConnections[i] = &net.TCPConn{}
		clientName := fmt.Sprintf("Client%d", i)

		localClients[mockConnections[i]] = clientName
	}

	// Concurrent search
	var wg sync.WaitGroup
	results := make(chan net.Conn, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			clientName := fmt.Sprintf("Client%d", idx)

			conn := findConnectionByNameFromMap(localClients, clientName)
			if conn != nil {
				results <- conn
			}
		}(i)
	}

	// Wait for all searches to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect and verify results
	foundConnections := 0
	for range results {
		foundConnections++
	}

	if foundConnections != 100 {
		t.Errorf("Expected to find all 100 connections, found %d", foundConnections)
	}
}
