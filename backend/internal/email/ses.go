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
	textBody := fmt.Sprintf("Use this link to sign in to Vocab Review on the web:\n\n%s\n\nTo sign in from the Chrome extension or iOS app, copy and paste this sign-in code:\n\n%s\n\nThis link and code expire at %s. If you did not request this email, you can ignore it.\n\nIf you requested multiple links, use the latest email.\n", verificationURL, token, expiresAt.Format(time.RFC1123))
	htmlURL := html.EscapeString(verificationURL)
	htmlToken := html.EscapeString(token)
	htmlBody := fmt.Sprintf(`<p>Use this link to sign in to Vocab Review on the web:</p><p><a href="%s">Sign in to Vocab Review</a></p><p>To sign in from the Chrome extension or iOS app, copy and paste this sign-in code:</p><p style="font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 24px; font-weight: 700; letter-spacing: 1px; padding: 16px 18px; background: #fff4f1; border: 1px solid #f2c9c2; border-radius: 8px; color: #3f2326; user-select: all;">%s</p><p>This link and code expire at %s. If you did not request this email, you can ignore it.</p><p>If you requested multiple links, use the latest email.</p>`, htmlURL, htmlToken, html.EscapeString(expiresAt.Format(time.RFC1123)))

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
