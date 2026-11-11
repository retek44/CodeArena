package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/retek44/CodeArena/api/internal/auth"
	"github.com/retek44/CodeArena/api/internal/httpapi"
)

const (
	issuer   = "https://cognito-idp.eu-central-1.amazonaws.com/eu-central-1_TEST"
	clientID = "spa-client"
	keyID    = "key-1"
)

type fixture struct {
	key  *rsa.PrivateKey
	auth *auth.CognitoAuthenticator
}

// newFixture serves a JWKS with one RSA key, like Cognito's /.well-known/jwks.json.
func newFixture(t *testing.T) fixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	jwks := map[string]any{"keys": []map[string]string{{
		"kty": "RSA",
		"kid": keyID,
		"use": "sig",
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	t.Cleanup(srv.Close)

	a, err := auth.NewCognito(t.Context(), auth.Config{Issuer: issuer, ClientID: clientID, JWKSURL: srv.URL})
	if err != nil {
		t.Fatalf("NewCognito: %v", err)
	}
	return fixture{key: key, auth: a}
}

func validClaims() jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"sub":       "1f2e3d4c-user",
		"iss":       issuer,
		"client_id": clientID,
		"token_use": "access",
		"username":  "alice",
		"iat":       now.Add(-time.Minute).Unix(),
		"exp":       now.Add(time.Hour).Unix(),
	}
}

func (f fixture) sign(t *testing.T, claims jwt.MapClaims, mutate func(*jwt.Token)) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = keyID
	if mutate != nil {
		mutate(token)
	}
	signed, err := token.SignedString(f.key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func requestWith(t *testing.T, authorization string) *http.Request {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/users/me/submissions", nil)
	if authorization != "" {
		r.Header.Set("Authorization", authorization)
	}
	return r
}

func TestValidAccessToken(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	p, err := f.auth.Authenticate(requestWith(t, "Bearer "+f.sign(t, validClaims(), nil)))
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if p.UserID != "1f2e3d4c-user" || p.Username != "alice" {
		t.Errorf("principal = %+v", p)
	}
}

func TestRejectedTokens(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	with := func(key string, value any) jwt.MapClaims {
		c := validClaims()
		if value == nil {
			delete(c, key)
		} else {
			c[key] = value
		}
		return c
	}
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signedWith := func(key *rsa.PrivateKey, kid string) string {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, validClaims())
		token.Header["kid"] = kid
		s, err := token.SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	// alg=none: the classic signature bypass.
	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, validClaims()).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	// HS256 keyed with the public modulus: algorithm confusion.
	hmacToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims()).SignedString(f.key.N.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]string{
		"no header":             "",
		"basic scheme":          "Basic YWxpY2U6cGFzcw==",
		"empty bearer":          "Bearer ",
		"malformed":             "Bearer not.a.jwt",
		"expired":               "Bearer " + f.sign(t, with("exp", time.Now().Add(-time.Hour).Unix()), nil),
		"no expiry":             "Bearer " + f.sign(t, with("exp", nil), nil),
		"issued in the future":  "Bearer " + f.sign(t, with("iat", time.Now().Add(time.Hour).Unix()), nil),
		"wrong issuer":          "Bearer " + f.sign(t, with("iss", "https://evil.example.com/pool"), nil),
		"other client":          "Bearer " + f.sign(t, with("client_id", "someone-else"), nil),
		"id token":              "Bearer " + f.sign(t, with("token_use", "id"), nil),
		"no subject":            "Bearer " + f.sign(t, with("sub", nil), nil),
		"signed by another key": "Bearer " + signedWith(otherKey, keyID),
		"unknown key id":        "Bearer " + signedWith(f.key, "rotated-away"),
		"alg none":              "Bearer " + unsigned,
		"alg HS256 confusion":   "Bearer " + hmacToken,
	}
	for name, header := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := f.auth.Authenticate(requestWith(t, header)); !errors.Is(err, httpapi.ErrUnauthenticated) {
				t.Errorf("err = %v, want ErrUnauthenticated", err)
			}
		})
	}
}

func TestBearerSchemeIsCaseInsensitive(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	if _, err := f.auth.Authenticate(requestWith(t, "bearer "+f.sign(t, validClaims(), nil))); err != nil {
		t.Fatalf("lowercase scheme rejected: %v", err)
	}
}

func TestNewCognitoRequiresConfig(t *testing.T) {
	t.Parallel()

	if _, err := auth.NewCognito(t.Context(), auth.Config{ClientID: clientID}); err == nil {
		t.Error("missing issuer accepted")
	}
	if _, err := auth.NewCognito(t.Context(), auth.Config{Issuer: issuer}); err == nil {
		t.Error("missing client ID accepted")
	}
}
