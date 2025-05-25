package main

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// MockSMTPServer represents a mock SMTP server for testing
type MockSMTPServer struct {
	listener  net.Listener
	tlsConfig *tls.Config
	address   string
	port      int
	running   bool
	mu        sync.Mutex

	// Recorded interactions
	Connections []MockConnection

	// Configuration
	RequireAuth      bool
	RequireSTARTTLS  bool
	SupportLoginAuth bool
	ResponseDelay    time.Duration
	FailCommands     map[string]bool // Commands to fail
	CustomResponses  map[string]string
	ImplicitTLS      bool // True if server uses implicit TLS (like port 465)
}

type MockConnection struct {
	Commands []string
	From     string
	To       []string
	Data     string
	AuthUser string
	AuthPass string
	UsedTLS  bool
}

// NewMockSMTPServer creates a new mock SMTP server
func NewMockSMTPServer(t *testing.T) *MockSMTPServer {
	cert, err := generateTestCert()
	if err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}

	return &MockSMTPServer{
		tlsConfig:       tlsConfig,
		Connections:     make([]MockConnection, 0),
		FailCommands:    make(map[string]bool),
		CustomResponses: make(map[string]string),
	}
}

// Start starts the mock SMTP server
func (s *MockSMTPServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}

	s.listener = listener
	s.address = listener.Addr().(*net.TCPAddr).IP.String()
	s.port = listener.Addr().(*net.TCPAddr).Port
	s.running = true

	go s.acceptConnections()

	// Give the server a moment to start
	time.Sleep(10 * time.Millisecond)
	return nil
}

// StartTLS starts the mock SMTP server with implicit TLS
func (s *MockSMTPServer) StartTLS() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}

	tlsListener := tls.NewListener(listener, s.tlsConfig)
	s.listener = tlsListener
	s.address = listener.Addr().(*net.TCPAddr).IP.String()
	s.port = listener.Addr().(*net.TCPAddr).Port
	s.running = true
	s.ImplicitTLS = true

	go s.acceptConnections()

	// Give the server a moment to start
	time.Sleep(10 * time.Millisecond)
	return nil
}

// Stop stops the mock SMTP server
func (s *MockSMTPServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener != nil {
		s.listener.Close()
		s.running = false
	}
}

// Address returns the server address
func (s *MockSMTPServer) Address() string {
	return s.address
}

// Port returns the server port
func (s *MockSMTPServer) Port() int {
	return s.port
}

// GetLastConnection returns the most recent connection
func (s *MockSMTPServer) GetLastConnection() *MockConnection {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.Connections) == 0 {
		return nil
	}
	return &s.Connections[len(s.Connections)-1]
}

// Reset clears all recorded connections
func (s *MockSMTPServer) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Connections = make([]MockConnection, 0)
}

func (s *MockSMTPServer) acceptConnections() {
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.running {
				fmt.Printf("Accept error: %v\n", err)
			}
			continue
		}

		go s.handleConnection(conn)
	}
}

func (s *MockSMTPServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	if s.ResponseDelay > 0 {
		time.Sleep(s.ResponseDelay)
	}

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	mockConn := MockConnection{
		Commands: make([]string, 0),
		To:       make([]string, 0),
	}

	// Check if this is a TLS connection (implicit TLS or post-STARTTLS)
	if _, ok := conn.(*tls.Conn); ok || s.ImplicitTLS {
		mockConn.UsedTLS = true
	}

	// Send greeting
	writer.WriteString("220 mock.smtp.server ESMTP ready\r\n")
	writer.Flush()

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		line = strings.TrimSpace(line)
		mockConn.Commands = append(mockConn.Commands, line)

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		cmd := strings.ToUpper(parts[0])

		// Check if we should fail this command
		if s.FailCommands[cmd] {
			writer.WriteString("550 Command failed\r\n")
			writer.Flush()
			continue
		}

		// Check for custom responses
		if response, exists := s.CustomResponses[cmd]; exists {
			writer.WriteString(response + "\r\n")
			writer.Flush()
			continue
		}

		switch cmd {
		case "EHLO", "HELO":
			s.handleEHLO(writer)
		case "STARTTLS":
			tlsConn, newReader, newWriter, upgraded := s.handleSTARTTLS(conn, reader, writer, &mockConn)
			if upgraded {
				// Connection was upgraded to TLS, switch to new connection
				conn = tlsConn
				reader = newReader
				writer = newWriter
			}
		case "AUTH":
			s.handleAUTH(parts, reader, writer, &mockConn)
		case "MAIL":
			s.handleMAIL(parts, writer, &mockConn)
		case "RCPT":
			s.handleRCPT(parts, writer, &mockConn)
		case "DATA":
			s.handleDATA(reader, writer, &mockConn)
		case "QUIT":
			writer.WriteString("221 Bye\r\n")
			writer.Flush()
			s.mu.Lock()
			s.Connections = append(s.Connections, mockConn)
			s.mu.Unlock()
			return
		default:
			writer.WriteString("500 Command not recognized\r\n")
			writer.Flush()
		}
	}

	s.mu.Lock()
	s.Connections = append(s.Connections, mockConn)
	s.mu.Unlock()
}

func (s *MockSMTPServer) handleEHLO(writer *bufio.Writer) {
	writer.WriteString("250-mock.smtp.server\r\n")
	if s.RequireSTARTTLS {
		writer.WriteString("250-STARTTLS\r\n")
	}
	if s.RequireAuth {
		if s.SupportLoginAuth {
			writer.WriteString("250-AUTH PLAIN LOGIN\r\n")
		} else {
			writer.WriteString("250-AUTH PLAIN\r\n")
		}
	}
	writer.WriteString("250 SIZE 10240000\r\n")
	writer.Flush()
}

func (s *MockSMTPServer) handleSTARTTLS(conn net.Conn, reader *bufio.Reader, writer *bufio.Writer, mockConn *MockConnection) (*tls.Conn, *bufio.Reader, *bufio.Writer, bool) {
	writer.WriteString("220 Ready to start TLS\r\n")
	writer.Flush()

	// Upgrade the connection to TLS
	tlsConn := tls.Server(conn, s.tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		// TLS handshake failed, return original connection
		return nil, reader, writer, false
	}

	mockConn.UsedTLS = true

	// Return new TLS connection and readers/writers
	newReader := bufio.NewReader(tlsConn)
	newWriter := bufio.NewWriter(tlsConn)

	return tlsConn, newReader, newWriter, true
}

func (s *MockSMTPServer) handleAUTH(parts []string, reader *bufio.Reader, writer *bufio.Writer, mockConn *MockConnection) {
	if len(parts) < 2 {
		writer.WriteString("501 Syntax error\r\n")
		writer.Flush()
		return
	}

	authType := strings.ToUpper(parts[1])

	switch authType {
	case "PLAIN":
		// PLAIN auth can be sent in initial command or as a response to challenge
		if len(parts) > 2 {
			// Credentials provided in initial command
			// authData := parts[2] // In a real implementation, we'd decode base64 and parse username/password
			mockConn.AuthUser = "testuser"
			mockConn.AuthPass = "testpass"

			writer.WriteString("235 Authentication successful\r\n")
			writer.Flush()
		} else {
			// Challenge/response mode
			writer.WriteString("334 \r\n")
			writer.Flush()

			authData, _ := reader.ReadString('\n')
			authData = strings.TrimSpace(authData)
			// In a real implementation, we'd decode base64 and parse username/password
			mockConn.AuthUser = "testuser"
			mockConn.AuthPass = "testpass"

			writer.WriteString("235 Authentication successful\r\n")
			writer.Flush()
		}

	case "LOGIN":
		writer.WriteString("334 VXNlcm5hbWU6\r\n") // "Username:" in base64
		writer.Flush()

		username, _ := reader.ReadString('\n')
		mockConn.AuthUser = strings.TrimSpace(username)

		writer.WriteString("334 UGFzc3dvcmQ6\r\n") // "Password:" in base64
		writer.Flush()

		password, _ := reader.ReadString('\n')
		mockConn.AuthPass = strings.TrimSpace(password)

		writer.WriteString("235 Authentication successful\r\n")
		writer.Flush()

	default:
		writer.WriteString("504 Authentication mechanism not supported\r\n")
		writer.Flush()
	}
}

func (s *MockSMTPServer) handleMAIL(parts []string, writer *bufio.Writer, mockConn *MockConnection) {
	if len(parts) < 2 {
		writer.WriteString("501 Syntax error\r\n")
		writer.Flush()
		return
	}

	fromAddr := strings.TrimPrefix(parts[1], "FROM:")
	fromAddr = strings.Trim(fromAddr, "<>")
	mockConn.From = fromAddr

	writer.WriteString("250 OK\r\n")
	writer.Flush()
}

func (s *MockSMTPServer) handleRCPT(parts []string, writer *bufio.Writer, mockConn *MockConnection) {
	if len(parts) < 2 {
		writer.WriteString("501 Syntax error\r\n")
		writer.Flush()
		return
	}

	toAddr := strings.TrimPrefix(parts[1], "TO:")
	toAddr = strings.Trim(toAddr, "<>")
	mockConn.To = append(mockConn.To, toAddr)

	writer.WriteString("250 OK\r\n")
	writer.Flush()
}

func (s *MockSMTPServer) handleDATA(reader *bufio.Reader, writer *bufio.Writer, mockConn *MockConnection) {
	writer.WriteString("354 Start mail input; end with <CRLF>.<CRLF>\r\n")
	writer.Flush()

	var dataBuilder strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		if strings.TrimSpace(line) == "." {
			break
		}

		dataBuilder.WriteString(line)
	}

	mockConn.Data = dataBuilder.String()

	writer.WriteString("250 OK: message accepted\r\n")
	writer.Flush()
}

// generateTestCert creates a self-signed certificate for testing
func generateTestCert() (tls.Certificate, error) {
	// Generate a private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"Test City"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		DNSNames:    []string{"localhost"},
	}

	// Generate the certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Create the tls.Certificate
	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  priv,
	}, nil
}
