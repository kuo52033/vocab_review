package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"
	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository"
)

const productionMagicLinkMinInterval = 60 * time.Second

type AuthConfig struct {
	Environment      string
	TokenHashSecret  string
	PublicWebBaseURL string
	DebugEmails      []string
}

type AuthResult struct {
	User        domain.User `json:"user"`
	Session     AuthSession `json:"session"`
	RedirectURL string      `json:"redirect_url"`
}

type AuthSession struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type MagicLinkSender interface {
	SendMagicLink(ctx context.Context, email, verificationURL, token string, expiresAt time.Time) error
}

type MagicLinkResponse struct {
	Message         string `json:"message"`
	Token           string `json:"token,omitempty"`
	VerificationURL string `json:"verification_url,omitempty"`
	ExpiresAt       string `json:"expires_at,omitempty"`
}

func (a *App) RequestMagicLink(ctx context.Context, email, baseURL, client string) (MagicLinkResponse, error) {
	email = normalizeEmail(email)
	if !validEmail(email) {
		return MagicLinkResponse{}, errors.New("valid email is required")
	}

	response := MagicLinkResponse{Message: "Check your email for the sign-in link."}
	isDevelopment := a.authConfig.Environment == "development"
	isDebugEmail := a.isDebugEmail(email)
	if !isDevelopment {
		if _, ok, err := a.store.GetUserByEmail(ctx, email); err != nil {
			return MagicLinkResponse{}, err
		} else if !ok {
			return response, nil
		}
	}

	rawToken := newID("ml")
	now := a.clock.Now()
	token := domain.MagicLinkToken{
		TokenHash: a.hashToken(rawToken),
		Email:     email,
		CreatedAt: now,
		ExpiresAt: now.Add(15 * time.Minute),
	}
	minInterval := time.Duration(0)
	if !isDevelopment && !isDebugEmail {
		minInterval = productionMagicLinkMinInterval
	}
	issued, err := a.store.PutMagicLink(ctx, token, minInterval)
	if err != nil {
		return MagicLinkResponse{}, err
	}
	if !issued {
		return response, nil
	}

	verificationURL := a.verificationURL(baseURL, rawToken, client)
	if isDevelopment || isDebugEmail {
		response.Token = rawToken
		response.VerificationURL = verificationURL
		response.ExpiresAt = token.ExpiresAt.Format(time.RFC3339)
	}
	if !isDevelopment && !isDebugEmail && a.magicLinkSender != nil {
		if err := a.magicLinkSender.SendMagicLink(ctx, email, verificationURL, rawToken, token.ExpiresAt); err != nil {
			return response, nil
		}
	}

	return response, nil
}

func (a *App) VerifyMagicLink(ctx context.Context, token string) (AuthResult, error) {
	now := a.clock.Now()
	token = strings.TrimSpace(token)
	if token == "" {
		return AuthResult{}, errors.New("invalid token")
	}
	sessionToken := newID("sess")
	newUser := domain.User{
		ID:        newID("usr"),
		CreatedAt: now,
	}
	newSession := domain.Session{
		TokenHash: a.hashToken(sessionToken),
		CreatedAt: now,
		ExpiresAt: now.Add(30 * 24 * time.Hour),
	}
	user, session, err := a.store.ConsumeMagicLink(ctx, a.hashToken(token), now, newUser, newSession)
	if errors.Is(err, repository.ErrNotFound) {
		return AuthResult{}, errors.New("invalid token")
	}
	if errors.Is(err, repository.ErrExpired) {
		return AuthResult{}, errors.New("token expired")
	}
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		User:        user,
		Session:     AuthSession{Token: sessionToken, ExpiresAt: session.ExpiresAt},
		RedirectURL: "/auth/success?session_token=" + sessionToken,
	}, nil
}

func (a *App) Session(ctx context.Context, token string) (domain.Session, domain.User, error) {
	session, user, ok, err := a.store.GetSessionUser(ctx, a.hashToken(strings.TrimSpace(token)))
	if err != nil {
		return domain.Session{}, domain.User{}, err
	}
	if !ok {
		return domain.Session{}, domain.User{}, errors.New("unauthorized")
	}
	if session.ExpiresAt.Before(a.clock.Now()) {
		return domain.Session{}, domain.User{}, errors.New("session expired")
	}
	return session, user, nil
}

func (a *App) hashToken(token string) string {
	mac := hmac.New(sha256.New, []byte(a.authConfig.TokenHashSecret))
	mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

func (a *App) verificationURL(baseURL, token, client string) string {
	webBaseURL := a.authConfig.PublicWebBaseURL
	if a.authConfig.Environment == "development" && strings.TrimSpace(baseURL) != "" {
		webBaseURL = baseURL
	}
	if strings.TrimSpace(webBaseURL) == "" {
		webBaseURL = "http://localhost:8080"
	}
	return strings.TrimRight(webBaseURL, "/") + "?token=" + token
}

func (a *App) isDebugEmail(email string) bool {
	_, ok := a.debugEmails[email]
	return ok
}

func normalizeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}

func validEmail(email string) bool {
	if email == "" {
		return false
	}
	address, err := mail.ParseAddress(email)
	return err == nil && strings.EqualFold(address.Address, email)
}
