package authclient

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestHTTPUserValidator_UserExists(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantExists bool
		wantErr    bool
	}{
		{name: "known user returns 200", statusCode: http.StatusOK, wantExists: true, wantErr: false},
		{name: "unknown user returns 404", statusCode: http.StatusNotFound, wantExists: false, wantErr: false},
		{name: "unexpected status is an error", statusCode: http.StatusInternalServerError, wantExists: false, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/users/42" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			v := &HTTPUserValidator{BaseURL: server.URL, HTTPClient: server.Client()}

			exists, err := v.UserExists(42)
			if exists != tt.wantExists {
				t.Errorf("UserExists() exists = %v, want %v", exists, tt.wantExists)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("UserExists() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHTTPUserValidator_UserExists_NetworkError(t *testing.T) {
	// A server that is immediately closed leaves BaseURL pointing at
	// nothing listening - the Get call should fail at the transport level.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	baseURL := server.URL
	server.Close()

	v := &HTTPUserValidator{BaseURL: baseURL, HTTPClient: http.DefaultClient}
	exists, err := v.UserExists(1)
	if exists {
		t.Errorf("UserExists() exists = true, want false on network error")
	}
	if err == nil {
		t.Errorf("UserExists() err = nil, want non-nil on network error")
	}
}

func TestHTTPUserValidator_TrimsTrailingSlashInBaseURL(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	v := &HTTPUserValidator{BaseURL: server.URL + "/", HTTPClient: server.Client()}
	if _, err := v.UserExists(7); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/api/users/7" {
		t.Errorf("got path %q, want /api/users/7", gotPath)
	}
}

func TestNewHTTPUserValidator_DefaultsAndEnvVar(t *testing.T) {
	original, hadOriginal := os.LookupEnv(EnvBaseURL)
	defer func() {
		if hadOriginal {
			os.Setenv(EnvBaseURL, original)
		} else {
			os.Unsetenv(EnvBaseURL)
		}
	}()

	os.Unsetenv(EnvBaseURL)
	v := NewHTTPUserValidator()
	if v.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL = %q, want default %q", v.BaseURL, DefaultBaseURL)
	}

	os.Setenv(EnvBaseURL, "http://auth.example.internal:9090")
	v = NewHTTPUserValidator()
	if v.BaseURL != "http://auth.example.internal:9090" {
		t.Errorf("BaseURL = %q, want env-configured value", v.BaseURL)
	}
}
