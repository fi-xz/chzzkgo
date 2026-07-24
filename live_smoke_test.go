//go:build live

package chzzkgo_test

import (
	"context"
	"testing"

	"github.com/fi-xz/chzzkgo"
)

// 라이브 스모크 테스트 — 치지직 서버가 SDK의 계약(경로, 인증, 응답 형태)대로
// 동작하는지 조회성 엔드포인트로만 확인한다. 쓰기성 검증은 수동으로 수행한다.
//
// 실행: go test -tags live -run '^TestLive' -v .
// .test.env에 CLIENT_ID/CLIENT_SECRET(+ Bearer 테스트는 ACCESS_TOKEN/REFRESH_TOKEN) 필요.

func TestLiveGetUser(t *testing.T) {
	chzzk := newTestClient(t, chzzkgo.Scopes{chzzkgo.UserRead})

	user, err := chzzk.GetUser(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	if user.ChannelID == "" {
		t.Error("ChannelID is empty")
	}
}

func TestLiveGetChannels(t *testing.T) {
	chzzk := newTestClientNoAuth(t)

	channels, err := chzzk.GetChannels(context.Background(), []string{"c42cd75ec4855a9edf204a407c3c1dd2"})

	if err != nil {
		t.Fatal(err)
	}

	if len(channels.Data) == 0 {
		t.Fatal("no channels returned")
	}

	if channels.Data[0].ChannelID != "c42cd75ec4855a9edf204a407c3c1dd2" || channels.Data[0].ChannelName == "" {
		t.Errorf("Data[0] = %+v", channels.Data[0])
	}
}

func TestLiveSearchCategory(t *testing.T) {
	chzzk := newTestClientNoAuth(t)

	categories, err := chzzk.SearchCategory(context.Background(), "게임")

	if err != nil {
		t.Fatal(err)
	}

	if len(categories.Data) == 0 {
		t.Fatal("no categories returned")
	}

	if categories.Data[0].CategoryID == "" || categories.Data[0].CategoryValue == "" {
		t.Errorf("Data[0] = %+v", categories.Data[0])
	}
}

// TestLiveSubscribeChatEventWithOverride는 [WithAccessToken] 오버라이드로
// 타 채널 토큰을 사용한 이벤트 구독을 실서버가 수락하는지 검증한다.
// mock으로는 검증 불가능한 유일한 영역 — 서버측 오버라이드 토큰 판정.
//
// TEST_SESSION_KEY, OTHER_CHANNEL_ACCESS_TOKEN 미설정 시 skip.
// 구독 후 즉시 해제하여 상태를 원복한다.
func TestLiveSubscribeChatEventWithOverride(t *testing.T) {
	sessionKey := requireEnv(t, "TEST_SESSION_KEY")
	otherToken := requireEnv(t, "OTHER_CHANNEL_ACCESS_TOKEN")

	chzzk := newTestClientNoAuth(t)
	ctx := chzzkgo.WithAccessToken(context.Background(), otherToken)

	if err := chzzk.SubscribeChatEvent(ctx, sessionKey); err != nil {
		t.Fatalf("Failed to subscribe with override token: %v", err)
	}

	if err := chzzk.UnsubscribeChatEvent(ctx, sessionKey); err != nil {
		t.Errorf("Failed to unsubscribe with override token: %v", err)
	}
}

func TestLiveGetLiveList(t *testing.T) {
	chzzk := newTestClientNoAuth(t)

	lives, err := chzzk.GetLiveList(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	if len(lives.Data) == 0 {
		t.Log("no live streams at the moment")
		return
	}

	if lives.Data[0].ChannelID == "" {
		t.Errorf("Data[0] = %+v", lives.Data[0])
	}
}
