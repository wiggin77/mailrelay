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

	cfg := &guerrilla.AppConfig{LogFile: log.OutputStdout.String(), AllowedHosts: []string{"warpmail.net"}}
	sc := guerrilla.ServerConfig{
		ListenInterface: "0.0.0.0:2525",
		IsEnabled:       true,
	}
	cfg.Servers = append(cfg.Servers, sc)

	bcfg := backends.BackendConfig{
		"save_workers_size":  3,
		"save_process":       "HeadersParser|Header|Hasher|Debugger|MailRelay",
		"log_received_mails": true,
		"primary_mail_host":  "homeoffice.com",
		"username":           appConfig.Username,
		"password":           appConfig.Password,
		"server":             appConfig.Server,
		"port":               appConfig.Port,
	}
	cfg.BackendConfig = bcfg

	d := guerrilla.Daemon{Config: cfg}
	d.AddProcessor("MailRelay", mailRelayProcessor)

	return d.Start()
}

// mailRelayProcessor decorator relays emails to another SMTP server.
var mailRelayProcessor = func() backends.Decorator {
	config := &mailRelayConfig{}
	initFunc := backends.InitializeWith(func(backendConfig backends.BackendConfig) error {
		configType := backends.BaseConfig(&mailRelayConfig{})
		bcfg, err := backends.Svc.ExtractConfig(backendConfig, configType)
		if err != nil {
			return err
		}
		config = bcfg.(*mailRelayConfig)
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
