package main

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
)

func envOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func SendEmail(recipient string, subject string, body string) error {
	smtpHost := envOrDefault("SMTP_HOST", "smtp.gmail.com")
	smtpPort := envOrDefault("SMTP_PORT", "587")
	authEmail := os.Getenv("EMAIL")
	authPassword := os.Getenv("EMAIL_PASSWORD")
	fromEmail := envOrDefault("EMAIL_FROM", authEmail)

	if authEmail == "" || authPassword == "" {
		return fmt.Errorf("EMAIL or EMAIL_PASSWORD not configured")
	}

	auth := smtp.PlainAuth("", authEmail, authPassword, smtpHost)
	to := []string{recipient}

	msg := []byte(
		"From: " + fromEmail + "\r\n" +
			"To: " + recipient + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n" +
			"\r\n" +
			body + "\r\n")

	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, authEmail, to, msg)
	if err != nil {
		return err
	}

	log.Printf("Email sent to %s (%s)", recipient, subject)
	return nil
}
