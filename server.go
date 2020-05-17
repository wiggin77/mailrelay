package main

import (
	"fmt"

	guerrilla "github.com/flashmob/go-guerrilla"
	"github.com/flashmob/go-guerrilla/backends"
	"github.com/flashmob/go-guerrilla/log"
	"github.com/flashmob/go-guerrilla/mail"
)

const saveWorkersSize = 3

// Start starts the server.
func Start(appConfig *mailRelayConfig, verbose bool) (err error) {
	listen := fmt.Sprintf("%s:%d", appConfig.LocalListenIP, appConfig.LocalListenPort)

	logLevel := "info"
	if verbose {
		logLevel = "debug"
	}

	cfg := &guerrilla.AppConfig{
		LogFile:      log.OutputStdout.String(),
		AllowedHosts: appConfig.AllowedHosts,
		LogLevel:     logLevel,
	}
	sc := guerrilla.ServerConfig{
		ListenInterface: listen,
		IsEnabled:       true,
	}
	cfg.Servers = append(cfg.Servers, sc)

	bcfg := backends.BackendConfig{
		"save_workers_size":     saveWorkersSize,
		"save_process":          "HeadersParser|Header|Hasher|Debugger|MailRelay",
		"log_received_mails":    true,
		"primary_mail_host":     "homeoffice.com",
		"smtp_username":         appConfig.SMTPUsername,
		"smtp_password":         appConfig.SMTPPassword,
		"smtp_server":           appConfig.SMTPServer,
		"smtp_port":             appConfig.SMTPPort,
		"smtp_starttls":         appConfig.SMTPStartTLS,
		"smtp_login_auth_type":  appConfig.SMTPLoginAuthType,
		"smtp_skip_cert_verify": appConfig.SkipCertVerify,
	}
	cfg.BackendConfig = bcfg

	d := guerrilla.Daemon{Config: cfg}
	d.AddProcessor("MailRelay", mailRelayProcessor)

	return d.Start()
}

type relayConfig struct {
	Server        string `json:"smtp_server"`
	Port          int    `json:"smtp_port"`
	STARTTLS      bool   `json:"smtp_starttls"`
	LoginAuthType bool   `json:"smtp_login_auth_type"`
	Username      string `json:"smtp_username"`
	Password      string `json:"smtp_password"`
	SkipVerify    bool   `json:"smtp_skip_cert_verify"`
}

// mailRelayProcessor decorator relays emails to another SMTP server.
var mailRelayProcessor = func() backends.Decorator {
	config := &relayConfig{}
	initFunc := backends.InitializeWith(func(backendConfig backends.BackendConfig) error {
		configType := backends.BaseConfig(&relayConfig{})
		bcfg, err := backends.Svc.ExtractConfig(backendConfig, configType)
		if err != nil {
			return err
		}
		config = bcfg.(*relayConfig)
		return nil
	})
	backends.Svc.AddInitializer(initFunc)

	return func(p backends.Processor) backends.Processor {
		return backends.ProcessWith(
			func(e *mail.Envelope, task backends.SelectTask) (backends.Result, error) {
				if task == backends.TaskSaveMail {

					err := sendMail(e, config)
					if err != nil {
						return backends.NewResult(err.Error()), err
					}

					return p.Process(e, task)
				}
				return p.Process(e, task)
			},
		)
	}
}
