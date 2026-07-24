# chzzkgo

A Go library for streaming platform 치지직(CHZZK).

치지직 Open API의 Go SDK입니다. Go 1.26 이상이 필요합니다.

> 이 프로젝트는 네이버(NAVER) 및 치지직(CHZZK)의 공식 라이브러리가 아닙니다.

## Installation

```sh
go get github.com/fi-xz/chzzkgo
```

## Quickstart

Client 인증(Client ID/Secret)만으로 호출 가능한 API는 토큰 없이 바로 사용할 수 있습니다.

```go
package main

import (
    "context"
    "fmt"

    "github.com/fi-xz/chzzkgo"
)

func main() {
    chzzk := chzzkgo.NewChzzkClient("SAMPLE_CLIENT_ID", "SAMPLE_CLIENT_SECRET", "http://localhost:8080/callback")

    lives, err := chzzk.GetLiveList(context.Background())

    if err != nil {
        panic(err)
    }

    for _, live := range lives.Data {
        fmt.Println(live.ChannelName, "-", live.LiveTitle)
    }
}
```

Client 인증 API: `GetChannels`, `SearchCategory`, `GetLiveList`, `CreateSessionWithClient`, `GetSessionsWithClient` 등.

## API 커버리지

| 카테고리 | 상태 | 메서드 |
|---|---|---|
| 인증 | ✅ | `GetAuthorizationURL`, `ExchangeCode`, `RequestToken`, `RevokeToken`, `LoginServer` |
| 유저 | ✅ | `GetUser` |
| 채널 | ✅ | `GetChannels`, `GetChannelManagers`, `GetChannelFollowers`, `GetChannelSubscribers` |
| 카테고리 | ✅ | `SearchCategory` |
| 라이브 | ✅ | `GetLiveList`, `GetStreamKey`, `GetLiveSettings`, `SetLiveSettings` |
| 채팅 | ✅ | `SendChatMessage`, `SetChatNotice`, `GetChatSettings`, `SetChatSettings`, `BlindChatMessage` |
| 활동제한 | ✅ | `AddRestriction`, `RemoveRestriction`, `GetRestrictions`, `AddTemporaryRestriction`, `RemoveTemporaryRestriction` |
| 세션 (URL 발급·목록·이벤트 구독/해제) | ✅ | `CreateSessionWithClient`, `CreateSessionWithUser`, `GetSessionsWithClient`, `GetSessionsWithUser`, `Subscribe·UnsubscribeChatEvent`, `Subscribe·UnsubscribeDonationEvent`, `Subscribe·UnsubscribeSubscriptionEvent` |
| 세션 (실시간 이벤트 수신) | ❌ | [알려진 제한](#알려진-제한) 참고 |
| 드롭스 | ❌ | 미구현 |

## OAuth 로그인

사용자 권한이 필요한 API는 OAuth 토큰이 필요합니다. 내장 [LoginServer](server.go)로 로그인 흐름을 처리할 수 있습니다.

```go
chzzk := chzzkgo.NewChzzkClient(clientID, clientSecret, "http://localhost:8080/callback")

// http://localhost:8080/login 접속 → 치지직 로그인 → 첫 로그인 성공 시 서버 자동 종료
tokens, err := chzzk.NewLoginServer().Start(context.Background())

if err != nil {
    panic(err)
}

// 일회용 모드에서는 발급된 토큰이 클라이언트에 자동 주입됩니다
user, err := chzzk.GetUser(context.Background())
```

여러 사용자의 로그인을 상시로 받는 endpoint는 `WithKeepAlive`와 `WithOnLogin`을 사용합니다.
이 경우 토큰은 클라이언트에 주입되지 않고 콜백으로만 전달됩니다.

```go
server := chzzk.NewLoginServer(
    chzzkgo.WithKeepAlive(),
    chzzkgo.WithOnLogin(func(t chzzkgo.Tokens) { /* 계정별 토큰 저장 */ }),
)
_, err := server.Start(ctx) // ctx 취소 전까지 유지
```

## 토큰 저장과 복원

토큰의 저장과 복원은 라이브러리 사용자가 직접 진행합니다.

```go
// 저장해 둔 토큰 복원
chzzk.SetTokens(accessToken, refreshToken, chzzkgo.ParseScopes("유저 조회 채팅 메시지 쓰기"))

// 액세스 토큰 만료 시 자동 갱신됨 — 갱신된 토큰을 콜백으로 받아 저장
chzzk.OnTokenRefresh(func(t chzzkgo.Tokens) {
    saveToStorage(t) // t를 JSON으로 저장하면 Scope까지 원형 유지됨
})
```

## Scope

치지직은 권한 이름으로 한국어 문자열을 사용하며, SDK는 이를 상수로 제공합니다. (`chzzkgo.UserRead` = "유저 조회" 등)
필요 권한이 토큰에 없으면 API 호출 전에 `MissingScopeError`를 반환합니다. 이는 편의를 위한 사전 검사이며, 최종 판정은 서버가 수행합니다.

## 설정 변경 (부분 업데이트)

방송/채팅 설정 변경은 포인터 필드 구조체를 사용합니다. nil 필드는 전송되지 않아 기존 값이 유지됩니다.
값 지정에는 내장 함수 `new`를 사용합니다.

```go
err := chzzk.SetLiveSettings(ctx, chzzkgo.LiveSettingsPatch{
    DefaultLiveTitle: new("새 방송 제목"),
    Tags:             new([]string{"개발자"}),
})
```

## 다른 채널 토큰으로 호출

세션 이벤트 구독 등에서 다른 계정의 액세스 토큰을 일시적으로 사용할 수 있습니다.

```go
ctx := chzzkgo.WithAccessToken(context.Background(), otherChannelToken)
err := chzzk.SubscribeChatEvent(ctx, sessionKey)
```

다중 계정을 다룰 때는 계정당 `ChzzkClient` 하나를 사용하는 것이 기본 방침이며, 토큰 오버라이드는 위와 같은 특수 케이스 전용입니다.

## 에러 처리

```go
user, err := chzzk.GetUser(ctx)

var apiErr *chzzkgo.APIError
var missing *chzzkgo.MissingScopeError

switch {
case errors.Is(err, chzzkgo.ErrNotAuthenticated):
    // 토큰 미설정 — OAuth 로그인 필요
case errors.As(err, &missing):
    // 권한 부족: missing.Scope
case errors.As(err, &apiErr):
    // 서버 오류: apiErr.StatusCode, apiErr.Code, apiErr.Message
}
```

## 알려진 제한

### 세션 실시간 이벤트 (Socket.IO)

치지직 세션 API의 실시간 이벤트 수신은 Socket.IO 2.x 프로토콜을 사용합니다.
현재 Go 생태계에 Socket.IO 클라이언트 2.x를 안정적으로 지원하는 라이브러리가
없어, 소켓 연결 기능은 미구현 상태입니다.

세션 URL 발급(`CreateSessionWithClient` 등)과 이벤트 구독/해제(`SubscribeChatEvent` 등)는
구현되어 있으므로, 소켓 연결 자체는 별도 Socket.IO 클라이언트로 직접 처리해야
합니다. Go에서 안정적인 2.x 클라이언트가 확보되면 지원 예정입니다.

## Testing

```sh
go test ./...            # 오프라인 테스트 — 네트워크·계정 불필요, 로컬 mock 서버 사용
go test -tags live ./... # 라이브 스모크 — .test.env에 CLIENT_ID 등 필요 (미설정 시 skip)
```

기본 빌드에는 라이브 테스트가 포함되지 않으므로 `go test ./...`는 항상 안전합니다.

## License

[MIT](./LICENSE)
