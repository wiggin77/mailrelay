package main

import (
	"net/smtp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoginAuth(t *testing.T) {
	auth := LoginAuth("testuser", "testpass")
	assert.NotNil(t, auth)

	// Type assertion to ensure we get the right type
	loginAuth, ok := auth.(*loginAuth)
	assert.True(t, ok)
	assert.Equal(t, "testuser", loginAuth.username)
	assert.Equal(t, "testpass", loginAuth.password)
}

func TestLoginAuthStart(t *testing.T) {
	auth := &loginAuth{
		username: "testuser",
		password: "testpass",
	}

	method, resp, err := auth.Start(&smtp.ServerInfo{})

	assert.NoError(t, err)
	assert.Equal(t, "LOGIN", method)
	assert.Empty(t, resp)
}

func TestLoginAuthNext(t *testing.T) {
	auth := &loginAuth{
		username: "testuser",
		password: "testpass",
	}

	tests := []struct {
		name      string
		serverMsg string
		more      bool
		expected  string
		expectErr bool
	}{
		{
			name:      "username prompt - User Name",
			serverMsg: "User Name",
			more:      true,
			expected:  "testuser",
			expectErr: false,
		},
		{
			name:      "username prompt - Username:",
			serverMsg: "Username:",
			more:      true,
			expected:  "testuser",
			expectErr: false,
		},
		{
			name:      "password prompt - Password",
			serverMsg: "Password",
			more:      true,
			expected:  "testpass",
			expectErr: false,
		},
		{
			name:      "password prompt - Password:",
			serverMsg: "Password:",
			more:      true,
			expected:  "testpass",
			expectErr: false,
		},
		{
			name:      "unknown server response",
			serverMsg: "Unknown Prompt",
			more:      true,
			expected:  "",
			expectErr: true,
		},
		{
			name:      "more is false",
			serverMsg: "anything",
			more:      false,
			expected:  "",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := auth.Next([]byte(tt.serverMsg), tt.more)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "unknown server response")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, string(resp))
			}
		})
	}
}
