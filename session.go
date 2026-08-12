package chzzkgo

import (
	"context"
	"net/http"
	"net/url"
)

// SessionURLResponse는 세션 생성 시 반환되는 URL 정보를 담는 구조체이다.
type SessionURLResponse struct {
	// 생성된 세션 URL
	URL string `json:"url"`
}

// Session은 세션 정보를 나타낸다.
type Session struct {
	// 세션 키
	SessionKey string `json:"sessionKey"`
	// 연결된 날짜
	ConnectedDate string `json:"connectedDate"`
	// 연결 해제된 날짜, 연결이 유지 중인 경우 null
	DisconnectedDate string `json:"disconnectedDate,omitempty"`
	// 구독된 이벤트 목록
	SubscribedEvents []Events `json:"subscribedEvents"`
}

// Events는 세션에서 구독된 이벤트 정보를 나타낸다.
type Events struct {
	// 이벤트 종류, [Event] 참고
	EventType Event `json:"eventType"`
	// 이벤트 구독 대상 채널 ID
	ChannelID string `json:"channelId"`
}

// Event는 세션에서 구독된 이벤트 종류를 나타낸다.
type Event string

const (
	// EventTypeChat은 채팅 이벤트이다.
	EventTypeChat Event = "CHAT"
	// EventTypeDonation은 후원 알림 이벤트이다.
	EventTypeDonation Event = "DONATION"
	// EventTypeSubscription은 구독 알림 이벤트이다.
	EventTypeSubscription Event = "SUBSCRIPTION"
)

// SessionPages는 세션 목록 결과를 담는 구조체이다.
type SessionPages struct {
	// 세션 정보 목록
	Data []Session `json:"data"`
	// 현재 페이지 번호
	Page int `json:"page"`
	// 총 세션 개수
	TotalCount int `json:"totalCount"`
	// 총 페이지 개수
	TotalPages int `json:"totalPages"`
}

// CreateSessionWithClient은 클라이언트 인증 방식으로 세션을 생성하고, 세션 URL을 반환한다.
//
// Client 인증을 사용하므로 OAuth 토큰 없이 호출 가능하다.
func (c *Client) CreateSessionWithClient(ctx context.Context) (*SessionURLResponse, error) {
	return getWithClient[SessionURLResponse](c, ctx, "/open/v1/sessions/auth/client", nil)
}

// CreateSessionWithUser은 사용자 인증 방식으로 세션을 생성하고, 세션 URL을 반환한다.
func (c *Client) CreateSessionWithUser(ctx context.Context) (*SessionURLResponse, error) {
	return get[SessionURLResponse](c, ctx, "/open/v1/sessions/auth", nil)
}

// GetSessionsWithClient은 클라이언트 인증 방식으로 세션 목록을 조회한다.
//
// Client 인증을 사용하므로 OAuth 토큰 없이 호출 가능하다.
// 선택적 파라미터로 size, page를 지정할 수 있다. [WithSize], [WithPage]를 참고.
// size의 경우, 지정되지 않았다면 서버에서 기본값 20을 사용하며, 최소 1에서 최대 50까지 지정 가능하다.
// page는 0부터 시작하며, 지정되지 않았다면 서버에서 기본값 0을 사용한다.
func (c *Client) GetSessionsWithClient(ctx context.Context, opts ...QueryOption) (*SessionPages, error) {
	q := buildQuery(opts...)

	if err := validateSize(q, 1, 50); err != nil {
		return nil, err
	}

	return getWithClient[SessionPages](c, ctx, "/open/v1/sessions/client", q)
}

// GetSessionsWithUser는 사용자 인증 방식으로 세션 목록을 조회한다.
//
// 선택적 파라미터로 size, page를 지정할 수 있다. [WithSize], [WithPage]를 참고.
// size의 경우, 지정되지 않았다면 서버에서 기본값 20을 사용하며, 최소 1에서 최대 50까지 지정 가능하다.
// page는 0부터 시작하며, 지정되지 않았다면 서버에서 기본값 0을 사용한다.
func (c *Client) GetSessionsWithUser(ctx context.Context, opts ...QueryOption) (*SessionPages, error) {
	q := buildQuery(opts...)

	if err := validateSize(q, 1, 50); err != nil {
		return nil, err
	}

	return get[SessionPages](c, ctx, "/open/v1/sessions/user", q)
}

// sessionEvent는 세션 이벤트 구독/해제 요청을 전송한다.
// 토큰 오버라이드([WithAccessToken])가 있으면 scope 사전 검사를 건너뛰고 서버 판정에 맡긴다.
func (c *Client) sessionEvent(ctx context.Context, path string, scope Scope, sessionKey string) error {
	if _, overridden := tokenOverrideOf(ctx); !overridden {
		if err := c.requireScope(scope); err != nil {
			return err
		}
	}

	q := url.Values{}
	q.Set("sessionKey", sessionKey)

	_, err := do[empty](c, ctx, http.MethodPost, path, q, nil)

	if err != nil {
		return err
	}

	return nil
}

// SubscribeChatEvent는 세션에서 채팅 이벤트를 구독한다.
//
// 이벤트 구독을 위해서는 세션 키(sessionKey)가 필요하다. 해당 값은 세션 생성 시 반환되는 URL에서 확인할 수 있다.
// (https://chzzk.gitbook.io/chzzk/chzzk-api/session#undefined)
// (https://chzzk.gitbook.io/chzzk/chzzk-api/session#undefined-1)
//
// [ChatMessageRead](채팅 메시지 조회) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
//
// 세션 구독 Event의 경우, Client 인증 방식일 경우 다른 사람의 Access Token을 사용하여 호출 해 타 채널의 이벤트를 구독할 수 있다.
// 따라서, Client 인증 방식으로 호출 시 타 채널의 이벤트를 구독하고 싶은 경우 Access Token을 Override하여 호출하는 것을 권장한다. [WithAccessToken] 참고.
//
//	chzzkgo.SubscribeChatEvent(ctx, sessionKey) // 현재 인증된 사용자의 채팅 이벤트 구독
//
//	chzzkgo.SubscribeChatEvent(chzzkgo.WithAccessToken(ctx, otherChannelToken), sessionKey) // 다른 채널의 채팅 이벤트 구독
func (c *Client) SubscribeChatEvent(ctx context.Context, sessionKey string) error {
	return c.sessionEvent(ctx, "/open/v1/sessions/events/subscribe/chat", ChatMessageRead, sessionKey)
}

// UnsubscribeChatEvent는 세션에서 채팅 이벤트 구독을 해제한다.
//
// 이벤트 구독 해제를 위해서는 세션 키(sessionKey)가 필요하다. 해당 값은 세션 생성 시 반환되는 URL에서 확인할 수 있다.
// (https://chzzk.gitbook.io/chzzk/chzzk-api/session#undefined)
// (https://chzzk.gitbook.io/chzzk/chzzk-api/session#undefined-1)
//
// [ChatMessageRead](채팅 메시지 조회) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
//
// 세션 구독 Event의 경우, Client 인증 방식일 경우 다른 사람의 Access Token을 사용하여 호출 해 타 채널의 이벤트를 구독 해제할 수 있다.
// 따라서, Client 인증 방식으로 호출 시 타 채널의 이벤트를 구독 해제하고 싶은 경우 Access Token을 Override하여 호출하는 것을 권장한다. [WithAccessToken] 참고.
//
//	chzzkgo.UnsubscribeChatEvent(ctx, sessionKey) // 현재 인증된 사용자의 채팅 이벤트 구독 해제
//
//	chzzkgo.UnsubscribeChatEvent(chzzkgo.WithAccessToken(ctx, otherChannelToken), sessionKey) // 다른 채널의 채팅 이벤트 구독 해제
func (c *Client) UnsubscribeChatEvent(ctx context.Context, sessionKey string) error {
	return c.sessionEvent(ctx, "/open/v1/sessions/events/unsubscribe/chat", ChatMessageRead, sessionKey)
}

// SubscribeDonationEvent는 세션에서 후원 이벤트를 구독한다.
//
// 이벤트 구독을 위해서는 세션 키(sessionKey)가 필요하다. 해당 값은 세션 생성 시 반환되는 URL에서 확인할 수 있다.
// (https://chzzk.gitbook.io/chzzk/chzzk-api/session#undefined)
// (https://chzzk.gitbook.io/chzzk/chzzk-api/session#undefined-1)
//
// [DonationRead](후원 조회) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
//
// 세션 구독 Event의 경우, Client 인증 방식일 경우 다른 사람의 Access Token을 사용하여 호출 해 타 채널의 이벤트를 구독할 수 있다.
// 따라서, Client 인증 방식으로 호출 시 타 채널의 이벤트를 구독하고 싶은 경우 Access Token을 Override하여 호출하는 것을 권장한다. [WithAccessToken] 참고.
//
//	chzzkgo.SubscribeDonationEvent(ctx, sessionKey) // 현재 인증된 사용자의 후원 이벤트 구독
//
//	chzzkgo.SubscribeDonationEvent(chzzkgo.WithAccessToken(ctx, otherChannelToken), sessionKey) // 다른 채널의 후원 이벤트 구독
func (c *Client) SubscribeDonationEvent(ctx context.Context, sessionKey string) error {
	return c.sessionEvent(ctx, "/open/v1/sessions/events/subscribe/donation", DonationRead, sessionKey)
}

// UnsubscribeDonationEvent는 세션에서 후원 이벤트 구독을 해제한다.
//
// 이벤트 구독 해제를 위해서는 세션 키(sessionKey)가 필요하다. 해당 값은 세션 생성 시 반환되는 URL에서 확인할 수 있다.
// (https://chzzk.gitbook.io/chzzk/chzzk-api/session#undefined)
// (https://chzzk.gitbook.io/chzzk/chzzk-api/session#undefined-1)
//
// [DonationRead](후원 조회) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
//
// 세션 구독 Event의 경우, Client 인증 방식일 경우 다른 사람의 Access Token을 사용하여 호출 해 타 채널의 이벤트를 구독 해제할 수 있다.
// 따라서, Client 인증 방식으로 호출 시 타 채널의 이벤트를 구독 해제하고 싶은 경우 Access Token을 Override하여 호출하는 것을 권장한다. [WithAccessToken] 참고.
//
//	chzzkgo.UnsubscribeDonationEvent(ctx, sessionKey) // 현재 인증된 사용자의 후원 이벤트 구독 해제
//
//	chzzkgo.UnsubscribeDonationEvent(chzzkgo.WithAccessToken(ctx, otherChannelToken), sessionKey) // 다른 채널의 후원 이벤트 구독 해제
func (c *Client) UnsubscribeDonationEvent(ctx context.Context, sessionKey string) error {
	return c.sessionEvent(ctx, "/open/v1/sessions/events/unsubscribe/donation", DonationRead, sessionKey)
}

// SubscribeSubscriptionEvent는 세션에서 구독 이벤트를 구독한다.
//
// 이벤트 구독을 위해서는 세션 키(sessionKey)가 필요하다. 해당 값은 세션 생성 시 반환되는 URL에서 확인할 수 있다.
// (https://chzzk.gitbook.io/chzzk/chzzk-api/session#undefined)
// (https://chzzk.gitbook.io/chzzk/chzzk-api/session#undefined-1)
//
// [SubscriptionRead](구독 조회) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
//
// 세션 구독 Event의 경우, Client 인증 방식일 경우 다른 사람의 Access Token을 사용하여 호출 해 타 채널의 이벤트를 구독할 수 있다.
// 따라서, Client 인증 방식으로 호출 시 타 채널의 이벤트를 구독하고 싶은 경우 Access Token을 Override하여 호출하는 것을 권장한다. [WithAccessToken] 참고.
//
//	chzzkgo.SubscribeSubscriptionEvent(ctx, sessionKey) // 현재 인증된 사용자의 구독 이벤트 구독
//
//	chzzkgo.SubscribeSubscriptionEvent(chzzkgo.WithAccessToken(ctx, otherChannelToken), sessionKey) // 다른 채널의 구독 이벤트 구독
func (c *Client) SubscribeSubscriptionEvent(ctx context.Context, sessionKey string) error {
	return c.sessionEvent(ctx, "/open/v1/sessions/events/subscribe/subscription", SubscriptionRead, sessionKey)
}

// UnsubscribeSubscriptionEvent는 세션에서 구독 이벤트 구독을 해제한다.
//
// 이벤트 구독 해제를 위해서는 세션 키(sessionKey)가 필요하다. 해당 값은 세션 생성 시 반환되는 URL에서 확인할 수 있다.
// (https://chzzk.gitbook.io/chzzk/chzzk-api/session#undefined)
// (https://chzzk.gitbook.io/chzzk/chzzk-api/session#undefined-1)
//
// [SubscriptionRead](구독 조회) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
//
// 세션 구독 Event의 경우, Client 인증 방식일 경우 다른 사람의 Access Token을 사용하여 호출 해 타 채널의 이벤트를 구독 해제할 수 있다.
// 따라서, Client 인증 방식으로 호출 시 타 채널의 이벤트를 구독 해제하고 싶은 경우 Access Token을 Override하여 호출하는 것을 권장한다. [WithAccessToken] 참고.
//
//	chzzkgo.UnsubscribeSubscriptionEvent(ctx, sessionKey) // 현재 인증된 사용자의 구독 이벤트 구독 해제
//
//	chzzkgo.UnsubscribeSubscriptionEvent(chzzkgo.WithAccessToken(ctx, otherChannelToken), sessionKey) // 다른 채널의 구독 이벤트 구독 해제
func (c *Client) UnsubscribeSubscriptionEvent(ctx context.Context, sessionKey string) error {
	return c.sessionEvent(ctx, "/open/v1/sessions/events/unsubscribe/subscription", SubscriptionRead, sessionKey)
}
