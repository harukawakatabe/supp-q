package mailer

import (
	"context"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
)

type LoginCodeSender interface {
	SendLoginCode(context.Context, string, string) error
}

type SMTP struct {
	Host     string
	Port     string
	From     string
	Username string
	Password string
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

	done := make(chan error, 1)
	go func() {
		done <- smtp.SendMail(address, auth, from.Address, []string{recipient}, []byte(message))
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("send login code: %w", err)
		}
		return nil
	}
}
