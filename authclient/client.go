// Package authclient isolates the one outbound HTTP dependency this repo has
// on auth-service: confirming that a user id corresponds to a real, existing
// system user. It is deliberately kept separate from the service/api layers
// so it can be swapped for a fake in tests without a live auth-service.
package authclient

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	// EnvBaseURL is the environment variable used to configure auth-service's
	// base URL for operator identity validation.
	EnvBaseURL = "AUTH_SERVICE_BASE_URL"
	// DefaultBaseURL is used when EnvBaseURL is unset or empty.
	//
	// [INFERRED — please validate] auth-service's own application.yml /
	// application-dev.yml set server.servlet.context-path: /auth/, which
	// would normally prefix every route (including /api/users/{id}) with
	// /auth. The cross-repo contract for this story explicitly confirms the
	// direct (non-gateway) route as /api/users/{id} with no such prefix, so
	// that is what this client assumes by default. If the deployed
	// auth-service instance actually applies that context-path, set
	// AUTH_SERVICE_BASE_URL to include it (e.g. http://host:8081/auth)
	// without any code change.
	DefaultBaseURL = "http://localhost:8081"
)

// UserValidator checks whether a user id corresponds to a real, existing
// system user. Implemented here by HTTPUserValidator (calling auth-service)
// and satisfied structurally - callers depending on this shape (see
// service.UserValidator) never need to import this package's HTTP machinery.
type UserValidator interface {
	UserExists(userID int) (bool, error)
}

// HTTPUserValidator validates a user id by calling auth-service's
// GET /api/users/{id} endpoint (confirmed route, see auth-service's
// UserController.java - mapped at /api/users, not /auth/api/users).
type HTTPUserValidator struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewHTTPUserValidator builds a validator using AUTH_SERVICE_BASE_URL (or
// DefaultBaseURL when unset/empty) as auth-service's base URL.
func NewHTTPUserValidator() *HTTPUserValidator {
	baseURL := os.Getenv(EnvBaseURL)
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &HTTPUserValidator{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// UserExists returns (true, nil) when auth-service confirms the user id
// exists (HTTP 200), (false, nil) when auth-service reports it does not
// (HTTP 404), and (false, non-nil error) for any other failure - a network
// error or an unexpected status code. Callers should treat a non-nil error
// as a distinct "couldn't check" outcome, not the same as "confirmed
// missing".
func (v *HTTPUserValidator) UserExists(userID int) (bool, error) {
	baseURL := strings.TrimRight(v.BaseURL, "/")
	url := fmt.Sprintf("%s/api/users/%d", baseURL, userID)

	client := v.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Get(url)
	if err != nil {
		return false, fmt.Errorf("calling auth-service at %s: %w", url, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("unexpected status %d from auth-service at %s", resp.StatusCode, url)
	}
}
