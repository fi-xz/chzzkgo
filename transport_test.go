package chzzkgo_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fi-xz/chzzkgo"
)

func testUserContent() map[string]any {
	return map[string]any{"channelId": "test-channel", "channelName": "테스트"}
}

// newRefreshTokenHandler는 refresh_token 요청을 검증하고 새 토큰을 발급하는 핸들러를 반환한다.
func newRefreshTokenHandler(t *testing.T, calls *atomic.Int32, scope string) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		var body map[string]string

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("Failed to decode token request body: %v", err)
		}

		if body["grantType"] != "refresh_token" {
			t.Errorf("grantType = %q, want refresh_token", body["grantType"])
		}

		if body["refreshToken"] != "old-refresh" {
			t.Errorf("refreshToken = %q, want old-refresh", body["refreshToken"])
		}

		writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", map[string]any{
			"accessToken":  "new-access",
			"refreshToken": "new-refresh",
			"expiresIn":    86400,
			"scope":        scope,
		})
	}
}

func TestAuthTransportBearerHeader(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/users/me", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-access" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer test-access")
		}

		writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", testUserContent())
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("test-access", "test-refresh", chzzkgo.Scopes{chzzkgo.UserRead})

	user, err := chzzk.GetUser(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	if user.ChannelID != "test-channel" {
		t.Errorf("ChannelID = %q, want test-channel", user.ChannelID)
	}
}

func TestAuthTransportRefreshOn401(t *testing.T) {
	var tokenCalls atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/users/me", func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("Authorization") {
		case "Bearer old-access":
			writeEnvelope(t, w, http.StatusUnauthorized, 401, "Unauthorized", nil)
		case "Bearer new-access":
			writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", testUserContent())
		default:
			t.Errorf("unexpected Authorization: %q", r.Header.Get("Authorization"))
			writeEnvelope(t, w, http.StatusUnauthorized, 401, "Unauthorized", nil)
		}
	})
	mux.HandleFunc("POST /auth/v1/token", newRefreshTokenHandler(t, &tokenCalls, "유저 조회"))

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("old-access", "old-refresh", chzzkgo.Scopes{chzzkgo.UserRead})

	refreshed := make(chan chzzkgo.Tokens, 1)
	chzzk.OnTokenRefresh(func(tk chzzkgo.Tokens) { refreshed <- tk })

	user, err := chzzk.GetUser(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	if user.ChannelID != "test-channel" {
		t.Errorf("ChannelID = %q, want test-channel", user.ChannelID)
	}

	if n := tokenCalls.Load(); n != 1 {
		t.Errorf("token endpoint called %d times, want 1", n)
	}

	select {
	case tk := <-refreshed:
		if tk.AccessToken != "new-access" || tk.RefreshToken != "new-refresh" {
			t.Errorf("OnTokenRefresh tokens = %+v", tk)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("OnTokenRefresh callback not called")
	}
}

func TestAuthTransportRefreshFailureReturnsOriginal401(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/users/me", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusUnauthorized, 401, "Unauthorized", nil)
	})
	mux.HandleFunc("POST /auth/v1/token", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusBadRequest, 400, "invalid refresh token", nil)
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("old-access", "old-refresh", chzzkgo.Scopes{chzzkgo.UserRead})

	_, err := chzzk.GetUser(context.Background())

	var apiErr *chzzkgo.APIError

	if !errors.As(err, &apiErr) {
		t.Fatalf("want APIError, got %v", err)
	}

	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("StatusCode = %d, want 401", apiErr.StatusCode)
	}

	// refresh 실패 시 토큰이 클리어되어 이후 호출은 ErrNotAuthenticated
	_, err = chzzk.GetUser(context.Background())

	if !errors.Is(err, chzzkgo.ErrNotAuthenticated) {
		t.Errorf("want ErrNotAuthenticated after failed refresh, got %v", err)
	}
}

func TestAuthTransportNotAuthenticated(t *testing.T) {
	// scope 사전 검사가 없는 CreateSessionWithUser로 transport 계층의 검사를 확인한다
	chzzk := newMockClient(t, http.NewServeMux())

	_, err := chzzk.CreateSessionWithUser(context.Background())

	if !errors.Is(err, chzzkgo.ErrNotAuthenticated) {
		t.Fatalf("want ErrNotAuthenticated, got %v", err)
	}
}

func TestWithAccessTokenOverride(t *testing.T) {
	var tokenCalls atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("POST /open/v1/sessions/events/subscribe/chat", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer override-access" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer override-access")
		}

		if got := r.URL.Query().Get("sessionKey"); got != "test-session-key" {
			t.Errorf("sessionKey = %q, want test-session-key", got)
		}

		writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", nil)
	})
	mux.HandleFunc("POST /auth/v1/token", func(w http.ResponseWriter, r *http.Request) {
		tokenCalls.Add(1)
	})

	chzzk := newMockClient(t, mux)
	// scope 없는 토큰 — 오버라이드 시 scope 사전 검사를 건너뛰는 것도 함께 검증
	chzzk.SetTokens("main-access", "main-refresh", chzzkgo.Scopes{})

	ctx := chzzkgo.WithAccessToken(context.Background(), "override-access")

	if err := chzzk.SubscribeChatEvent(ctx, "test-session-key"); err != nil {
		t.Fatal(err)
	}

	if n := tokenCalls.Load(); n != 0 {
		t.Errorf("token endpoint called %d times, want 0", n)
	}
}

func TestWithAccessTokenOverrideNoRefreshOn401(t *testing.T) {
	var tokenCalls atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("POST /open/v1/sessions/events/subscribe/chat", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusUnauthorized, 401, "Unauthorized", nil)
	})
	mux.HandleFunc("POST /auth/v1/token", func(w http.ResponseWriter, r *http.Request) {
		tokenCalls.Add(1)
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("main-access", "main-refresh", chzzkgo.Scopes{})

	ctx := chzzkgo.WithAccessToken(context.Background(), "override-access")
	err := chzzk.SubscribeChatEvent(ctx, "test-session-key")

	var apiErr *chzzkgo.APIError

	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want APIError 401, got %v", err)
	}

	// 오버라이드 토큰의 401은 refresh를 시도하지 않는다
	if n := tokenCalls.Load(); n != 0 {
		t.Errorf("token endpoint called %d times, want 0", n)
	}
}

func TestClientAuthHeaders(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/categories/search", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Client-Id"); got != "test-client-id" {
			t.Errorf("Client-Id = %q, want test-client-id", got)
		}

		if got := r.Header.Get("Client-Secret"); got != "test-client-secret" {
			t.Errorf("Client-Secret = %q, want test-client-secret", got)
		}

		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization = %q, want empty", got)
		}

		if got := r.URL.Query().Get("query"); got != "스포츠" {
			t.Errorf("query = %q, want 스포츠", got)
		}

		writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", map[string]any{
			"data": []map[string]any{{
				"categoryType":   "SPORTS",
				"categoryId":     "soccer",
				"categoryValue":  "축구",
				"posterImageUrl": "https://example.com/soccer.png",
			}},
		})
	})

	// 토큰 미주입 — Client 인증은 OAuth 토큰 없이 동작해야 한다
	chzzk := newMockClient(t, mux)

	pages, err := chzzk.SearchCategory(context.Background(), "스포츠")

	if err != nil {
		t.Fatal(err)
	}

	if len(pages.Data) != 1 || pages.Data[0].CategoryID != "soccer" {
		t.Errorf("pages = %+v", pages)
	}
}

func TestRefreshSingleFlight(t *testing.T) {
	var tokenCalls atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/users/me", func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("Authorization") {
		case "Bearer old-access":
			writeEnvelope(t, w, http.StatusUnauthorized, 401, "Unauthorized", nil)
		default:
			writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", testUserContent())
		}
	})
	mux.HandleFunc("POST /auth/v1/token", func(w http.ResponseWriter, r *http.Request) {
		tokenCalls.Add(1)
		time.Sleep(50 * time.Millisecond) // 경쟁 구간을 넓혀 double-check 검증

		writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", map[string]any{
			"accessToken":  "new-access",
			"refreshToken": "new-refresh",
			"expiresIn":    86400,
			"scope":        "유저 조회",
		})
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("old-access", "old-refresh", chzzkgo.Scopes{chzzkgo.UserRead})

	const workers = 4

	var wg sync.WaitGroup

	errs := make([]error, workers)

	for i := range workers {
		wg.Add(1)

		go func() {
			defer wg.Done()
			_, errs[i] = chzzk.GetUser(context.Background())
		}()
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("worker %d: %v", i, err)
		}
	}

	if n := tokenCalls.Load(); n != 1 {
		t.Errorf("token endpoint called %d times, want 1 (single flight)", n)
	}
}

func TestRetryReplaysRequestBody(t *testing.T) {
	var tokenCalls atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("POST /open/v1/chats/send", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)

		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
		}

		if r.Header.Get("Authorization") == "Bearer old-access" {
			writeEnvelope(t, w, http.StatusUnauthorized, 401, "Unauthorized", nil)
			return
		}

		// 재시도 요청에도 body가 온전히 복원되어야 한다
		var msg map[string]string

		if err := json.Unmarshal(body, &msg); err != nil {
			t.Errorf("Failed to decode retried body %q: %v", body, err)
		}

		if msg["message"] != "replay-me" {
			t.Errorf("retried message = %q, want replay-me", msg["message"])
		}

		writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", map[string]any{"messageId": "m1"})
	})
	mux.HandleFunc("POST /auth/v1/token", newRefreshTokenHandler(t, &tokenCalls, "채팅 메시지 쓰기"))

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("old-access", "old-refresh", chzzkgo.Scopes{chzzkgo.ChatMessageWrite})

	result, err := chzzk.SendChatMessage(context.Background(), "replay-me")

	if err != nil {
		t.Fatal(err)
	}

	if result.MessageID != "m1" {
		t.Errorf("MessageID = %q, want m1", result.MessageID)
	}
}
