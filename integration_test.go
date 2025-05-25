package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/jpillora/ipfilter"
	"github.com/phires/go-guerrilla/log"
	"github.com/phires/go-guerrilla/mail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestLogger initializes the logger for testing.
func setupTestLogger(t *testing.T) {
	var err error
	Logger, err = log.GetLogger("stdout", "info")
	require.NoError(t, err)
}

func TestSendMail_Success(t *testing.T) {
	setupTestLogger(t)

	// Start mock SMTP server
	server := NewMockSMTPServer(t)
	require.NoError(t, server.Start())
	defer server.Stop()

	// Configure for testing
	config := &relayConfig{
		Server:        server.Address(),
		Port:          server.Port(),
		STARTTLS:      true,
		LoginAuthType: false,
		Username:      "",
		Password:      "",
		SkipVerify:    true,
		HeloHost:      "",
	}

	// Create test envelope
	envelope := &mail.Envelope{
		MailFrom: mail.Address{User: "sender", Host: "test.com"},
		RcptTo: []mail.Address{
			{User: "recipient1", Host: "example.com"},
			{User: "recipient2", Host: "example.com"},
		},
		Data:     *bytes.NewBufferString("Subject: Test\r\n\r\nThis is a test email."),
		RemoteIP: "127.0.0.1",
	}

	// Set up IP filter to allow this IP
	AllowedSendersFilter = ipfilter.New(ipfilter.Options{
		AllowedIPs:     []string{"127.0.0.1"},
		BlockByDefault: false,
	})

	// Send email
	err := sendMail(envelope, config)
	assert.NoError(t, err)

	// Verify the mock server received the email
	conn := server.GetLastConnection()
	require.NotNil(t, conn)
	assert.Equal(t, "sender@test.com", conn.From)
	assert.Equal(t, []string{"recipient1@example.com", "recipient2@example.com"}, conn.To)
	assert.Contains(t, conn.Data, "Subject: Test")
	assert.Contains(t, conn.Data, "This is a test email.")
}

func TestSendMail_WithAuthentication(t *testing.T) {
	setupTestLogger(t)
	// Start mock SMTP server with auth requirement
	server := NewMockSMTPServer(t)
	server.RequireAuth = true
	require.NoError(t, server.Start())
	defer server.Stop()

	// Configure with authentication
	config := &relayConfig{
		Server:        server.Address(),
		Port:          server.Port(),
		STARTTLS:      true,
		LoginAuthType: false,
		Username:      "testuser",
		Password:      "testpass",
		SkipVerify:    true,
		HeloHost:      "relay.test.com",
	}

	// Create test envelope
	envelope := &mail.Envelope{
		MailFrom: mail.Address{User: "sender", Host: "test.com"},
		RcptTo: []mail.Address{
			{User: "recipient", Host: "example.com"},
		},
		Data:     *bytes.NewBufferString("Subject: Auth Test\r\n\r\nAuthenticated email."),
		RemoteIP: "127.0.0.1",
	}

	// Allow IP
	AllowedSendersFilter = ipfilter.New(ipfilter.Options{
		BlockByDefault: false,
	})

	// Send email
	err := sendMail(envelope, config)
	assert.NoError(t, err)

	// Verify authentication was used
	conn := server.GetLastConnection()
	require.NotNil(t, conn)
	assert.NotEmpty(t, conn.AuthUser)
	assert.NotEmpty(t, conn.AuthPass)
}

func TestSendMail_WithLoginAuth(t *testing.T) {
	setupTestLogger(t)
	// Start mock SMTP server with LOGIN auth support
	server := NewMockSMTPServer(t)
	server.RequireAuth = true
	server.SupportLoginAuth = true
	require.NoError(t, server.Start())
	defer server.Stop()

	// Configure with LOGIN authentication
	config := &relayConfig{
		Server:        server.Address(),
		Port:          server.Port(),
		STARTTLS:      true,
		LoginAuthType: true,
		Username:      "testuser",
		Password:      "testpass",
		SkipVerify:    true,
		HeloHost:      "",
	}

	// Create test envelope
	envelope := &mail.Envelope{
		MailFrom: mail.Address{User: "sender", Host: "test.com"},
		RcptTo: []mail.Address{
			{User: "recipient", Host: "example.com"},
		},
		Data:     *bytes.NewBufferString("Subject: LOGIN Auth Test\r\n\r\nLOGIN authenticated email."),
		RemoteIP: "127.0.0.1",
	}

	// Allow IP
	AllowedSendersFilter = ipfilter.New(ipfilter.Options{
		BlockByDefault: false,
	})

	// Send email
	err := sendMail(envelope, config)
	assert.NoError(t, err)

	// Verify LOGIN authentication was used
	conn := server.GetLastConnection()
	require.NotNil(t, conn)
	assert.NotEmpty(t, conn.AuthUser)
	assert.NotEmpty(t, conn.AuthPass)
}

func TestSendMail_IPFiltering_Blocked(t *testing.T) {
	setupTestLogger(t)
	// Start mock SMTP server
	server := NewMockSMTPServer(t)
	require.NoError(t, server.Start())
	defer server.Stop()

	// Configure server
	config := &relayConfig{
		Server:        server.Address(),
		Port:          server.Port(),
		STARTTLS:      true,
		LoginAuthType: false,
		Username:      "",
		Password:      "",
		SkipVerify:    true,
		HeloHost:      "",
	}

	// Create test envelope with blocked IP
	envelope := &mail.Envelope{
		MailFrom: mail.Address{User: "sender", Host: "test.com"},
		RcptTo: []mail.Address{
			{User: "recipient", Host: "example.com"},
		},
		Data:     *bytes.NewBufferString("Subject: Test\r\n\r\nThis should be blocked."),
		RemoteIP: "192.168.1.100", // This IP will be blocked
	}

	// Set up IP filter to block this IP
	AllowedSendersFilter = ipfilter.New(ipfilter.Options{
		AllowedIPs:     []string{"127.0.0.1"},
		BlockByDefault: true,
	})

	// Send email - should fail due to IP filtering
	err := sendMail(envelope, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "192.168.1.100")
	assert.Contains(t, err.Error(), "not allowed to send email")

	// Verify no email was sent to the server
	conn := server.GetLastConnection()
	// The connection might be nil or have no data since the IP was blocked before SMTP
	if conn != nil {
		assert.Empty(t, conn.From)
	}
}

func TestSendMail_IPFiltering_Allowed(t *testing.T) {
	setupTestLogger(t)
	// Start mock SMTP server
	server := NewMockSMTPServer(t)
	require.NoError(t, server.Start())
	defer server.Stop()

	// Configure server
	config := &relayConfig{
		Server:        server.Address(),
		Port:          server.Port(),
		STARTTLS:      true,
		LoginAuthType: false,
		Username:      "",
		Password:      "",
		SkipVerify:    true,
		HeloHost:      "",
	}

	// Create test envelope with allowed IP
	envelope := &mail.Envelope{
		MailFrom: mail.Address{User: "sender", Host: "test.com"},
		RcptTo: []mail.Address{
			{User: "recipient", Host: "example.com"},
		},
		Data:     *bytes.NewBufferString("Subject: Test\r\n\r\nThis should be allowed."),
		RemoteIP: "192.168.1.100",
	}

	// Set up IP filter to allow this specific IP
	AllowedSendersFilter = ipfilter.New(ipfilter.Options{
		AllowedIPs:     []string{"192.168.1.0/24"},
		BlockByDefault: true,
	})

	// Send email - should succeed
	err := sendMail(envelope, config)
	assert.NoError(t, err)

	// Verify email was sent to the server
	conn := server.GetLastConnection()
	require.NotNil(t, conn)
	assert.Equal(t, "sender@test.com", conn.From)
	assert.Equal(t, []string{"recipient@example.com"}, conn.To)
}

func TestSendMail_ServerErrors(t *testing.T) {
	setupTestLogger(t)
	tests := []struct {
		name        string
		failCommand string
		expectError string
	}{
		{
			name:        "MAIL command fails",
			failCommand: "MAIL",
			expectError: "mail error",
		},
		{
			name:        "RCPT command fails",
			failCommand: "RCPT",
			expectError: "rcpt error",
		},
		{
			name:        "DATA command fails",
			failCommand: "DATA",
			expectError: "data error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Start mock SMTP server
			server := NewMockSMTPServer(t)
			server.FailCommands[tt.failCommand] = true
			require.NoError(t, server.Start())
			defer server.Stop()

			// Configure server
			config := &relayConfig{
				Server:        server.Address(),
				Port:          server.Port(),
				STARTTLS:      true,
				LoginAuthType: false,
				Username:      "",
				Password:      "",
				SkipVerify:    true,
				HeloHost:      "",
			}

			// Create test envelope
			envelope := &mail.Envelope{
				MailFrom: mail.Address{User: "sender", Host: "test.com"},
				RcptTo: []mail.Address{
					{User: "recipient", Host: "example.com"},
				},
				Data:     *bytes.NewBufferString("Subject: Test\r\n\r\nThis should fail."),
				RemoteIP: "127.0.0.1",
			}

			// Allow IP
			AllowedSendersFilter = ipfilter.New(ipfilter.Options{
				BlockByDefault: false,
			})

			// Send email - should fail
			err := sendMail(envelope, config)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

func TestSendMail_ConnectionTimeout(t *testing.T) {
	setupTestLogger(t)
	// Start mock SMTP server with delay
	server := NewMockSMTPServer(t)
	server.ResponseDelay = 100 * time.Millisecond
	require.NoError(t, server.Start())
	defer server.Stop()

	// Configure server
	config := &relayConfig{
		Server:        server.Address(),
		Port:          server.Port(),
		STARTTLS:      true,
		LoginAuthType: false,
		Username:      "",
		Password:      "",
		SkipVerify:    true,
		HeloHost:      "",
	}

	// Create test envelope
	envelope := &mail.Envelope{
		MailFrom: mail.Address{User: "sender", Host: "test.com"},
		RcptTo: []mail.Address{
			{User: "recipient", Host: "example.com"},
		},
		Data:     *bytes.NewBufferString("Subject: Timeout Test\r\n\r\nThis tests server delays."),
		RemoteIP: "127.0.0.1",
	}

	// Allow IP
	AllowedSendersFilter = ipfilter.New(ipfilter.Options{
		BlockByDefault: false,
	})

	// Send email - should still succeed despite delay
	err := sendMail(envelope, config)
	assert.NoError(t, err)

	// Verify email was eventually sent
	conn := server.GetLastConnection()
	require.NotNil(t, conn)
	assert.Equal(t, "sender@test.com", conn.From)
}
