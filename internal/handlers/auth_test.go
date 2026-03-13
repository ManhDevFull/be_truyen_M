package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestRegisterLoginRefreshLogout(t *testing.T) {
	app, _, _, _, _ := setupTestApp(t)

	email := "test+" + time.Now().Format("150405.000") + "@example.com"
	registerBody := map[string]any{
		"username": "user" + time.Now().Format("150405"),
		"email":    email,
		"password": "password123",
	}
	res := doJSON(t, app, http.MethodPost, "/api/auth/register", registerBody, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("register status = %d", res.StatusCode)
	}

	loginBody := map[string]any{"identifier": email, "password": "password123"}
	res = doJSON(t, app, http.MethodPost, "/api/auth/login", loginBody, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d", res.StatusCode)
	}

	// refresh should succeed (cookie set by login)
	res = doJSON(t, app, http.MethodPost, "/api/auth/refresh", nil, res.Header)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("refresh status = %d", res.StatusCode)
	}

	// logout should revoke
	res = doJSON(t, app, http.MethodPost, "/api/auth/logout", nil, res.Header)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("logout status = %d", res.StatusCode)
	}
}

func TestBanCheck(t *testing.T) {
	app, _, db, _, _ := setupTestApp(t)

	email := "ban+" + time.Now().Format("150405.000") + "@example.com"
	registerBody := map[string]any{
		"username": "banuser" + time.Now().Format("150405"),
		"email":    email,
		"password": "password123",
	}
	res := doJSON(t, app, http.MethodPost, "/api/auth/register", registerBody, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("register status = %d", res.StatusCode)
	}

	// mark banned
	db.Exec("UPDATE users SET is_banned = true, ban_expires_at = ? WHERE email = ?", time.Now().Add(1*time.Hour), email)

	loginBody := map[string]any{"identifier": email, "password": "password123"}
	res = doJSON(t, app, http.MethodPost, "/api/auth/login", loginBody, nil)
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for banned, got %d", res.StatusCode)
	}

	// expire ban
	db.Exec("UPDATE users SET ban_expires_at = ? WHERE email = ?", time.Now().Add(-1*time.Hour), email)
	res = doJSON(t, app, http.MethodPost, "/api/auth/login", loginBody, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected login ok after ban expiry, got %d", res.StatusCode)
	}
}

func TestLoginRateLimit(t *testing.T) {
	app, _, _, rdb, _ := setupTestApp(t)
	if rdb == nil {
		t.Skip("TEST_REDIS_ADDR not set")
	}

	loginBody := map[string]any{"identifier": "nope@example.com", "password": "bad"}
	var lastStatus int
	for i := 0; i < 6; i++ {
		res := doJSON(t, app, http.MethodPost, "/api/auth/login", loginBody, nil)
		lastStatus = res.StatusCode
	}
	if lastStatus != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after rate limit, got %d", lastStatus)
	}
}

func doJSON(t *testing.T, app *fiber.App, method, path string, body any, headers http.Header) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if headers != nil {
		for k, v := range headers {
			for _, hv := range v {
				req.Header.Add(k, hv)
			}
		}
	}
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test error: %v", err)
	}
	return res
}
