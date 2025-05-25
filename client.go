package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"net/textproto"
	"strconv"

	"github.com/phires/go-guerrilla/mail"
	"github.com/pkg/errors"
)

type closeable interface {
	Close() error
}

// sendMail sends the contents of the envelope to a SMTP server.
func sendMail(e *mail.Envelope, config *relayConfig) error {
	server := net.JoinHostPort(config.Server, strconv.Itoa(config.Port))
	to := getTo(e)

	var msg bytes.Buffer
	msg.Write(e.Data.Bytes())
	msg.WriteString("\r\n")

	Logger.Infof("starting email send -- from:%s, starttls:%t", e.MailFrom.String(), config.STARTTLS)
	Logger.Infof("Client Remote IP: %s", e.RemoteIP)

	var err error
	var conn net.Conn
	var client *smtp.Client
	var writer io.WriteCloser

	if AllowedSendersFilter.Blocked(e.RemoteIP) {
		Logger.Info("Remote IP of " + e.RemoteIP + " not allowed to send email.")
		return errors.New("Remote IP of " + e.RemoteIP + " not allowed to send email.")
	}

	tlsconfig := &tls.Config{
		// InsecureSkipVerify is configurable to support legacy SMTP servers with
		// self-signed certificates or hostname mismatches. This should only be
		// enabled in trusted network environments.
		InsecureSkipVerify: config.SkipVerify, //nolint:gosec
		ServerName:         config.Server,
	}

	if config.STARTTLS {
		if conn, err = net.Dial("tcp", server); err != nil {
			return errors.Wrap(err, "dial error")
		}
	} else {
		if conn, err = tls.Dial("tcp", server, tlsconfig); err != nil {
			return errors.Wrap(err, "TLS dial error")
		}
	}

	if client, err = smtp.NewClient(conn, config.Server); err != nil {
		close(conn, "conn")
		return errors.Wrap(err, "newclient error")
	}
	shouldCloseClient := true
	defer func(shouldClose *bool) {
		if *shouldClose {
			close(client, "client")
		}
	}(&shouldCloseClient)

	if err = handshake(client, config, tlsconfig); err != nil {
		return err
	}

	if err = client.Mail(e.MailFrom.String()); err != nil {
		return errors.Wrap(err, "mail error")
	}

	for _, addy := range to {
		if err = client.Rcpt(addy); err != nil {
			return errors.Wrap(err, "rcpt error")
		}
	}

	if writer, err = client.Data(); err != nil {
		return errors.Wrap(err, "data error")
	}
	_, err = writer.Write(msg.Bytes())
	close(writer, "writer")
	if err != nil {
		return errors.Wrap(err, "write error")
	}

	if err = client.Quit(); isQuitError(err) {
		return errors.Wrap(err, "quit error")
	}
	// We only need to close client if some other error prevented us
	// from getting to `client.Quit`
	shouldCloseClient = false
	Logger.Info("email sent with no errors.")
	return nil
}

func handshake(client *smtp.Client, config *relayConfig, tlsConfig *tls.Config) error {
	if config.HeloHost != "" {
		if err := client.Hello(config.HeloHost); err != nil {
			return errors.Wrap(err, "HELO error")
		}
	}

	if config.STARTTLS {
		if err := client.StartTLS(tlsConfig); err != nil {
			return errors.Wrap(err, "starttls error")
		}
	}

	var auth smtp.Auth = nil

	if config.LoginAuthType {
		auth = LoginAuth(config.Username, config.Password)
	} else if config.Username != "" {
		auth = smtp.PlainAuth("", config.Username, config.Password, config.Server)
	}

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return errors.Wrap(err, "auth error")
		}
	}
	return nil
}

func close(c closeable, what string) {
	err := c.Close()
	if err != nil {
		fmt.Printf("Error closing %s: %v\n", what, err)
	}
}

func isQuitError(err error) bool {
	if err == nil {
		return false
	}
	e, ok := err.(*textproto.Error)
	if ok {
		// SMTP codes 221 or 250 are acceptable here
		if e.Code == 221 || e.Code == 250 {
			return false
		}
	}
	return true
}

// getTo returns the array of email addresses in the envelope.
func getTo(e *mail.Envelope) []string {
	var ret []string
	for i := range e.RcptTo {
		ret = append(ret, e.RcptTo[i].String())
	}
	return ret
}
