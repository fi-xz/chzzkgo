package chzzkgo_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/fi-xz/chzzkgo"
)

func TestGetAuthorizationURL(t *testing.T) {
	chzzk := chzzkgo.New("test-client-id", "test-client-secret", "http://localhost:12940/callback")

	raw := chzzk.GetAuthorizationURL("test-state")

	// clientSecret은 인증 URL에 절대 포함되면 안 된다 (과거 버그 회귀 방지)
	if strings.Contains(raw, "test-client-secret") {
		t.Fatalf("Authorization URL must not contain client secret: %s", raw)
	}

	u, err := url.Parse(raw)

	if err != nil {
		t.Fatal(err)
	}

	if u.Host != "chzzk.naver.com" || u.Path != "/account-interlock" {
		t.Errorf("unexpected URL: %s", raw)
	}

	q := u.Query()

	if got := q.Get("clientId"); got != "test-client-id" {
		t.Errorf("clientId = %q, want %q", got, "test-client-id")
	}

	if got := q.Get("redirectUri"); got != "http://localhost:12940/callback" {
		t.Errorf("redirectUri = %q, want %q", got, "http://localhost:12940/callback")
	}

	if got := q.Get("state"); got != "test-state" {
		t.Errorf("state = %q, want %q", got, "test-state")
	}
}

func TestRequireScope(t *testing.T) {
	t.Run("토큰 미설정", func(t *testing.T) {
		chzzk := chzzkgo.New("test-client-id", "test-client-secret", "http://localhost:12940/callback")

		_, err := chzzk.GetUser(context.Background())

		if !errors.Is(err, chzzkgo.ErrNotAuthenticated) {
			t.Fatalf("want ErrNotAuthenticated, got %v", err)
		}
	})

	t.Run("scope 부족", func(t *testing.T) {
		chzzk := chzzkgo.New("test-client-id", "test-client-secret", "http://localhost:12940/callback")
		chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.ChatMessageRead})

		_, err := chzzk.GetUser(context.Background())

		var missing *chzzkgo.MissingScopeError

		if !errors.As(err, &missing) {
			t.Fatalf("want MissingScopeError, got %v", err)
		}

		if missing.Scope != chzzkgo.UserRead {
			t.Errorf("missing scope = %q, want %q", missing.Scope, chzzkgo.UserRead)
		}
	})
}

func TestExchangeCode(t *testing.T) {
	var gotBody map[string]string

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/v1/token", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("Failed to decode token request body: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", map[string]any{
			"accessToken":  "issued-access",
			"refreshToken": "issued-refresh",
			"expiresIn":    86400,
			"scope":        "유저 조회",
		})
	})

	chzzk := newMockClient(t, mux)

	tokens, err := chzzk.ExchangeCode(context.Background(), "test-code", "test-state")

	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"grantType":    "authorization_code",
		"clientId":     "test-client-id",
		"clientSecret": "test-client-secret",
		"code":         "test-code",
		"state":        "test-state",
	}

	for k, v := range want {
		if gotBody[k] != v {
			t.Errorf("request body %s = %q, want %q", k, gotBody[k], v)
		}
	}

	if tokens.AccessToken != "issued-access" || tokens.RefreshToken != "issued-refresh" {
		t.Errorf("tokens = %+v", tokens)
	}

	if !tokens.Scope.Has(chzzkgo.UserRead) {
		t.Errorf("scope = %v, want to contain %q", tokens.Scope, chzzkgo.UserRead)
	}

	// ExchangeCode는 토큰을 클라이언트에 주입하지 않는다
	_, err = chzzk.GetUser(context.Background())

	if !errors.Is(err, chzzkgo.ErrNotAuthenticated) {
		t.Errorf("tokens must not be injected into client, got %v", err)
	}
}

func TestRevokeToken(t *testing.T) {
	var gotBody map[string]string

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/v1/token/revoke", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("Failed to decode revoke request body: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", nil)
	})

	chzzk := newMockClient(t, mux)

	err := chzzk.RevokeToken(context.Background(), chzzkgo.RevokeTokenRequest{
		Token:         "revoke-me",
		TokenTypeHint: "refresh_token",
	})

	if err != nil {
		t.Fatal(err)
	}

	if gotBody["token"] != "revoke-me" || gotBody["tokenTypeHint"] != "refresh_token" {
		t.Errorf("request body = %v", gotBody)
	}

	// ClientID/ClientSecret 미지정 시 클라이언트 값으로 자동 채움
	if gotBody["clientId"] != "test-client-id" || gotBody["clientSecret"] != "test-client-secret" {
		t.Errorf("clientId/clientSecret not auto-filled: %v", gotBody)
	}

	// 토큰 제거 요청에는 grantType이 전송되지 않는다
	if v, ok := gotBody["grantType"]; ok {
		t.Errorf("grantType must be omitted, got %q", v)
	}
}

func TestRevokeTokenError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/v1/token/revoke", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusBadRequest, 400, "invalid token", nil)
	})

	chzzk := newMockClient(t, mux)

	err := chzzk.RevokeToken(context.Background(), chzzkgo.RevokeTokenRequest{
		Token:         "bad-token",
		TokenTypeHint: "access_token",
	})

	if err == nil {
		t.Fatal("want error, got nil")
	}

	if !strings.Contains(err.Error(), "token revoke") {
		t.Errorf("error = %v, want to mention token revoke", err)
	}
}
