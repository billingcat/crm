package controller

import (
	"fmt"

	"github.com/mailjet/mailjet-apiv3-go"
)

func (ctrl *controller) sendEmail(to string, subject string, body string) error {
	// when in production, send real email, else just log to console
	if ctrl.model.Config.Mode == "production" {
		return ctrl.sendRealEmail(to, subject, body)
	}
	fmt.Println("Sending email to", to, "with subject", subject, "and body", body)
	return nil
}

func (ctrl *controller) sendRealEmail(to string, subject string, body string) error {
	mj := mailjet.NewMailjetClient(ctrl.model.Config.MailAPIKey, ctrl.model.Config.MailSecret)

	messagesInfo := []mailjet.InfoMessagesV31{
		{
			From: &mailjet.RecipientV31{
				Email: "app@billingcat.de",
				Name:  "billingcat app",
			},
			To: &mailjet.RecipientsV31{
				mailjet.RecipientV31{
					Email: to,
				},
			},
			Subject:  subject,
			TextPart: body,
		},
	}

	messages := mailjet.MessagesV31{Info: messagesInfo}
	if _, err := mj.SendMailV31(&messages); err != nil {
		return ErrInvalid(err, "Fehler beim Senden der E-Mail")
	}
	return nil
}
