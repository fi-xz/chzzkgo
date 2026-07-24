package chzzkgo_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/fi-xz/chzzkgo"
)

func TestGetLiveList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/lives", func(w http.ResponseWriter, r *http.Request) {
		serveFixture(t, w, "get_live_list.json")
	})

	// Client 인증 — OAuth 토큰 미주입
	chzzk := newMockClient(t, mux)

	lives, err := chzzk.GetLiveList(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	if len(lives.Data) == 0 {
		t.Fatal("no lives decoded")
	}

	live := lives.Data[0]

	if live.LiveID == 0 || live.LiveTitle == "" || live.ChannelID == "" || live.ChannelName == "" {
		t.Error("Data[0] has empty fields")
	}

	if lives.Page.Next == "" {
		t.Error("Page.Next is empty")
	}
}

func TestGetStreamKey(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/streams/key", func(w http.ResponseWriter, r *http.Request) {
		serveFixture(t, w, "get_stream_key.json")
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.LiveStreamKeyRead})

	streamKey, err := chzzk.GetStreamKey(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	if streamKey.StreamKey != "dummy-stream-key" {
		t.Errorf("StreamKey = %q", streamKey.StreamKey)
	}
}

func TestGetLiveSettings(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/lives/setting", func(w http.ResponseWriter, r *http.Request) {
		serveFixture(t, w, "get_live_settings.json")
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.LiveSettingRead})

	settings, err := chzzk.GetLiveSettings(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	if settings.DefaultLiveTitle == "" {
		t.Error("DefaultLiveTitle is empty")
	}

	// 중첩 Category 디코딩 확인
	if settings.Category.CategoryID == "" || settings.Category.CategoryType == "" {
		t.Error("Category has empty fields")
	}
}

func TestSetLiveSettings(t *testing.T) {
	var gotBody map[string]any

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /open/v1/lives/setting", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", nil)
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.LiveSettingWrite})

	err := chzzk.SetLiveSettings(context.Background(), chzzkgo.LiveSettingsPatch{
		DefaultLiveTitle: new("새 방송 제목"),
		Tags:             new([]string{"태그1", "태그2"}),
	})

	if err != nil {
		t.Fatal(err)
	}

	// nil 필드(categoryType, categoryId)는 전송되지 않아야 한다
	if len(gotBody) != 2 {
		t.Errorf("request body has %d fields, want 2: %v", len(gotBody), gotBody)
	}

	if gotBody["defaultLiveTitle"] != "새 방송 제목" {
		t.Errorf("defaultLiveTitle = %v", gotBody["defaultLiveTitle"])
	}
}
