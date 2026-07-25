package chzzkgo_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/fi-xz/chzzkgo"
)

// newTokenEndpointMux는 인증 코드를 토큰으로 교환해 주는 mock 서버 핸들러를 반환한다.
// 토큰 주입 여부를 확인할 수 있도록 users/me 엔드포인트도 함께 제공한다.
func newTokenEndpointMux(t *testing.T) *http.ServeMux {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/v1/token", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", map[string]any{
			"accessToken":  "issued-access",
			"refreshToken": "issued-refresh",
			"expiresIn":    86400,
			"scope":        "유저 조회",
		})
	})
	mux.HandleFunc("GET /open/v1/users/me", func(w http.ResponseWriter, r *http.Request) {
		serveFixture(t, w, "get_user.json")
	})

	return mux
}

// issueState는 LoginHandler를 호출해 발급된 state 값을 반환한다.
func issueState(t *testing.T, server *chzzkgo.LoginServer) string {
	t.Helper()

	rec := httptest.NewRecorder()
	server.LoginHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))

	if rec.Code != http.StatusFound {
		t.Fatalf("LoginHandler status = %d, want 302", rec.Code)
	}

	loc, err := url.Parse(rec.Header().Get("Location"))

	if err != nil {
		t.Fatal(err)
	}

	state := loc.Query().Get("state")

	if state == "" {
		t.Fatal("state is empty")
	}

	return state
}

func TestLoginHandler(t *testing.T) {
	chzzk := newMockClient(t, http.NewServeMux())
	server := chzzk.NewLoginServer()

	rec := httptest.NewRecorder()
	server.LoginHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}

	raw := rec.Header().Get("Location")

	// 인가 URL에는 clientSecret이 절대 포함되면 안 된다
	if strings.Contains(raw, "test-client-secret") {
		t.Fatalf("Location must not contain client secret: %s", raw)
	}

	loc, err := url.Parse(raw)

	if err != nil {
		t.Fatal(err)
	}

	if loc.Host != "chzzk.naver.com" || loc.Path != "/account-interlock" {
		t.Errorf("Location = %s", raw)
	}

	if loc.Query().Get("state") == "" {
		t.Error("state is empty")
	}

	// 매 요청마다 서로 다른 state가 발급되어야 한다
	if second := issueState(t, server); second == loc.Query().Get("state") {
		t.Error("state is not unique per request")
	}
}

func TestCallbackHandler(t *testing.T) {
	chzzk := newMockClient(t, newTokenEndpointMux(t))

	loggedIn := make(chan chzzkgo.Tokens, 1)
	server := chzzk.NewLoginServer(chzzkgo.WithOnLogin(func(tk chzzkgo.Tokens) { loggedIn <- tk }))

	state := issueState(t, server)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/callback?code=test-code&state="+state, nil)
	server.CallbackHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "로그인 완료") {
		t.Errorf("body = %s", rec.Body.String())
	}

	// 일회용 모드(기본값)에서는 발급된 토큰이 클라이언트에 주입된다
	if _, err := chzzk.GetUser(t.Context()); err != nil {
		t.Errorf("tokens not injected into client: %v", err)
	}

	select {
	case tk := <-loggedIn:
		if tk.AccessToken != "issued-access" || !tk.Scope.Has(chzzkgo.UserRead) {
			t.Errorf("OnLogin tokens = %+v", tk)
		}
	case <-time.After(2 * time.Second):
		t.Error("OnLogin callback not called")
	}
}

func TestCallbackHandlerRejectsInvalidState(t *testing.T) {
	chzzk := newMockClient(t, newTokenEndpointMux(t))
	server := chzzk.NewLoginServer()

	cases := []struct {
		name  string
		query string
	}{
		{"code 누락", "?state=some-state"},
		{"state 누락", "?code=test-code"},
		{"발급되지 않은 state", "?code=test-code&state=forged-state"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			server.CallbackHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/callback"+tc.query, nil))

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
		})
	}
}

// TestCallbackHandlerStateIsSingleUse는 state가 일회용으로 소비되는지 검증한다.
func TestCallbackHandlerStateIsSingleUse(t *testing.T) {
	chzzk := newMockClient(t, newTokenEndpointMux(t))
	server := chzzk.NewLoginServer()

	state := issueState(t, server)
	target := "/callback?code=test-code&state=" + state

	first := httptest.NewRecorder()
	server.CallbackHandler().ServeHTTP(first, httptest.NewRequest(http.MethodGet, target, nil))

	if first.Code != http.StatusOK {
		t.Fatalf("first attempt status = %d, want 200", first.Code)
	}

	second := httptest.NewRecorder()
	server.CallbackHandler().ServeHTTP(second, httptest.NewRequest(http.MethodGet, target, nil))

	if second.Code != http.StatusBadRequest {
		t.Errorf("replayed state status = %d, want 400", second.Code)
	}
}

// TestCallbackHandlerKeepAliveDoesNotInjectTokens는 상시 모드에서
// 토큰이 클라이언트에 주입되지 않고 콜백으로만 전달되는지 검증한다.
func TestCallbackHandlerKeepAliveDoesNotInjectTokens(t *testing.T) {
	chzzk := newMockClient(t, newTokenEndpointMux(t))

	loggedIn := make(chan chzzkgo.Tokens, 1)
	server := chzzk.NewLoginServer(
		chzzkgo.WithKeepAlive(),
		chzzkgo.WithOnLogin(func(tk chzzkgo.Tokens) { loggedIn <- tk }),
	)

	state := issueState(t, server)

	rec := httptest.NewRecorder()
	server.CallbackHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/callback?code=test-code&state="+state, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	select {
	case tk := <-loggedIn:
		if tk.AccessToken != "issued-access" {
			t.Errorf("OnLogin tokens = %+v", tk)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("OnLogin callback not called")
	}

	// 상시 모드는 다중 계정 전제 — 클라이언트 토큰이 오염되면 안 된다
	if _, err := chzzk.GetUser(t.Context()); !errors.Is(err, chzzkgo.ErrNotAuthenticated) {
		t.Errorf("want ErrNotAuthenticated in keep-alive mode, got %v", err)
	}
}

func TestCallbackHandlerUsesCustomSuccessPage(t *testing.T) {
	chzzk := newMockClient(t, newTokenEndpointMux(t))
	server := chzzk.NewLoginServer(chzzkgo.WithSuccessPage("<html><body>커스텀 완료</body></html>"))

	state := issueState(t, server)

	rec := httptest.NewRecorder()
	server.CallbackHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/callback?code=test-code&state="+state, nil))

	if !strings.Contains(rec.Body.String(), "커스텀 완료") {
		t.Errorf("body = %s", rec.Body.String())
	}
}

// TestStartRequiresExplicitPort는 RedirectURI에 포트가 없으면
// 서버를 열기 전에 오류를 반환하는지 검증한다.
func TestStartRequiresExplicitPort(t *testing.T) {
	chzzk := chzzkgo.NewChzzkClient("test-client-id", "test-client-secret", "http://localhost/callback")

	_, err := chzzk.NewLoginServer().Start(t.Context())

	if err == nil {
		t.Fatal("want error for RedirectURI without port, got nil")
	}

	if !strings.Contains(err.Error(), "port") {
		t.Errorf("error = %v, want to mention port", err)
	}
}
