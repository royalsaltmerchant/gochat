package main

import (
	"fmt"
	"net/smtp"
	"os"
	"strings"
)

func SendEmail(recipient string, subject string, body string) error {
	smtpHost := getenvOrDefault("SMTP_HOST", "smtp.gmail.com")
	smtpPort := getenvOrDefault("SMTP_PORT", "587")
	authEmail := strings.TrimSpace(os.Getenv("EMAIL"))
	authPassword := os.Getenv("EMAIL_PASSWORD")

	if authEmail == "" || authPassword == "" {
		return fmt.Errorf("missing EMAIL or EMAIL_PASSWORD env vars")
	}

	auth := smtp.PlainAuth("", authEmail, authPassword, smtpHost)

	msg := []byte(
		"From: " + authEmail + "\r\n" +
			"To: " + recipient + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n" +
			"\r\n" +
			body + "\r\n",
	)

	return smtp.SendMail(smtpHost+":"+smtpPort, auth, authEmail, []string{recipient}, msg)
}

func getenvOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
