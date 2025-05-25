package main

import (
	"bytes"
	"testing"

	"github.com/flashmob/go-guerrilla/mail"
	"github.com/jpillora/ipfilter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendMail_STARTTLS(t *testing.T) {
	setupTestLogger(t)
	// Start mock SMTP server with STARTTLS requirement
	server := NewMockSMTPServer(t)
	server.RequireSTARTTLS = true
	require.NoError(t, server.Start())
	defer server.Stop()

	// Configure for STARTTLS
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
		Data:     *bytes.NewBufferString("Subject: STARTTLS Test\r\n\r\nThis tests STARTTLS."),
		RemoteIP: "127.0.0.1",
	}

	// Allow IP
	AllowedSendersFilter = ipfilter.New(ipfilter.Options{
		BlockByDefault: false,
	})

	// Send email
	err := sendMail(envelope, config)
	assert.NoError(t, err)

	// Verify the connection was established
	conn := server.GetLastConnection()
	require.NotNil(t, conn)
	assert.Equal(t, "sender@test.com", conn.From)
	assert.Equal(t, []string{"recipient@example.com"}, conn.To)

	// Verify STARTTLS was used (check commands include STARTTLS)
	starttlsFound := false
	for _, cmd := range conn.Commands {
		if cmd == "STARTTLS" {
			starttlsFound = true
			break
		}
	}
	assert.True(t, starttlsFound, "STARTTLS command should have been sent")
	assert.True(t, conn.UsedTLS, "Connection should be marked as using TLS")
}

func TestSendMail_ImplicitTLS(t *testing.T) {
	setupTestLogger(t)
	// Start mock SMTP server with implicit TLS
	server := NewMockSMTPServer(t)
	require.NoError(t, server.StartTLS())
	defer server.Stop()

	// Configure for implicit TLS (no STARTTLS)
	config := &relayConfig{
		Server:        server.Address(),
		Port:          server.Port(),
		STARTTLS:      false,
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
		Data:     *bytes.NewBufferString("Subject: Implicit TLS Test\r\n\r\nThis tests implicit TLS."),
		RemoteIP: "127.0.0.1",
	}

	// Allow IP
	AllowedSendersFilter = ipfilter.New(ipfilter.Options{
		BlockByDefault: false,
	})

	// Send email
	err := sendMail(envelope, config)
	assert.NoError(t, err)

	// Verify the connection was established
	conn := server.GetLastConnection()
	require.NotNil(t, conn)
	assert.Equal(t, "sender@test.com", conn.From)
	assert.Equal(t, []string{"recipient@example.com"}, conn.To)

	// Verify no STARTTLS command was sent (since we're using implicit TLS)
	starttlsFound := false
	for _, cmd := range conn.Commands {
		if cmd == "STARTTLS" {
			starttlsFound = true
			break
		}
	}
	assert.False(t, starttlsFound, "STARTTLS command should not have been sent for implicit TLS")
}

func TestSendMail_TLSWithAuthentication(t *testing.T) {
	setupTestLogger(t)
	// Start mock SMTP server with both TLS and authentication
	server := NewMockSMTPServer(t)
	server.RequireSTARTTLS = true
	server.RequireAuth = true
	require.NoError(t, server.Start())
	defer server.Stop()

	// Configure for STARTTLS with authentication
	config := &relayConfig{
		Server:        server.Address(),
		Port:          server.Port(),
		STARTTLS:      true,
		LoginAuthType: false,
		Username:      "tlsuser",
		Password:      "tlspass",
		SkipVerify:    true,
		HeloHost:      "secure.relay.com",
	}

	// Create test envelope
	envelope := &mail.Envelope{
		MailFrom: mail.Address{User: "sender", Host: "test.com"},
		RcptTo: []mail.Address{
			{User: "recipient", Host: "example.com"},
		},
		Data:     *bytes.NewBufferString("Subject: TLS + Auth Test\r\n\r\nThis tests TLS with authentication."),
		RemoteIP: "127.0.0.1",
	}

	// Allow IP
	AllowedSendersFilter = ipfilter.New(ipfilter.Options{
		BlockByDefault: false,
	})

	// Send email
	err := sendMail(envelope, config)
	assert.NoError(t, err)

	// Verify the connection was established with both TLS and auth
	conn := server.GetLastConnection()
	require.NotNil(t, conn)
	assert.Equal(t, "sender@test.com", conn.From)
	assert.Equal(t, []string{"recipient@example.com"}, conn.To)
	assert.True(t, conn.UsedTLS, "Connection should use TLS")
	assert.NotEmpty(t, conn.AuthUser, "Authentication should have been used")
	assert.NotEmpty(t, conn.AuthPass, "Authentication should have been used")

	// Verify command sequence (STARTTLS should come before AUTH)
	starttlsIndex := -1
	authIndex := -1
	for i, cmd := range conn.Commands {
		if cmd == "STARTTLS" {
			starttlsIndex = i
		}
		if len(cmd) >= 4 && cmd[:4] == "AUTH" {
			authIndex = i
		}
	}
	assert.True(t, starttlsIndex >= 0, "STARTTLS command should be present")
	assert.True(t, authIndex >= 0, "AUTH command should be present")
	assert.True(t, starttlsIndex < authIndex, "STARTTLS should come before AUTH")
}

func TestSendMail_TLSSkipVerify(t *testing.T) {
	setupTestLogger(t)
	// Test that we can handle certificate verification settings
	server := NewMockSMTPServer(t)
	require.NoError(t, server.StartTLS()) // Use implicit TLS
	defer server.Stop()

	tests := []struct {
		name       string
		skipVerify bool
	}{
		{"skip certificate verification", true},
		{"enforce certificate verification", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server.Reset()

			config := &relayConfig{
				Server:        server.Address(),
				Port:          server.Port(),
				STARTTLS:      false,
				LoginAuthType: false,
				Username:      "",
				Password:      "",
				SkipVerify:    tt.skipVerify,
				HeloHost:      "",
			}

			envelope := &mail.Envelope{
				MailFrom: mail.Address{User: "sender", Host: "test.com"},
				RcptTo: []mail.Address{
					{User: "recipient", Host: "example.com"},
				},
				Data:     *bytes.NewBufferString("Subject: TLS Verify Test\r\n\r\nTesting certificate verification."),
				RemoteIP: "127.0.0.1",
			}

			// Allow IP
			AllowedSendersFilter = ipfilter.New(ipfilter.Options{
				BlockByDefault: false,
			})

			// Send email
			err := sendMail(envelope, config)
			
			if tt.skipVerify {
				// Should succeed when skipping verification
				assert.NoError(t, err)
				
				// Verify email was sent
				conn := server.GetLastConnection()
				require.NotNil(t, conn)
				assert.Equal(t, "sender@test.com", conn.From)
			} else {
				// Should fail when enforcing verification with self-signed cert
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "certificate")
			}
		})
	}
}