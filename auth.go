package main

import (
	"fmt"
	"net/smtp"
)

type loginAuth struct {
	username, password string
}

// LoginAuth provides a simple implementation of LOGIN authorization of SMTP as
// described here: https://www.ietf.org/archive/id/draft-murchison-sasl-login-00.txt
func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte{}, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch string(fromServer) {
		case "User Name", "Username:":
			return []byte(a.username), nil
		case "Password", "Password:":
			return []byte(a.password), nil
		default:
			return nil, fmt.Errorf("unknown server response \"%s\"", string(fromServer))
		}
	}
	return nil, nil
}
