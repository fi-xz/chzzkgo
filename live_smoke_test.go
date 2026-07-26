//go:build live

package chzzkgo_test

import (
	"context"
	"testing"
	"time"

	"github.com/fi-xz/chzzkgo"
)

// 라이브 스모크 테스트 — 치지직 서버가 SDK의 계약(경로, 인증, 응답 형태)대로
// 동작하는지 조회성 엔드포인트로만 확인한다. 쓰기성 검증은 수동으로 수행한다.
//
// 실행: go test -tags live -run '^TestLive' -v .
// .test.env에 CLIENT_ID/CLIENT_SECRET(+ Bearer 테스트는 ACCESS_TOKEN/REFRESH_TOKEN) 필요.
//
// 세션을 다루는 테스트의 제약:
//   - 세션 URL은 발급 후 30초 안에 연결해야 하고, 세션 키는 소켓이 연결되어 있는
//     동안에만 유효하다. 둘 다 환경 변수에 둘 수 없으므로 테스트가 직접 발급받는다.
//     연결에 실패해 다시 시도할 때도 같은 URL을 재사용하지 말고 발급부터 다시 한다.
//   - 동시 연결 가능한 세션은 클라이언트 인증 10개, 사용자 인증 3개다.
//     세션 소켓을 여는 테스트에는 t.Parallel을 쓰지 않는다.
//   - 세션은 목록에서 지울 수 없고 끊긴 뒤에도 90일간 조회되므로,
//     실패 경로에서도 소켓을 닫고 걸어 둔 구독을 해제한다.

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

// TestLiveSessionSocket은 실제 세션 서버에 접속해 세션 키를 받고
// 채팅 이벤트를 구독했다가 해제한다.
//
// 채팅이 실제로 오는지는 누군가 방송에 채팅을 보내야 확인할 수 있으므로
// 여기서는 접속·세션 키 수신·구독까지만 검증한다.
func TestLiveSessionSocket(t *testing.T) {
	chzzk := newTestClient(t, chzzkgo.Scopes{chzzkgo.ChatMessageRead})

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	socket, err := chzzk.ConnectSessionWithUser(ctx, func(s *chzzkgo.SessionSocket) {
		s.OnChat(func(e chzzkgo.ChatEvent) {
			t.Logf("CHAT %s: %s", e.Profile.Nickname, e.Content)
		})
		s.OnSystem(func(e chzzkgo.SystemEvent) {
			t.Logf("SYSTEM %s %+v", e.Type, e.Data)
		})
		s.OnError(func(err error) {
			t.Errorf("socket error: %v", err)
		})
	})

	if err != nil {
		t.Fatal(err)
	}

	defer socket.Close()

	sessionKey := socket.SessionKey()

	if sessionKey == "" {
		t.Fatal("SessionKey is empty")
	}

	if err := chzzk.SubscribeChatEvent(ctx, sessionKey); err != nil {
		t.Fatalf("Failed to subscribe chat event: %v", err)
	}

	// 구독 상태를 남기지 않도록 정리한다.
	defer func() {
		if err := chzzk.UnsubscribeChatEvent(context.WithoutCancel(ctx), sessionKey); err != nil {
			t.Errorf("Failed to unsubscribe chat event: %v", err)
		}
	}()

	// 구독 직후 SYSTEM subscribed가 도착할 시간을 잠시 준다.
	select {
	case <-socket.Done():
		t.Fatalf("socket closed: %v", socket.Err())
	case <-time.After(3 * time.Second):
	}
}

// TestLiveSubscribeChatEventWithOverride는 [WithAccessToken] 오버라이드로
// 타 채널 토큰을 사용한 이벤트 구독을 실서버가 수락하는지 검증한다.
// mock으로는 검증 불가능한 유일한 영역 — 서버측 오버라이드 토큰 판정.
//
// OTHER_CHANNEL_ACCESS_TOKEN 미설정 시 skip.
// 세션 키는 소켓이 연결되어 있는 동안에만 유효하므로 여기에서 직접 발급받는다.
// 구독 후 즉시 해제하여 상태를 원복한다.
func TestLiveSubscribeChatEventWithOverride(t *testing.T) {
	otherToken := requireEnv(t, "OTHER_CHANNEL_ACCESS_TOKEN")

	chzzk := newTestClientNoAuth(t)

	connectCtx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	socket, err := chzzk.ConnectSessionWithClient(connectCtx, nil)

	if err != nil {
		t.Fatal(err)
	}

	defer socket.Close()

	sessionKey := socket.SessionKey()
	ctx := chzzkgo.WithAccessToken(connectCtx, otherToken)

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
