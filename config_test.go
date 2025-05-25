package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigDefaults(t *testing.T) {
	var cfg mailRelayConfig
	configDefaults(&cfg)

	assert.Equal(t, DefaultSMTPPort, cfg.SMTPPort)
	assert.Equal(t, false, cfg.SMTPStartTLS)
	assert.Equal(t, false, cfg.SMTPLoginAuthType)
	assert.Equal(t, int64(DefaultMaxEmailSize), cfg.MaxEmailSize)
	assert.Equal(t, false, cfg.SkipCertVerify)
	assert.Equal(t, DefaultLocalListenIP, cfg.LocalListenIP)
	assert.Equal(t, DefaultLocalListenPort, cfg.LocalListenPort)
	assert.Equal(t, []string{"*"}, cfg.AllowedHosts)
	assert.Equal(t, "*", cfg.AllowedSenders)
	assert.Equal(t, DefaultTimeoutSecs, cfg.TimeoutSecs)
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantErr  bool
		validate func(t *testing.T, cfg *mailRelayConfig)
	}{
		{
			name:     "valid config",
			filename: "testdata/valid.json",
			wantErr:  false,
			validate: func(t *testing.T, cfg *mailRelayConfig) {
				assert.Equal(t, "smtp.test.com", cfg.SMTPServer)
				assert.Equal(t, 587, cfg.SMTPPort)
				assert.Equal(t, true, cfg.SMTPStartTLS)
				assert.Equal(t, "testuser@test.com", cfg.SMTPUsername)
				assert.Equal(t, "testpassword", cfg.SMTPPassword)
				assert.Equal(t, "relay.test.com", cfg.SMTPHelo)
				assert.Equal(t, "127.0.0.1", cfg.LocalListenIP)
				assert.Equal(t, 2525, cfg.LocalListenPort)
				assert.Equal(t, []string{"test.com", "example.com"}, cfg.AllowedHosts)
				assert.Equal(t, 60, cfg.TimeoutSecs)
			},
		},
		{
			name:     "minimal config with defaults",
			filename: "testdata/minimal.json",
			wantErr:  false,
			validate: func(t *testing.T, cfg *mailRelayConfig) {
				assert.Equal(t, "smtp.minimal.com", cfg.SMTPServer)
				assert.Equal(t, "user@minimal.com", cfg.SMTPUsername)
				assert.Equal(t, "password", cfg.SMTPPassword)
				// Check that defaults are applied
				assert.Equal(t, DefaultSMTPPort, cfg.SMTPPort)
				assert.Equal(t, DefaultLocalListenIP, cfg.LocalListenIP)
				assert.Equal(t, DefaultLocalListenPort, cfg.LocalListenPort)
				assert.Equal(t, []string{"*"}, cfg.AllowedHosts)
			},
		},
		{
			name:     "invalid JSON",
			filename: "testdata/invalid.json",
			wantErr:  true,
			validate: nil,
		},
		{
			name:     "nonexistent file",
			filename: "testdata/nonexistent.json",
			wantErr:  true,
			validate: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := loadConfig(tt.filename)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, cfg)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)

			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}
