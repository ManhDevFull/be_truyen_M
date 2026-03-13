package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"truyenm/backend/internal/models"
	"truyenm/backend/internal/utils"
)

func TestAdminStatsAndUsers(t *testing.T) {
	app, _, db, _, _ := setupTestApp(t)

	// create owner user
	email := "owner+" + time.Now().Format("150405.000") + "@example.com"
	hash, _ := utils.HashPassword("password123")
	db.Create(&models.User{
		Username:     "owner" + time.Now().Format("150405"),
		Email:        email,
		PasswordHash: hash,
		Role:         models.RoleOwner,
	})

	token := loginForToken(t, app, email, "password123")

	res := doJSONWithAuth(t, app, http.MethodGet, "/api/admin/stats", nil, token)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("stats status = %d", res.StatusCode)
	}

	res = doJSONWithAuth(t, app, http.MethodGet, "/api/admin/users?limit=5", nil, token)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("users status = %d", res.StatusCode)
	}
}

func loginForToken(t *testing.T, app *fiber.App, email, password string) string {
	t.Helper()
	body := map[string]any{"identifier": email, "password": password}
	res := doJSON(t, app, http.MethodPost, "/api/auth/login", body, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d", res.StatusCode)
	}
	var parsed map[string]any
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	token, _ := parsed["token"].(string)
	if token == "" {
		t.Fatalf("empty token")
	}
	return token
}

func doJSONWithAuth(t *testing.T, app *fiber.App, method, path string, body any, token string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test error: %v", err)
	}
	return res
}
