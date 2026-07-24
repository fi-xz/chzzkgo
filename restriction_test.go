package chzzkgo_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/fi-xz/chzzkgo"
)

func TestAddRestriction(t *testing.T) {
	var gotBody map[string]string

	mux := http.NewServeMux()
	mux.HandleFunc("POST /open/v1/restrict-channels", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", nil)
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.RestrictionWrite})

	if err := chzzk.AddRestriction(context.Background(), "target-channel-1"); err != nil {
		t.Fatal(err)
	}

	if gotBody["targetChannelId"] != "target-channel-1" {
		t.Errorf("targetChannelId = %q, want target-channel-1", gotBody["targetChannelId"])
	}
}

func TestRemoveRestriction(t *testing.T) {
	var gotBody map[string]string

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /open/v1/restrict-channels", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", nil)
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.RestrictionWrite})

	if err := chzzk.RemoveRestriction(context.Background(), "target-channel-1"); err != nil {
		t.Fatal(err)
	}

	if gotBody["targetChannelId"] != "target-channel-1" {
		t.Errorf("targetChannelId = %q, want target-channel-1", gotBody["targetChannelId"])
	}
}

func TestGetRestrictions(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/restrict-channels", func(w http.ResponseWriter, r *http.Request) {
		serveFixture(t, w, "get_restrictions.json")
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.RestrictionRead})

	restrictions, err := chzzk.GetRestrictions(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	if len(restrictions.Data) == 0 {
		t.Fatal("no restrictions decoded")
	}

	for i, r := range restrictions.Data {
		// ReleaseDate는 영구 제한(null)이면 빈 문자열이 정상이므로 검사하지 않는다
		if r.RestrictedChannelID == "" || r.RestrictedChannelName == "" || r.CreatedDate == "" {
			t.Errorf("Data[%d] has empty fields", i)
		}
	}

	if restrictions.Page.Next == "" {
		t.Error("Page.Next is empty")
	}
}

// TestGetRestrictionsLastPage는 마지막 페이지 응답의 page: null이
// 제로 값으로 디코딩되는지 검증한다.
func TestGetRestrictionsLastPage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/restrict-channels", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", map[string]any{
			"data": []any{},
			"page": nil,
		})
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.RestrictionRead})

	restrictions, err := chzzk.GetRestrictions(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	if len(restrictions.Data) != 0 || restrictions.Page.Next != "" {
		t.Errorf("restrictions = %+v", restrictions)
	}
}

func TestAddTemporaryRestriction(t *testing.T) {
	var gotBody map[string]string

	mux := http.NewServeMux()
	mux.HandleFunc("POST /open/v1/temporary-restrict-channels", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", nil)
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.RestrictionWrite})

	if err := chzzk.AddTemporaryRestriction(context.Background(), "target-channel-1", "chat-channel-1"); err != nil {
		t.Fatal(err)
	}

	if gotBody["targetChannelId"] != "target-channel-1" || gotBody["chatChannelId"] != "chat-channel-1" {
		t.Errorf("request body = %v", gotBody)
	}
}

func TestRemoveTemporaryRestriction(t *testing.T) {
	var gotBody map[string]string

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /open/v1/temporary-restrict-channels", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		writeEnvelope(t, w, http.StatusOK, 200, "SUCCESS", nil)
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.RestrictionWrite})

	if err := chzzk.RemoveTemporaryRestriction(context.Background(), "target-channel-1", "chat-channel-1"); err != nil {
		t.Fatal(err)
	}

	if gotBody["targetChannelId"] != "target-channel-1" || gotBody["chatChannelId"] != "chat-channel-1" {
		t.Errorf("request body = %v", gotBody)
	}
}
