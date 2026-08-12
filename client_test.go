package chzzkgo_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fi-xz/chzzkgo"
)

// TestNewWithoutAuth는 자격 증명 없이 만든 클라이언트도 인증 계층이 온전히 연결되어
// 나중에 채워 넣은 값으로 요청이 나가는지 검증한다.
func TestNewWithoutAuth(t *testing.T) {
	var gotClientID, gotClientSecret string

	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/sessions/auth/client", func(w http.ResponseWriter, r *http.Request) {
		gotClientID = r.Header.Get("Client-Id")
		gotClientSecret = r.Header.Get("Client-Secret")
		writeEnvelope(t, w, http.StatusOK, 200, "OK", map[string]any{"url": "wss://example.invalid/socket"})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	chzzk := chzzkgo.NewWithoutAuth()
	chzzk.ClientID = "test-client-id"
	chzzk.ClientSecret = "test-client-secret"
	chzzk.SetBaseURL(srv.URL)

	// 인증 정보를 나중에 채워도 정상 동작해야 한다 — authManager나 authTransport가
	// 빠져 있으면 여기에서 nil 역참조로 panic하거나 헤더가 비어서 나간다.
	chzzk.SetLogger(nil)

	if _, err := chzzk.CreateSessionWithClient(context.Background()); err != nil {
		t.Fatal(err)
	}

	if gotClientID != "test-client-id" || gotClientSecret != "test-client-secret" {
		t.Errorf("Client auth headers = %q/%q, want the values assigned after construction", gotClientID, gotClientSecret)
	}
}

// TestSetTokenFields는 토큰 구성 요소를 개별 setter로 채워도
// [chzzkgo.Client.SetTokens]와 같은 상태가 되는지 검증한다.
func TestSetTokenFields(t *testing.T) {
	var gotAuthorization string

	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/users/me", func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		serveFixture(t, w, "get_user.json")
	})

	chzzk := newMockClient(t, mux)

	chzzk.SetAccessToken("access")
	chzzk.SetRefreshToken("refresh")

	// scope를 아직 채우지 않았으므로 사전 검사에서 막혀야 한다.
	_, err := chzzk.GetUser(context.Background())

	var scopeErr *chzzkgo.MissingScopeError

	if !errors.As(err, &scopeErr) {
		t.Fatalf("GetUser error = %v, want MissingScopeError", err)
	}

	chzzk.SetScopes(chzzkgo.Scopes{chzzkgo.UserRead})

	if _, err := chzzk.GetUser(context.Background()); err != nil {
		t.Fatal(err)
	}

	if gotAuthorization != "Bearer access" {
		t.Errorf("Authorization = %q, want %q", gotAuthorization, "Bearer access")
	}
}

// TestSetTokenFieldsOnFreshClient는 SetTokens를 거치지 않은 클라이언트에
// 개별 setter를 먼저 호출해도 안전한지 검증한다.
func TestSetTokenFieldsOnFreshClient(t *testing.T) {
	chzzk := chzzkgo.NewWithoutAuth()

	chzzk.SetScopes(chzzkgo.Scopes{chzzkgo.UserRead})
	chzzk.SetRefreshToken("refresh")

	// 액세스 토큰이 아직 없으므로 인증 오류여야 한다. (panic이 아니라)
	if _, err := chzzk.GetUser(context.Background()); !errors.Is(err, chzzkgo.ErrNotAuthenticated) {
		t.Fatalf("GetUser error = %v, want ErrNotAuthenticated", err)
	}
}
