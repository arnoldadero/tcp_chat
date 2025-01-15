# tcp_chat

A simple TCP chat application written in Go.

## Overview

This application provides a basic TCP-based chat server and client. It allows multiple clients to connect to the server and exchange messages in real-time.

## Features

- **Multiple Client Support:** The server can handle multiple concurrent client connections.
- **Real-time Messaging:** Messages are broadcasted to all connected clients in real-time.
- **Usernames:** Clients are prompted to enter a username upon connection, which is used to identify their messages.
- **Message History:** New clients receive all previous chat messages upon joining.
- **Join/Leave Notifications:** Clients are notified when other users join or leave the chat.
- **Timestamped Messages:** Messages are displayed with the timestamp of when they were sent.
- **Private Messaging:** Users can send private messages to specific recipients using the `/msg <username> <message>` command.
- **User Listing:** Users can list all connected clients using the `/list` command.
- **Connection Limit:** The server has a maximum connection limit to prevent overload.
- **ASCII Art Logo:** A fun ASCII art logo is displayed upon client connection.
- **Protocol Validation:** Server validates client connections using CHAT/1.0 protocol handshake
- **Connection Management:** Automatic reconnection with timeout and retry logic
- **Keep-alive Mechanism:** Continuous connection monitoring with status updates
- **Message Size Limits:** Messages are limited to 1024 bytes to prevent abuse
- **Enhanced Error Handling:** Graceful handling of connection errors and server disconnections

## Protocol

The chat protocol uses a simple handshake mechanism:

1. Client connects to server
2. Client sends "CHAT/1.0\\n" as initial handshake
3. Server validates handshake and proceeds with connection

## Getting Started

### Prerequisites

- Go installed on your system.

### Running the Server

1. Clone the repository:
   ```bash
   git clone https://github.com/arnoldadero/tcp_chat.git
   cd tcp_chat
   ```

2. Build and run the server:
   ```bash
   go run main.go [port]
   ```
   If no port is specified, the default port `8989` will be used.

   Alternatively, you can build and run the server as a binary:
   ```bash
   go build -o tcp_chat_server
   ./tcp_chat_server [port]
   ```

3. (Optional) Run tests:
   ```bash
   go test ./...
   ```

### Running the Client

1. Navigate to the client directory.
2. Run the command `go run client.go <server_address> <port>` to connect to the server. Replace `<server_address>` with the IP address or hostname of the server and `<port>` with the port the server is listening on.

## Testing

The server includes comprehensive tests with over 85% coverage. To run tests and check coverage:

1. Run all tests:
   ```bash
   go test -v ./...
   ```

2. Generate test coverage report:
   ```bash
   go test -coverprofile=coverage.out ./...
   go tool cover -html=coverage.out
   ```

3. Check test coverage:
   ```bash
   go test -cover ./...
   ```

The test suite includes unit tests, integration tests, and end-to-end tests for all core functionalities.

## Open Issues

### High Priority
- Ensure all clients receive messages ([#15](https://github.com/arnoldadero/tcp_chat/issues/15))
- Handle client disconnections gracefully ([#16](https://github.com/arnoldadero/tcp_chat/issues/16))
- Control the maximum number of connections ([#10](https://github.com/arnoldadero/tcp_chat/issues/10))

### Medium Priority
- Respond with usage message for incorrect arguments ([#18](https://github.com/arnoldadero/tcp_chat/issues/18))
- Set default port if none is specified ([#17](https://github.com/arnoldadero/tcp_chat/issues/17))
- Inform clients about new joins and leaves ([#14](https://github.com/arnoldadero/tcp_chat/issues/14))
- Send previous messages to new clients ([#13](https://github.com/arnoldadero/tcp_chat/issues/13))

### Low Priority
- Identify messages with timestamp and sender's name ([#12](https://github.com/arnoldadero/tcp_chat/issues/12))
- Do not broadcast empty messages from a client ([#11](https://github.com/arnoldadero/tcp_chat/issues/11))
- Require client to provide a name upon connection ([#9](https://github.com/arnoldadero/tcp_chat/issues/9))
- Inform clients about new connections and disconnections ([#8](https://github.com/arnoldadero/tcp_chat/issues/8))
- Load previous messages for new clients ([#7](https://github.com/arnoldadero/tcp_chat/issues/7))
- Prompt client for name ([#6](https://github.com/arnoldadero/tcp_chat/issues/6))
- Display ASCII logo on client connection ([#5](https://github.com/arnoldadero/tcp_chat/issues/5))
- Implement basic client functionality (connecting, sending/receiving messages) ([#3](https://github.com/arnoldadero/tcp_chat/issues/3))

## Contributing

Contributions are welcome! Please feel free to submit pull requests or open issues to suggest improvements or report bugs.

## License

[LICENSE]