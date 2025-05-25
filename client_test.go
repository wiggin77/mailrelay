package main

import (
	"net/textproto"
	"testing"

	"github.com/flashmob/go-guerrilla/mail"
	"github.com/stretchr/testify/assert"
)

func TestGetTo(t *testing.T) {
	tests := []struct {
		name     string
		envelope *mail.Envelope
		expected []string
	}{
		{
			name: "single recipient",
			envelope: &mail.Envelope{
				RcptTo: []mail.Address{
					{User: "user1", Host: "example.com"},
				},
			},
			expected: []string{"user1@example.com"},
		},
		{
			name: "multiple recipients",
			envelope: &mail.Envelope{
				RcptTo: []mail.Address{
					{User: "user1", Host: "example.com"},
					{User: "user2", Host: "test.com"},
					{User: "admin", Host: "company.org"},
				},
			},
			expected: []string{
				"user1@example.com",
				"user2@test.com", 
				"admin@company.org",
			},
		},
		{
			name:     "no recipients",
			envelope: &mail.Envelope{RcptTo: []mail.Address{}},
			expected: nil,
		},
		{
			name:     "nil envelope recipients",
			envelope: &mail.Envelope{RcptTo: nil},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getTo(tt.envelope)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsQuitError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "SMTP 221 code (acceptable)",
			err:      &textproto.Error{Code: 221, Msg: "Bye"},
			expected: false,
		},
		{
			name:     "SMTP 250 code (acceptable)",
			err:      &textproto.Error{Code: 250, Msg: "OK"},
			expected: false,
		},
		{
			name:     "SMTP 550 error code",
			err:      &textproto.Error{Code: 550, Msg: "Mailbox not found"},
			expected: true,
		},
		{
			name:     "SMTP 421 error code",
			err:      &textproto.Error{Code: 421, Msg: "Service not available"},
			expected: true,
		},
		{
			name:     "non-textproto error",
			err:      assert.AnError,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isQuitError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}