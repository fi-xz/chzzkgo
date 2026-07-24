package chzzkgo_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/fi-xz/chzzkgo"
)

func TestGetChannels(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/channels", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Client-Id"); got != "test-client-id" {
			t.Errorf("Client-Id = %q, want test-client-id", got)
		}

		if got := r.URL.Query().Get("channelIds"); got != "channel-1,channel-2" {
			t.Errorf("channelIds = %q, want channel-1,channel-2", got)
		}

		serveFixture(t, w, "get_channels.json")
	})

	// Client 인증 — OAuth 토큰 미주입
	chzzk := newMockClient(t, mux)

	channels, err := chzzk.GetChannels(context.Background(), []string{"channel-1", "channel-2"})

	if err != nil {
		t.Fatal(err)
	}

	if len(channels.Data) != 2 {
		t.Fatalf("len(Data) = %d, want 2", len(channels.Data))
	}

	first := channels.Data[0]

	if first.ChannelID != "c42cd75ec4855a9edf204a407c3c1dd2" || first.ChannelName != "치지직" || !first.VerifiedMark {
		t.Errorf("Data[0] = %+v", first)
	}
}

func TestGetChannelManagers(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/channels/streaming-roles", func(w http.ResponseWriter, r *http.Request) {
		serveFixture(t, w, "get_channel_managers.json")
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.ChannelManagerRead})

	managers, err := chzzk.GetChannelManagers(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	if len(managers.Data) == 0 {
		t.Fatal("no managers decoded")
	}

	hasStreamer := false

	for i, m := range managers.Data {
		if m.ManagerChannelID == "" || m.ManagerChannelName == "" || m.CreatedDate == "" || m.UserRole == "" {
			t.Errorf("Data[%d] has empty fields", i)
		}

		if m.UserRole == chzzkgo.Streamer {
			hasStreamer = true
		}
	}

	// 미문서 역할 "STREAMER"가 [Streamer] 상수로 디코딩되는지 확인
	if !hasStreamer {
		t.Error("no manager with STREAMER role decoded")
	}
}

func TestGetChannelFollowers(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/channels/followers", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		if q.Get("size") != "50" || q.Get("page") != "2" {
			t.Errorf("query = %v, want size=50 page=2", q)
		}

		serveFixture(t, w, "get_channel_followers.json")
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.ChannelInfoRead})

	followers, err := chzzk.GetChannelFollowers(context.Background(), chzzkgo.WithSize(50), chzzkgo.WithPage(2))

	if err != nil {
		t.Fatal(err)
	}

	if len(followers.Data) == 0 || followers.TotalCount == 0 {
		t.Fatalf("followers empty: len(Data) = %d, TotalCount = %d", len(followers.Data), followers.TotalCount)
	}

	for i, f := range followers.Data {
		if f.ChannelID == "" || f.ChannelName == "" || f.CreatedDate == "" {
			t.Errorf("Data[%d] has empty fields", i)
		}
	}
}

func TestGetChannelSubscribers(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/channels/subscribers", func(w http.ResponseWriter, r *http.Request) {
		serveFixture(t, w, "get_channel_subscribers.json")
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.ChannelInfoRead})

	subscribers, err := chzzk.GetChannelSubscribers(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	// 빈 목록도 정상 케이스 — 디코딩 성공과 필드 형태만 검증
	for i, sub := range subscribers.Data {
		if sub.ChannelID == "" || sub.CreatedDate == "" {
			t.Errorf("Data[%d] has empty fields", i)
		}
	}
}
