package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/model"
)

func TestBasicAuthAcceptsManagedAccessToken(t *testing.T) {
	oldLoad := loadManagedAccessTokenFromKeystore
	t.Cleanup(func() {
		loadManagedAccessTokenFromKeystore = oldLoad
	})
	loadManagedAccessTokenFromKeystore = func() (string, error) {
		return "managed-token", nil
	}

	handler := BasicAuth(func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		w.WriteHeader(http.StatusAccepted)
	}, "api-user", "api-pass")

	for name, applyAuth := range map[string]func(*http.Request){
		"x-api-token": func(req *http.Request) {
			req.Header.Set(model.API_TOKEN, "managed-token")
		},
		"bearer-token": func(req *http.Request) {
			req.Header.Set("Authorization", "Bearer managed-token")
		},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/stats", nil)
			applyAuth(req)
			recorder := httptest.NewRecorder()
			handler(recorder, req, nil)

			if recorder.Code != http.StatusAccepted {
				t.Fatalf("unexpected status: %d", recorder.Code)
			}
		})
	}
}

func TestBasicAuthFallsBackToBasicAuthCredentials(t *testing.T) {
	oldLoad := loadManagedAccessTokenFromKeystore
	t.Cleanup(func() {
		loadManagedAccessTokenFromKeystore = oldLoad
	})
	loadManagedAccessTokenFromKeystore = func() (string, error) {
		return "", nil
	}

	handler := BasicAuth(func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		w.WriteHeader(http.StatusAccepted)
	}, "api-user", "api-pass")

	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	req.SetBasicAuth("api-user", "api-pass")
	recorder := httptest.NewRecorder()
	handler(recorder, req, nil)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}
