# chzzkgo

A Go library for streaming platform 치지직(CHZZK).

치지직 Open API의 Go SDK입니다. Go 1.26 이상이 필요합니다.

REST API와 세션 서버의 실시간 이벤트 수신을 모두 지원합니다.
실시간 이벤트에는 Socket.IO 2.x 클라이언트인
[socketio2](https://github.com/fi-xz/socketio2)를 사용합니다.

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
    chzzk := chzzkgo.New("SAMPLE_CLIENT_ID", "SAMPLE_CLIENT_SECRET", "http://localhost:8080/callback")

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
| 인증 | ✅ | `GetAuthorizationURL`, `ExchangeCode`, `RequestToken`, `RevokeToken`, `LoginServer` (`Start` / `LoginHandler` / `CallbackHandler`) |
| 유저 | ✅ | `GetUser` |
| 채널 | ✅ | `GetChannels`, `GetChannelManagers`, `GetChannelFollowers`, `GetChannelSubscribers` |
| 카테고리 | ✅ | `SearchCategory` |
| 라이브 | ✅ | `GetLiveList`, `GetStreamKey`, `GetLiveSettings`, `SetLiveSettings` |
| 채팅 | ✅ | `SendChatMessage`, `SetChatNotice`, `GetChatSettings`, `SetChatSettings`, `BlindChatMessage` |
| 활동제한 | ✅ | `AddRestriction`, `RemoveRestriction`, `GetRestrictions`, `AddTemporaryRestriction`, `RemoveTemporaryRestriction` |
| 세션 (URL 발급·목록·이벤트 구독/해제) | ✅ | `CreateSessionWithClient`, `CreateSessionWithUser`, `GetSessionsWithClient`, `GetSessionsWithUser`, `Subscribe·UnsubscribeChatEvent`, `Subscribe·UnsubscribeDonationEvent`, `Subscribe·UnsubscribeSubscriptionEvent` |
| 세션 (실시간 이벤트 수신) | ✅ | `SessionSocket`, `ConnectSessionWithUser`, `ConnectSessionWithClient` |
| 드롭스 | ❌ | 미구현 |

## OAuth 로그인

사용자 권한이 필요한 API는 OAuth 토큰이 필요합니다. 내장 [LoginServer](server.go)로 로그인 흐름을 처리할 수 있습니다.

```go
chzzk := chzzkgo.New(clientID, clientSecret, "http://localhost:8080/callback")

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

### 기존 서버에 통합

이미 운영 중인 HTTP 서버가 있다면 `Start` 대신 핸들러를 직접 등록할 수 있습니다.
`Start`는 아래 두 핸들러를 자체 서버에 얹어 주는 Wrapper입니다.

```go
server := chzzk.NewLoginServer(
    chzzkgo.WithOnLogin(func(t chzzkgo.Tokens) { /* 토큰 저장 */ }),
)

http.Handle("/auth/chzzk", server.LoginHandler())       // state 발급 + 인가 페이지로 리다이렉트
http.Handle("/callback", server.CallbackHandler())      // state 검증 + 토큰 교환
```

핸들러를 직접 사용할 때 서버의 수명과 포트는 호출자가 관리합니다.
`WithKeepAlive`의 서버 유지 동작은 `Start` 전용이며, 핸들러 사용 시에는 영향이 없습니다.

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

## 실시간 이벤트 수신

세션 서버에 접속하면 채팅·후원·구독 이벤트를 실시간으로 받을 수 있습니다.
세션 URL을 발급받고 소켓에 연결한 뒤, 세션 키로 원하는 이벤트를 구독하는 순서입니다.

```go
socket, err := chzzk.ConnectSessionWithUser(ctx, func(s *chzzkgo.SessionSocket) {
    // 핸들러는 반드시 연결 전에 등록합니다.
    s.OnChat(func(e chzzkgo.ChatEvent) {
        fmt.Printf("%s: %s\n", e.Profile.Nickname, e.Content)
    })
    s.OnDonation(func(e chzzkgo.DonationEvent) {
        fmt.Printf("%s님이 %d원 후원\n", e.DonatorNickname, e.PayAmount)
    })
})

if err != nil {
    return err
}

defer socket.Close()

// 연결만으로는 이벤트가 오지 않습니다. 세션 키로 구독해야 합니다.
if err := chzzk.SubscribeChatEvent(ctx, socket.SessionKey()); err != nil {
    return err
}

<-socket.Done()
return socket.Err()
```

세션 URL을 직접 다루려면 `NewSessionSocket`을 사용합니다.

```go
session, err := chzzk.CreateSessionWithUser(ctx)
socket := chzzkgo.NewSessionSocket(session.URL)
socket.OnChat(...)
err = socket.Connect(ctx)
```

`Connect`는 서버가 세션 키를 보낼 때까지 기다린 뒤 반환하므로, 반환 직후
`SessionKey()`로 구독을 시작할 수 있습니다.

### 재연결은 하지 않습니다

세션 URL은 한 번만 쓸 수 있어 같은 URL로 다시 접속할 수 없습니다.
`Done()`이 닫히면 `Err()`로 원인을 확인하고 세션을 새로 발급받아 다시 연결하세요.
`ConnectSessionWithUser`를 반복 호출하면 발급과 연결을 한 번에 처리할 수 있습니다.

```go
for {
    socket, err := chzzk.ConnectSessionWithUser(ctx, register)
    if err != nil {
        return err
    }

    if err := chzzk.SubscribeChatEvent(ctx, socket.SessionKey()); err != nil {
        return err
    }

    <-socket.Done()
    socket.Close()
}
```

서버가 구독을 취소하면 `OnSystem`으로 `revoked` 시스템 이벤트가 전달됩니다.
이 경우 연결은 살아 있지만 해당 이벤트는 더 이상 오지 않으므로 다시 구독해야 합니다.

### 이벤트 시각

`eventSentAt`은 오프셋 표기 없이 KST로 전달되므로 `EventTime`이 이를 해석합니다.
`ChatEvent.MessageTime`은 epoch 밀리초(UTC)이며 `MessageAt()`으로 변환합니다.
채팅 블라인드에 필요한 값은 `BlindRequest()`로 바로 만들 수 있습니다.

```go
s.OnChat(func(e chzzkgo.ChatEvent) {
    if strings.Contains(e.Content, "금지어") {
        chzzk.BlindChatMessage(ctx, e.BlindRequest())
    }
})
```

## 다른 채널 토큰으로 호출

세션 이벤트 구독 등에서 다른 계정의 액세스 토큰을 일시적으로 사용할 수 있습니다.

```go
ctx := chzzkgo.WithAccessToken(context.Background(), otherChannelToken)
err := chzzk.SubscribeChatEvent(ctx, sessionKey)
```

다중 계정을 다룰 때는 계정당 `Client` 하나를 사용하는 것이 기본 방침이며, 토큰 오버라이드는 위와 같은 특수 케이스 전용입니다.

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

### 구독 이벤트는 미검증입니다

`SubscriptionEvent` 구조체는 공식 문서만을 근거로 정의했습니다.
구독 기능이 치지직 프로 회원에게만 열려 있어 실제 페이로드를 관측하지 못했고,
스튜디오의 구독 알림 테스트는 후원 알림과 달리 세션 소켓으로 전달되지 않습니다.

문서의 타입 표기가 실제와 다른 전례가 있으므로(후원의 `payAmount`는 문서상
문자열이지만 실제로는 숫자입니다) 값이 비어 있다면 `OnAny`로 원본을 확인하세요.

### 드롭스 API는 미구현입니다

## Testing

```sh
go test ./...            # 오프라인 테스트 — 네트워크·계정 불필요, 로컬 mock 서버 사용
go test -tags live ./... # 라이브 스모크 — .test.env에 CLIENT_ID 등 필요 (미설정 시 skip)
```

기본 빌드에는 라이브 테스트가 포함되지 않으므로 `go test ./...`는 항상 안전합니다.

## License

[MIT](./LICENSE)
