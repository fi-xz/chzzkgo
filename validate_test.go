package chzzkgo_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/fi-xz/chzzkgo"
)

// TestValidateSize는 size 범위 검증이 네트워크 요청 전에 실패하는지 검증한다.
// (클라이언트의 baseURL이 실서버를 가리키지만 요청은 발생하지 않아야 한다)
func TestValidateSize(t *testing.T) {
	chzzk := chzzkgo.NewChzzkClient("test-client-id", "test-client-secret", "http://localhost:12940/callback")

	cases := []struct {
		name    string
		call    func() error
		wantMsg string
	}{
		{
			"GetLiveList 상한 초과",
			func() error {
				_, err := chzzk.GetLiveList(context.Background(), chzzkgo.WithSize(21))
				return err
			},
			"between 1 and 20",
		},
		{
			"GetLiveList 하한 미만",
			func() error {
				_, err := chzzk.GetLiveList(context.Background(), chzzkgo.WithSize(0))
				return err
			},
			"between 1 and 20",
		},
		{
			"SearchCategory 상한 초과",
			func() error {
				_, err := chzzk.SearchCategory(context.Background(), "게임", chzzkgo.WithSize(51))
				return err
			},
			"between 1 and 50",
		},
		{
			"GetSessionsWithClient 상한 초과",
			func() error {
				_, err := chzzk.GetSessionsWithClient(context.Background(), chzzkgo.WithSize(51))
				return err
			},
			"between 1 and 50",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()

			if err == nil {
				t.Fatal("want error, got nil")
			}

			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error = %v, want to contain %q", err, tc.wantMsg)
			}
		})
	}
}

func TestValidateSizePassesValidValue(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/lives", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("size"); got != "20" {
			t.Errorf("size = %q, want 20", got)
		}

		writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", map[string]any{
			"data": []any{},
			"page": map[string]any{"next": ""},
		})
	})

	chzzk := newMockClient(t, mux)

	if _, err := chzzk.GetLiveList(context.Background(), chzzkgo.WithSize(20)); err != nil {
		t.Fatal(err)
	}
}

// TestValidateChatSettings는 허용 목록 밖의 값이 네트워크 요청 전에 거부되는지 검증한다.
func TestValidateChatSettings(t *testing.T) {
	chzzk := chzzkgo.NewChzzkClient("test-client-id", "test-client-secret", "http://localhost:12940/callback")
	chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.ChatSettingsWrite})

	cases := []struct {
		name    string
		patch   chzzkgo.ChatSettingsPatch
		wantMsg string
	}{
		{
			"잘못된 허용 조건",
			chzzkgo.ChatSettingsPatch{ChatAvailableCondition: new(chzzkgo.ChatAvailableCondition("INVALID"))},
			"ChatAvailableCondition",
		},
		{
			"잘못된 허용 그룹",
			chzzkgo.ChatSettingsPatch{ChatAvailableGroup: new(chzzkgo.ChatAvailableGroup("INVALID"))},
			"ChatAvailableGroup",
		},
		{
			"잘못된 팔로우 시간",
			chzzkgo.ChatSettingsPatch{MinFollowerMinute: new(7)},
			"MinFollowerMinute",
		},
		{
			"잘못된 슬로우 모드",
			chzzkgo.ChatSettingsPatch{ChatSlowModeSec: new(4)},
			"ChatSlowModeSec",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := chzzk.SetChatSettings(context.Background(), tc.patch)

			if err == nil {
				t.Fatal("want error, got nil")
			}

			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error = %v, want to contain %q", err, tc.wantMsg)
			}
		})
	}
}

// TestSetChatSettingsSendsOnlySetFields는 nil 필드가 요청 본문에서 제외되는지 검증한다.
// (서버는 전송되지 않은 필드의 기존 값을 유지한다)
func TestSetChatSettingsSendsOnlySetFields(t *testing.T) {
	var gotBody map[string]any

	mux := http.NewServeMux()
	mux.HandleFunc("PUT /open/v1/chats/settings", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", map[string]any{
			"chatAvailableCondition":        "NONE",
			"chatAvailableGroup":            "ALL",
			"minFollowerMinute":             0,
			"allowSubscriberInFollowerMode": false,
			"chatSlowModeSec":               0,
			"chatEmojiMode":                 true,
		})
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.ChatSettingsWrite})

	settings, err := chzzk.SetChatSettings(context.Background(), chzzkgo.ChatSettingsPatch{
		ChatEmojiMode: new(true),
	})

	if err != nil {
		t.Fatal(err)
	}

	if len(gotBody) != 1 {
		t.Errorf("request body has %d fields, want 1: %v", len(gotBody), gotBody)
	}

	if v, ok := gotBody["chatEmojiMode"].(bool); !ok || !v {
		t.Errorf("chatEmojiMode = %v, want true", gotBody["chatEmojiMode"])
	}

	if !settings.ChatEmojiMode {
		t.Errorf("response ChatEmojiMode = false, want true")
	}
}
