package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/smtp"
	"net/textproto"

	"github.com/flashmob/go-guerrilla/mail"
	"github.com/pkg/errors"
)

type closeable interface {
	Close() error
}

// sendMail sends the contents of the envelope to a SMTP server.
func sendMail(e *mail.Envelope, config *relayConfig) error {
	server := fmt.Sprintf("%s:%d", config.SMTPServer, config.SMTPPort)
	to := getTo(e)

	var msg bytes.Buffer
	msg.Write(e.Data.Bytes())
	msg.WriteString("\r\n")

	fmt.Println("==== Starting email send ====")
	defer fmt.Println("==== Finished email send ====")
	var err error
	var conn *tls.Conn
	var client *smtp.Client
	var writer io.WriteCloser

	tlsconfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         config.SMTPServer,
	}

	if conn, err = tls.Dial("tcp", server, tlsconfig); err != nil {
		return errors.Wrap(err, "dial error")
	}

	if client, err = smtp.NewClient(conn, config.SMTPServer); err != nil {
		close(conn, "conn")
		return errors.Wrap(err, "newclient error")
	}
	shouldCloseClient := true
	defer func(shouldClose *bool) {
		if *shouldClose {
			close(client, "client")
		}
	}(&shouldCloseClient)

	auth := smtp.PlainAuth("", config.SMTPUsername, config.SMTPPassword, config.SMTPServer)
	if err = client.Auth(auth); err != nil {
		return errors.Wrap(err, "auth error")
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
	return nil
}

func close(c closeable, what string) {
	err := c.Close()
	if err != nil {
		fmt.Printf("!!!!! Error closing %s: %v\n", what, err)
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
	for _, addy := range e.RcptTo {
		ret = append(ret, addy.String())
	}
	return ret
}

func display(b []byte) {
	s := string(b)
	fmt.Println("################################")
	fmt.Printf("%s\n", s)
	fmt.Println("################################")
}
