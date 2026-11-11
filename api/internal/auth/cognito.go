// Package auth verifies Cognito access tokens for the HTTP API.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/retek44/CodeArena/api/internal/httpapi"
)

// Config identifies the user pool and the app client whose tokens are accepted.
type Config struct {
	// Issuer is https://cognito-idp.<region>.amazonaws.com/<user pool id>.
	Issuer string
	// ClientID is the SPA app client. Cognito access tokens carry it in "client_id"
	// (they have no "aud" claim).
	ClientID string
	// JWKSURL defaults to <Issuer>/.well-known/jwks.json. Tests point it elsewhere.
	JWKSURL string
	// Leeway tolerates small clock differences for exp/iat.
	Leeway time.Duration
}

// CognitoAuthenticator implements httpapi.Authenticator for Cognito access tokens.
type CognitoAuthenticator struct {
	cfg    Config
	keys   keyfunc.Keyfunc
	parser *jwt.Parser
}

// accessTokenClaims are the Cognito access token claims the API relies on.
type accessTokenClaims struct {
	jwt.RegisteredClaims
	TokenUse string `json:"token_use"`
	ClientID string `json:"client_id"`
	Username string `json:"username"`
}

// NewCognito fetches the signing keys once and keeps them fresh in the background
// until ctx is cancelled (keys rotate; an unknown "kid" triggers a rate-limited refresh).
func NewCognito(ctx context.Context, cfg Config) (*CognitoAuthenticator, error) {
	if cfg.Issuer == "" || cfg.ClientID == "" {
		return nil, errors.New("cognito issuer and client ID are required")
	}
	if cfg.JWKSURL == "" {
		cfg.JWKSURL = strings.TrimSuffix(cfg.Issuer, "/") + "/.well-known/jwks.json"
	}
	if cfg.Leeway == 0 {
		cfg.Leeway = 30 * time.Second
	}

	keys, err := keyfunc.NewDefaultCtx(ctx, []string{cfg.JWKSURL})
	if err != nil {
		return nil, fmt.Errorf("load JWKS from %s: %w", cfg.JWKSURL, err)
	}

	return &CognitoAuthenticator{
		cfg:  cfg,
		keys: keys,
		parser: jwt.NewParser(
			// Only RS256: blocks "none" and HMAC-with-the-public-key confusion.
			jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
			jwt.WithIssuer(cfg.Issuer),
			jwt.WithExpirationRequired(),
			jwt.WithIssuedAt(),
			jwt.WithLeeway(cfg.Leeway),
		),
	}, nil
}

// Authenticate implements httpapi.Authenticator.
func (a *CognitoAuthenticator) Authenticate(r *http.Request) (httpapi.Principal, error) {
	raw, ok := bearerToken(r)
	if !ok {
		return httpapi.Principal{}, httpapi.ErrUnauthenticated
	}

	claims := &accessTokenClaims{}
	if _, err := a.parser.ParseWithClaims(raw, claims, a.keys.Keyfunc); err != nil {
		return httpapi.Principal{}, fmt.Errorf("%w: %w", httpapi.ErrUnauthenticated, err)
	}

	switch {
	case claims.TokenUse != "access":
		// ID tokens are for the client, not for API calls.
		return httpapi.Principal{}, fmt.Errorf("%w: token_use is %q", httpapi.ErrUnauthenticated, claims.TokenUse)
	case claims.ClientID != a.cfg.ClientID:
		return httpapi.Principal{}, fmt.Errorf("%w: token issued to another client", httpapi.ErrUnauthenticated)
	case claims.Subject == "":
		return httpapi.Principal{}, fmt.Errorf("%w: missing sub", httpapi.ErrUnauthenticated)
	}

	username := claims.Username
	if username == "" {
		username = claims.Subject
	}
	return httpapi.Principal{UserID: claims.Subject, Username: username}, nil
}

func bearerToken(r *http.Request) (string, bool) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}
