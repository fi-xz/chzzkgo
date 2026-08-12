package chzzkgo_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/fi-xz/chzzkgo"
)

func TestSendChatMessage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /open/v1/chats/send", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		if body["message"] != "테스트 채팅" {
			t.Errorf("message = %q, want 테스트 채팅", body["message"])
		}

		writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", map[string]any{"messageId": "msg-1"})
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.ChatMessageWrite})

	result, err := chzzk.SendChatMessage(context.Background(), "테스트 채팅")

	if err != nil {
		t.Fatal(err)
	}

	if result.MessageID != "msg-1" {
		t.Errorf("MessageID = %q, want msg-1", result.MessageID)
	}
}

// TestSendChatMessageTooLong은 100자(rune 기준) 초과 메시지가 요청 전에 거부되는지 검증한다.
func TestSendChatMessageTooLong(t *testing.T) {
	chzzk := chzzkgo.New("test-client-id", "test-client-secret", "http://localhost:12940/callback")
	chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.ChatMessageWrite})

	// 한글 101자 — 바이트 수가 아니라 rune 수 기준으로 거부되어야 한다
	_, err := chzzk.SendChatMessage(context.Background(), strings.Repeat("가", 101))

	if err == nil {
		t.Fatal("want error for 101-rune message, got nil")
	}

	// 한글 100자는 바이트로는 300이지만 rune 기준 100이므로 길이 검사를 통과해야 한다
	mux := http.NewServeMux()
	mux.HandleFunc("POST /open/v1/chats/send", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", map[string]any{"messageId": "msg-2"})
	})

	mocked := newMockClient(t, mux)
	mocked.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.ChatMessageWrite})

	if _, err := mocked.SendChatMessage(context.Background(), strings.Repeat("가", 100)); err != nil {
		t.Errorf("100-rune message rejected: %v", err)
	}
}

func TestSetChatNotice(t *testing.T) {
	t.Run("메시지 공지", func(t *testing.T) {
		var gotBody map[string]string

		mux := http.NewServeMux()
		mux.HandleFunc("POST /open/v1/chats/notice", func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Errorf("Failed to decode request body: %v", err)
			}

			writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", nil)
		})

		chzzk := newMockClient(t, mux)
		chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.ChatNoticeWrite})

		err := chzzk.SetChatNotice(context.Background(), chzzkgo.ChatNoticeRequest{Message: "테스트 공지"})

		if err != nil {
			t.Fatal(err)
		}

		if gotBody["message"] != "테스트 공지" {
			t.Errorf("message = %q, want 테스트 공지", gotBody["message"])
		}
	})

	t.Run("빈 요청 거부", func(t *testing.T) {
		chzzk := chzzkgo.New("test-client-id", "test-client-secret", "http://localhost:12940/callback")
		chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.ChatNoticeWrite})

		err := chzzk.SetChatNotice(context.Background(), chzzkgo.ChatNoticeRequest{})

		if err == nil {
			t.Fatal("want error for empty request, got nil")
		}
	})
}

func TestGetChatSettings(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/chats/settings", func(w http.ResponseWriter, r *http.Request) {
		serveFixture(t, w, "get_chat_settings.json")
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.ChatSettingsRead})

	settings, err := chzzk.GetChatSettings(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	if settings.ChatAvailableCondition == "" || settings.ChatAvailableGroup == "" {
		t.Errorf("settings = %+v", settings)
	}
}

func TestBlindChatMessage(t *testing.T) {
	t.Run("정상 요청", func(t *testing.T) {
		var gotBody map[string]any

		mux := http.NewServeMux()
		mux.HandleFunc("POST /open/v1/chats/blind-message", func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Errorf("Failed to decode request body: %v", err)
			}

			writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", nil)
		})

		chzzk := newMockClient(t, mux)
		chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.ChatMessageWrite})

		err := chzzk.BlindChatMessage(context.Background(), chzzkgo.ChatBlindRequest{
			ChatChannelID:   "chat-channel-1",
			MessageTime:     1721900000000,
			SenderChannelID: "sender-channel-1",
		})

		if err != nil {
			t.Fatal(err)
		}

		if gotBody["chatChannelId"] != "chat-channel-1" || gotBody["senderChannelId"] != "sender-channel-1" ||
			gotBody["messageTime"] != float64(1721900000000) {
			t.Errorf("request body = %v", gotBody)
		}
	})

	t.Run("필수 필드 누락 거부", func(t *testing.T) {
		chzzk := chzzkgo.New("test-client-id", "test-client-secret", "http://localhost:12940/callback")
		chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.ChatMessageWrite})

		err := chzzk.BlindChatMessage(context.Background(), chzzkgo.ChatBlindRequest{
			ChatChannelID: "chat-channel-1",
		})

		if err == nil {
			t.Fatal("want error for missing fields, got nil")
		}
	})
}
