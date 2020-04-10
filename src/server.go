package main

import (
	"fmt"

	guerrilla "github.com/flashmob/go-guerrilla"
	"github.com/flashmob/go-guerrilla/backends"
	"github.com/flashmob/go-guerrilla/log"
	"github.com/flashmob/go-guerrilla/mail"
)

// Start starts the server.
func Start(appConfig *mailRelayConfig) (err error) {

	listen := fmt.Sprintf("%s:%d", appConfig.LocalListenIP, appConfig.LocalListenPort)

	cfg := &guerrilla.AppConfig{LogFile: log.OutputStdout.String(), AllowedHosts: appConfig.AllowedHosts}
	sc := guerrilla.ServerConfig{
		ListenInterface: listen,
		IsEnabled:       true,
	}
	cfg.Servers = append(cfg.Servers, sc)

	bcfg := backends.BackendConfig{
		"save_workers_size":  3,
		"save_process":       "HeadersParser|Header|Hasher|Debugger|MailRelay",
		"log_received_mails": true,
		"primary_mail_host":  "homeoffice.com",
		"smtp_username":      appConfig.SMTPUsername,
		"smtp_password":      appConfig.SMTPPassword,
		"smtp_server":        appConfig.SMTPServer,
		"smtp_port":          appConfig.SMTPPort,
	}
	cfg.BackendConfig = bcfg

	d := guerrilla.Daemon{Config: cfg}
	d.AddProcessor("MailRelay", mailRelayProcessor)

	return d.Start()
}

type relayConfig struct {
	SMTPServer   string `json:"smtp_server"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
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
						fmt.Printf("!!! %v\n", err)
						return backends.NewResult(fmt.Sprintf("554 Error: %s", err)), err
					}

					return p.Process(e, task)
				}
				return p.Process(e, task)
			},
		)
	}
}
