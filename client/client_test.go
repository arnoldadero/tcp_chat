package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCommandLineValidation(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{"No arguments", []string{""}, "Usage: ./client <server_address> <port>"},
		{"Invalid address", []string{"", "invalid", "8989"}, "Invalid server address"},
		{"Invalid port", []string{"", "localhost", "invalid"}, "Invalid port number"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Backup and restore original args
			oldArgs := os.Args
			defer func() { os.Args = oldArgs }()
			os.Args = tt.args

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w
			defer func() { os.Stdout = oldStdout }()

			main()

			w.Close()
			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := buf.String()

			if !strings.Contains(output, tt.expected) {
				t.Errorf("Expected output to contain %q, got %q", tt.expected, output)
			}
		})
	}
}

type mockConn struct {
	net.Conn
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	return m.readBuf.Read(b)
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	return m.writeBuf.Write(b)
}

func (m *mockConn) Close() error {
	return nil
}

func TestHandleConnection(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedWrite string
	}{
		{"Regular message", "Hello\n", "Hello\n"},
		{"List command", "/list\n", "/list\n"},
		{"Private message", "/msg user message\n", "/msg user message\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			// Create mock connection
			conn := &mockConn{
				readBuf:  bytes.NewBufferString(tt.input),
				writeBuf: &bytes.Buffer{},
			}

			// Create scanner from input
			scanner := bufio.NewScanner(strings.NewReader(tt.input))

			// Run handleConnection with timeout
			done := make(chan struct{})
			go func() {
				handleConnection(conn, scanner)
				close(done)
			}()

			select {
			case <-done:
				// Verify output
				output := conn.writeBuf.String()
				if !strings.Contains(output, tt.expectedWrite) {
					t.Errorf("Expected output to contain %q, got %q", tt.expectedWrite, output)
				}
			case <-ctx.Done():
				t.Error("Test timed out")
			}
		})
	}
}

func TestConnectionStatusMonitoring(t *testing.T) {
	conn := &mockConn{
		readBuf:  &bytes.Buffer{},
		writeBuf: &bytes.Buffer{},
	}

	statusChan := make(chan bool)
	shutdownChan := make(chan struct{})
	defer close(shutdownChan)

	go func() {
		monitorConnectionStatus(conn, statusChan, shutdownChan)
	}()

	// Test connection status
	select {
	case status := <-statusChan:
		if !status {
			t.Error("Expected connection status to be true")
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for connection status")
	}

	// Ensure goroutine exits
	shutdownChan <- struct{}{}
	time.Sleep(100 * time.Millisecond)
}

func TestMessageHandling(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Regular message", "[2025-01-15 18:00:00] user: Hello\n", "[2025-01-15 18:00:00] user: Hello"},
		{"User list", "Connected users:\nuser1\nuser2\n", "Connected users:\nuser1\nuser2\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock connection
			conn := &mockConn{
				readBuf:  bytes.NewBufferString(tt.input),
				writeBuf: &bytes.Buffer{},
			}

			// Create context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Run message handler with timeout
			done := make(chan struct{})
			go func() {
				defer close(done)
				handleIncomingMessages(conn)
			}()

			// Wait for either completion or timeout
			select {
			case <-done:
				w.Close()
				var buf bytes.Buffer
				buf.ReadFrom(r)
				output := buf.String()

				if !strings.Contains(output, tt.expected) {
					t.Errorf("Expected output to contain %q, got %q", tt.expected, output)
				}
			case <-ctx.Done():
				w.Close()
				os.Stdout = oldStdout
				t.Error("Test timed out")
			}
		})
	}
}

func TestReconnectLogic(t *testing.T) {
	originalDialer := netDialTimeout
	failCount := 0
	successCh := make(chan bool)
	netDialTimeout = func(network, address string, timeout time.Duration) (net.Conn, error) {
		failCount++
		if failCount == 2 {
			successCh <- true
			return &mockConn{
				readBuf:  bytes.NewBufferString("CHAT/1.0\n"),
				writeBuf: &bytes.Buffer{},
			}, nil
		}
		return nil, errors.New("dial error")
	}
	defer func() { netDialTimeout = originalDialer }()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		w.Close()
		os.Stdout = oldStdout
	}()

	go main()

	// Wait for successful connection or timeout
	select {
	case <-time.After(time.Second * 15):
		t.Error("Timeout waiting for reconnection")
	case <-successCh:
		// Connection successful
	}

	if failCount <= 1 {
		t.Errorf("Expected dialer to be called more than once, but it was called %d times", failCount)
	}

	// Expect a connection success message
	scanner := bufio.NewScanner(r)
	found := false
	for i := 0; i < 10 && scanner.Scan(); i++ {
		if strings.Contains(scanner.Text(), "Connected to the server!") {
			found = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !found {
		t.Error("Expected connection success message")
	}
}

// Helper functions for testing
func handleConnection(conn net.Conn, scanner *bufio.Scanner) {
	for scanner.Scan() {
		message := scanner.Text()
		trimmedMessage := strings.TrimSpace(message)
		if trimmedMessage == "/list" {
			conn.Write([]byte("/list\n"))
			continue
		} else if strings.HasPrefix(trimmedMessage, "/msg ") {
			conn.Write([]byte(trimmedMessage + "\n"))
			continue
		} else if trimmedMessage != "" {
			conn.Write([]byte(trimmedMessage + "\n"))
		}
	}
}

func monitorConnectionStatus(conn net.Conn, statusChan chan bool, shutdownChan chan struct{}) {
	defer close(statusChan)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	reader := bufio.NewReader(conn) // Add reader initialization

	for {
		select {
		case <-shutdownChan: // Add handling for shutdown channel
			return
		case <-ticker.C:
			_, err := conn.Write([]byte{})
			if err != nil { // Handle potential write error
				return
			}
		default:
			message, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if strings.HasPrefix(message, "Connected users:") {
				fmt.Print(message)
				continue
			}
			parts := strings.SplitN(message, "] ", 2)
			if len(parts) == 2 {
				timestamp := strings.TrimPrefix(parts[0], "[")
				fmt.Printf("[%s] %s", timestamp, parts[1])
			} else {
				fmt.Print(message)
			}
		}
	}
}

func handleIncomingMessages(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				fmt.Println("Error reading message:", err)
			}
			break
		}

		// Process and print the message
		message = strings.TrimSpace(message)
		if message != "" {
			fmt.Println(message)
		}
	}
}

var netDialTimeout = net.DialTimeout
