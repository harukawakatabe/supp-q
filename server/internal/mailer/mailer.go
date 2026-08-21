package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

type LoginCodeSender interface {
	SendLoginCode(context.Context, string, string) error
}

type SMTP struct {
	Host       string
	Port       string
	From       string
	Username   string
	Password   string
	RequireTLS bool
}

func (sender SMTP) SendLoginCode(ctx context.Context, recipient, code string) error {
	from, err := mail.ParseAddress(sender.From)
	if err != nil {
		return fmt.Errorf("parse sender: %w", err)
	}
	address := net.JoinHostPort(sender.Host, sender.Port)
	var auth smtp.Auth
	if sender.Username != "" {
		auth = smtp.PlainAuth("", sender.Username, sender.Password, sender.Host)
	}

	message := strings.Join([]string{
		"From: " + sender.From,
		"To: " + recipient,
		"Subject: =?UTF-8?B?5bCP6KGlUSDpqozor4HnoIE=?=",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		"你的小补Q验证码是：" + code,
		"",
		"验证码 10 分钟内有效。如果不是你本人操作，请忽略此邮件。",
	}, "\r\n")

	dialer := net.Dialer{Timeout: 15 * time.Second}
	connection, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("connect SMTP: %w", err)
	}
	defer connection.Close()
	deadline := time.Now().Add(20 * time.Second)
	if value, ok := ctx.Deadline(); ok && value.Before(deadline) {
		deadline = value
	}
	_ = connection.SetDeadline(deadline)
	client, err := smtp.NewClient(connection, sender.Host)
	if err != nil {
		return fmt.Errorf("start SMTP: %w", err)
	}
	defer client.Close()
	if supported, _ := client.Extension("STARTTLS"); supported {
		if err = client.StartTLS(&tls.Config{ServerName: sender.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("start SMTP TLS: %w", err)
		}
	} else if sender.RequireTLS {
		return fmt.Errorf("SMTP server does not offer STARTTLS")
	}
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("authenticate SMTP: %w", err)
		}
	}
	if err = client.Mail(from.Address); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if err = client.Rcpt(recipient); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("start SMTP message: %w", err)
	}
	if _, err = writer.Write([]byte(message)); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err = writer.Close(); err != nil {
		return fmt.Errorf("finish SMTP message: %w", err)
	}
	if err = client.Quit(); err != nil {
		return fmt.Errorf("quit SMTP: %w", err)
	}
	return nil
}
