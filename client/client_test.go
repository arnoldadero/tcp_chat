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
			conn := newMockConn()
			conn.readBuffer = bytes.NewBufferString(tt.input)

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
				output := conn.writeBuffer.String()
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
	conn := newMockConn()

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
		{"Regular message", "[2025-01-15 18:00:00] user: Hello\n", "user: Hello"},
		{"User list", "Connected users:\nuser1\nuser2\n", "Connected users:\nuser1\nuser2\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock connection
			conn := newMockConn()
			conn.readBuffer = bytes.NewBufferString(tt.input)

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
			conn := newMockConn()
			conn.readBuffer = bytes.NewBufferString("CHAT/1.0\n")
			return conn, nil
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

func TestMessageSizeLimit(t *testing.T) {
	tests := []struct {
		name            string
		messageSize     int
		shouldBeSkipped bool
	}{
		{"Normal message", 50, false},
		{"Large message", 2000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock connection
			conn := newMockConn()
			conn.readBuffer = bytes.NewBufferString(strings.Repeat("x", tt.messageSize) + "\n")

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w
			defer func() {
				w.Close()
				os.Stdout = oldStdout
			}()

			// Run message handler
			done := make(chan struct{})
			go func() {
				defer close(done)
				handleIncomingMessages(conn)
			}()

			// Wait for message processing
			select {
			case <-done:
				w.Close()
				var buf bytes.Buffer
				buf.ReadFrom(r)
				output := buf.String()

				if tt.shouldBeSkipped {
					if !strings.Contains(output, "Message too large") {
						t.Errorf("Expected message size limit warning for large message")
					}
				} else {
					if output == "" {
						t.Errorf("Expected message to be processed")
					}
				}
			case <-time.After(2 * time.Second):
				t.Error("Test timed out")
			}
		})
	}
}

func TestPrivateMessageValidation(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedOutput string
	}{
		{"Valid private message", "/msg user Hello", "/msg user Hello\n"},
		{"Incomplete private message", "/msg user", "Invalid private message format"},
		{"Missing recipient", "/msg Hello", "Invalid private message format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock connection
			conn := newMockConn()
			conn.readBuffer = bytes.NewBufferString(tt.input + "\n")

			// Create scanner from input
			scanner := bufio.NewScanner(strings.NewReader(tt.input + "\n"))

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w
			defer func() {
				w.Close()
				os.Stdout = oldStdout
			}()

			// Run handleConnection
			done := make(chan struct{})
			go func() {
				handleConnection(conn, scanner)
				close(done)
			}()

			// Wait for processing
			select {
			case <-done:
				w.Close()
				var buf bytes.Buffer
				buf.ReadFrom(r)

				if tt.expectedOutput != "" {
					writtenOutput := conn.writeBuffer.String()
					if !strings.Contains(writtenOutput, tt.expectedOutput) {
						t.Errorf("Expected output to contain %q, got %q", tt.expectedOutput, writtenOutput)
					}
				}
			case <-time.After(2 * time.Second):
				t.Error("Test timed out")
			}
		})
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
