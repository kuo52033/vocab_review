package email

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

type SESSender struct {
	client    *sesv2.Client
	fromEmail string
	fromName  string
}

func NewSESSender(config aws.Config, fromEmail, fromName string) *SESSender {
	return &SESSender{
		client:    sesv2.NewFromConfig(config),
		fromEmail: strings.TrimSpace(fromEmail),
		fromName:  strings.TrimSpace(fromName),
	}
}

func (s *SESSender) SendMagicLink(ctx context.Context, email, verificationURL, token string, expiresAt time.Time) error {
	from := s.fromEmail
	if s.fromName != "" {
		from = fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail)
	}
	textBody := fmt.Sprintf("Use this link to sign in to Vocab Review on the web:\n\n%s\n\nUsing the Chrome extension or iOS app? Paste this verification token instead:\n\n%s\n\nThis link and token expire at %s. If you did not request it, you can ignore this email.\n", verificationURL, token, expiresAt.Format(time.RFC1123))
	htmlURL := html.EscapeString(verificationURL)
	htmlToken := html.EscapeString(token)
	htmlBody := fmt.Sprintf(`<p>Use this link to sign in to Vocab Review on the web:</p><p><a href="%s">Sign in to Vocab Review</a></p><p>Using the Chrome extension or iOS app? Paste this verification token instead:</p><p><code>%s</code></p><p>This link and token expire at %s. If you did not request it, you can ignore this email.</p>`, htmlURL, htmlToken, html.EscapeString(expiresAt.Format(time.RFC1123)))

	_, err := s.client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(from),
		Destination: &types.Destination{
			ToAddresses: []string{email},
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{
					Data:    aws.String("Your Vocab Review sign-in link"),
					Charset: aws.String("UTF-8"),
				},
				Body: &types.Body{
					Text: &types.Content{
						Data:    aws.String(textBody),
						Charset: aws.String("UTF-8"),
					},
					Html: &types.Content{
						Data:    aws.String(htmlBody),
						Charset: aws.String("UTF-8"),
					},
				},
			},
		},
	})
	return err
}
