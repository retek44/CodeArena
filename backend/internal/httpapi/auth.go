package httpapi

import (
	"context"
	"errors"
	"net/http"
	"regexp"
)

// Principal is the authenticated caller.
type Principal struct {
	UserID   string // Cognito "sub"
	Username string
}

// Authenticator turns a request into a Principal. The Cognito JWT implementation
// lives in internal/auth; tests and local development use DevAuthenticator.
type Authenticator interface {
	Authenticate(r *http.Request) (Principal, error)
}

// ErrUnauthenticated means the request has no valid credentials.
var ErrUnauthenticated = errors.New("unauthenticated")

type principalKey struct{}

func principalFrom(ctx context.Context) Principal {
	p, _ := ctx.Value(principalKey{}).(Principal)
	return p
}

func (s *server) requireAuth(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := s.Auth.Authenticate(r)
		if err != nil {
			w.Header().Set("WWW-Authenticate", `Bearer realm="codearena"`)
			s.writeProblem(w, http.StatusUnauthorized, "missing or invalid access token")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
	})
}

// DevUserHeader carries the user ID for DevAuthenticator.
const DevUserHeader = "X-Dev-User"

var devUserPattern = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)

// DevAuthenticator trusts the X-Dev-User header. Only for local development and
// tests: cmd/api enables it only with AUTH_MODE=dev, and AWS deployments set cognito.
type DevAuthenticator struct{}

// Authenticate implements Authenticator.
func (DevAuthenticator) Authenticate(r *http.Request) (Principal, error) {
	id := r.Header.Get(DevUserHeader)
	if !devUserPattern.MatchString(id) {
		return Principal{}, ErrUnauthenticated
	}
	return Principal{UserID: "dev-" + id, Username: id}, nil
}
