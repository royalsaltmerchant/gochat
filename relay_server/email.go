package main

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
)

func SendEmail(recipient string, subject string, body string) {
	smtpHost := "smtp.gmail.com"
	smtpPort := "587"
	authEmail := os.Getenv("EMAIL")
	authPassword := os.Getenv("EMAIL_PASSWORD")

	auth := smtp.PlainAuth("", authEmail, authPassword, smtpHost)

	to := []string{recipient}

	// Proper headers and body
	msg := []byte(
		"To: " + recipient + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n" +
			"\r\n" + // blank line between headers and body
			body + "\r\n")

	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, authEmail, to, msg)
	if err != nil {
		log.Fatal("Error sending email:", err)
	}

	fmt.Println("Email sent successfully!")
}
