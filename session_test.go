package chzzkgo_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/fi-xz/chzzkgo"
)

func TestCreateSessionWithClient(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/sessions/auth/client", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Client-Id"); got != "test-client-id" {
			t.Errorf("Client-Id = %q, want test-client-id", got)
		}

		serveFixture(t, w, "create_session.json")
	})

	// Client 인증 — OAuth 토큰 미주입
	chzzk := newMockClient(t, mux)

	result, err := chzzk.CreateSessionWithClient(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	if result.URL == "" {
		t.Error("session URL is empty")
	}
}

func TestCreateSessionWithUser(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/sessions/auth", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer access" {
			t.Errorf("Authorization = %q, want Bearer access", got)
		}

		serveFixture(t, w, "create_session.json")
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{})

	result, err := chzzk.CreateSessionWithUser(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	if result.URL == "" {
		t.Error("session URL is empty")
	}
}

func TestGetSessionsWithClient(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/sessions/client", func(w http.ResponseWriter, r *http.Request) {
		serveFixture(t, w, "get_sessions.json")
	})

	chzzk := newMockClient(t, mux)

	result, err := chzzk.GetSessionsWithClient(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	if result.TotalCount != 1 || len(result.Data) != 1 {
		t.Fatalf("result = %+v", result)
	}

	session := result.Data[0]

	if session.SessionKey != "session-key-1" {
		t.Errorf("SessionKey = %q", session.SessionKey)
	}

	// disconnectedDate: null → 빈 문자열로 디코딩
	if session.DisconnectedDate != "" {
		t.Errorf("DisconnectedDate = %q, want empty", session.DisconnectedDate)
	}

	if len(session.SubscribedEvents) != 1 || session.SubscribedEvents[0].EventType != chzzkgo.EventTypeChat {
		t.Errorf("SubscribedEvents = %+v", session.SubscribedEvents)
	}
}

func TestGetSessionsWithUser(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/sessions/user", func(w http.ResponseWriter, r *http.Request) {
		serveFixture(t, w, "get_sessions.json")
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{})

	result, err := chzzk.GetSessionsWithUser(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	if result.TotalPages != 1 || len(result.Data) != 1 {
		t.Errorf("result = %+v", result)
	}
}

// TestSessionEventEndpoints는 6개 이벤트 구독/해제 함수의
// 경로·sessionKey 전달·scope 매핑을 전수 검증한다.
func TestSessionEventEndpoints(t *testing.T) {
	cases := []struct {
		name  string
		path  string
		scope chzzkgo.Scope
		call  func(*chzzkgo.Client, context.Context, string) error
	}{
		{"채팅 구독", "/open/v1/sessions/events/subscribe/chat", chzzkgo.ChatMessageRead, (*chzzkgo.Client).SubscribeChatEvent},
		{"채팅 구독 해제", "/open/v1/sessions/events/unsubscribe/chat", chzzkgo.ChatMessageRead, (*chzzkgo.Client).UnsubscribeChatEvent},
		{"후원 구독", "/open/v1/sessions/events/subscribe/donation", chzzkgo.DonationRead, (*chzzkgo.Client).SubscribeDonationEvent},
		{"후원 구독 해제", "/open/v1/sessions/events/unsubscribe/donation", chzzkgo.DonationRead, (*chzzkgo.Client).UnsubscribeDonationEvent},
		{"구독 알림 구독", "/open/v1/sessions/events/subscribe/subscription", chzzkgo.SubscriptionRead, (*chzzkgo.Client).SubscribeSubscriptionEvent},
		{"구독 알림 구독 해제", "/open/v1/sessions/events/unsubscribe/subscription", chzzkgo.SubscriptionRead, (*chzzkgo.Client).UnsubscribeSubscriptionEvent},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var hit bool

			mux := http.NewServeMux()
			mux.HandleFunc("POST "+tc.path, func(w http.ResponseWriter, r *http.Request) {
				hit = true

				if got := r.URL.Query().Get("sessionKey"); got != "test-session-key" {
					t.Errorf("sessionKey = %q, want test-session-key", got)
				}

				writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", nil)
			})

			chzzk := newMockClient(t, mux)
			chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{tc.scope})

			if err := tc.call(chzzk, context.Background(), "test-session-key"); err != nil {
				t.Fatal(err)
			}

			if !hit {
				t.Error("endpoint not called")
			}

			// scope 없는 클라이언트는 요청 전에 MissingScopeError
			noScope := newMockClient(t, mux)
			noScope.SetTokens("access", "refresh", chzzkgo.Scopes{})

			err := tc.call(noScope, context.Background(), "test-session-key")

			var missing *chzzkgo.MissingScopeError

			if !errors.As(err, &missing) || missing.Scope != tc.scope {
				t.Errorf("want MissingScopeError{%q}, got %v", tc.scope, err)
			}
		})
	}
}
