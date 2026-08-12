package chzzkgo

import (
	"context"
	"net/url"
	"strings"
)

// Channel은 치지직에서 사용되는 채널 정보를 나타낸다.
type Channel struct {
	// 치지직 채널 ID (예: c42cd75ec4855a9edf204a407c3c1dd2)
	ChannelID string `json:"channelId"`
	// 치지직 채널 이름 (예: 치지직)
	ChannelName string `json:"channelName"`
	// 치지직 채널 이미지 URL
	ChannelImageURL string `json:"channelImageUrl"`
	// 치지직 채널 팔로워 수
	FollowerCount int `json:"followerCount"`
	// 채널 인증 마크 여부
	VerifiedMark bool `json:"verifiedMark"`
}

// ChannelPages는 채널 검색 결과를 담는 구조체이다.
type ChannelPages struct {
	// 채널 정보 목록
	Data []Channel `json:"data"`
}

// StreamingRole은 치지직에서 사용되는 권한 체계를 나타낸다.
type StreamingRole struct {
	// 매니저/관리자 채널 ID
	ManagerChannelID string `json:"managerChannelId"`
	// 매니저/관리자 채널 이름
	ManagerChannelName string `json:"managerChannelName"`
	// 사용자 역할. [UserRole] 상수 참고.
	UserRole UserRole `json:"userRole"`
	// 권한이 부여된 날짜
	CreatedDate string `json:"createdDate"`
}

// StreamingRolePages는 권한 검색 결과를 담는 구조체이다.
type StreamingRolePages struct {
	// 권한 정보 목록
	Data []StreamingRole `json:"data"`
}

// UserRole은 치지직에서 사용되는 사용자 역할을 나타낸다.
type UserRole string

const (
	// StreamingChannelOwner는 치지직 채널 소유자이다. (미사용으로 추정 - [Streamer] 참고)
	StreamingChannelOwner UserRole = "STREAMING_CHANNEL_OWNER"
	// StreamingChannelManager는 치지직 채널 관리자이다.
	StreamingChannelManager UserRole = "STREAMING_CHANNEL_MANAGER"
	// StreamingChatManager는 치지직 채널 채팅 관리자이다.
	StreamingChatManager UserRole = "STREAMING_CHAT_MANAGER"
	// StreamingSettlementManager는 치지직 채널 정산 관리자이다.
	StreamingSettlementManager UserRole = "STREAMING_SETTLEMENT_MANAGER"
	// Streamer는 스트리머 본인이다. (공식 문서에는 없으나 실응답에서 확인된 역할)
	Streamer UserRole = "STREAMER"
)

// ChannelFollower는 치지직에서 사용되는 채널 팔로워 정보를 나타낸다.
type ChannelFollower struct {
	// 팔로워 채널 ID
	ChannelID string `json:"channelId"`
	// 팔로워 채널 이름
	ChannelName string `json:"channelName"`
	// 팔로우 일자
	CreatedDate string `json:"createdDate"`
}

// ChannelFollowerPages는 채널 팔로워 검색 결과를 담는 구조체이다.
type ChannelFollowerPages struct {
	// 채널 팔로워 정보 목록
	Data []ChannelFollower `json:"data"`
	// 현재 페이지 번호
	Page int `json:"page"`
	// 총 팔로워 수
	TotalCount int `json:"totalCount"`
	// 총 페이지 수
	TotalPages int `json:"totalPages"`
}

// ChannelSubscriber는 치지직에서 사용되는 채널 구독자 정보를 나타낸다.
type ChannelSubscriber struct {
	// 구독자 채널 ID
	ChannelID string `json:"channelId"`
	// 구독자 채널 이름
	ChannelName string `json:"channelName"`
	// 구독 기간 (개월)
	Month int `json:"month"`
	// 구독 티어 (1, 2)
	TierNo int `json:"tierNo"`
	// 구독 일자
	CreatedDate string `json:"createdDate"`
}

// ChannelSubscriberPages는 채널 구독자 검색 결과를 담는 구조체이다.
type ChannelSubscriberPages struct {
	// 채널 구독자 정보 목록
	Data []ChannelSubscriber `json:"data"`
	// 현재 페이지 번호
	Page int `json:"page"`
	// 총 구독자 수
	TotalCount int `json:"totalCount"`
	// 총 페이지 수
	TotalPages int `json:"totalPages"`
}

// GetChannels는 입력된 channelIDs에 대해 채널 정보를 조회한다.
//
// 조회를 원하는 채널 ID들이 포함된 channelIDs가 필요하다. channelIDs는 최대 20개까지 요청 가능하다.
//
// Client 인증을 사용하므로 OAuth 토큰 없이 호출 가능하다.
func (c *Client) GetChannels(ctx context.Context, channelIDs []string) (*ChannelPages, error) {
	q := url.Values{}
	q.Set("channelIds", strings.Join(channelIDs, ","))

	channelPages, err := getWithClient[ChannelPages](c, ctx, "/open/v1/channels", q)

	if err != nil {
		return nil, err
	}

	return channelPages, nil
}

// GetChannelManagers는 현재 인증된 사용자의 채널 관리자 정보를 조회한다.
//
// [ChannelManagerRead](채널 관리자 조회) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
func (c *Client) GetChannelManagers(ctx context.Context) (*StreamingRolePages, error) {
	if err := c.requireScope(ChannelManagerRead); err != nil {
		return nil, err
	}

	streamingRolePages, err := get[StreamingRolePages](c, ctx, "/open/v1/channels/streaming-roles", nil)

	if err != nil {
		return nil, err
	}

	return streamingRolePages, nil
}

// GetChannelFollowers는 현재 인증된 사용자의 채널 팔로워 정보를 조회한다.
//
// [ChannelInfoRead](채널 정보 조회) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
//
// 선택적 파라미터로 size, page를 지정할 수 있다. [WithSize], [WithPage]를 참고.
// size가 지정되지 않았다면 서버에서 기본값 30을 사용하며, 최소 1에서 최대 50까지 지정 가능하다.
// page는 0부터 시작하며, 지정되지 않았다면 서버에서 기본값 0을 사용한다.
func (c *Client) GetChannelFollowers(ctx context.Context, opts ...QueryOption) (*ChannelFollowerPages, error) {
	// 채널 팔로워 조회는 ChannelInfoRead Scope 권한이 필요.
	// 네이버 문서상에서는 "채널 팔로워 조회"가 필요하다고 명시되어있으나
	// 고객센터 문의 결과 "채널 정보 조회" 권한으로 조회가 가능하다고 답변 받은 바 있음.
	if err := c.requireScope(ChannelInfoRead); err != nil {
		return nil, err
	}

	q := buildQuery(opts...)

	if err := validateSize(q, 1, 50); err != nil {
		return nil, err
	}

	channelFollowerPages, err := get[ChannelFollowerPages](c, ctx, "/open/v1/channels/followers", q)

	if err != nil {
		return nil, err
	}

	return channelFollowerPages, nil
}

// GetChannelSubscribers는 현재 인증된 사용자의 채널 구독자 정보를 조회한다.
//
// [ChannelInfoRead](채널 정보 조회) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
//
// 선택적 파라미터로 size를 지정할 수 있다. [WithSize]를 참고.
// size가 지정되지 않았다면 서버에서 기본값 30을 사용하며, 최소 1에서 최대 50까지 지정 가능하다.
func (c *Client) GetChannelSubscribers(ctx context.Context, opts ...QueryOption) (*ChannelSubscriberPages, error) {
	// 채널 구독자 조회는 ChannelInfoRead Scope 권한이 필요.
	// 네이버 문서상에서는 "채널 구독자"가 필요하다고 명시되어있으나
	// 고객센터 문의 결과 "채널 정보 조회" 권한으로 조회가 가능하다고 답변 받은 바 있음.
	if err := c.requireScope(ChannelInfoRead); err != nil {
		return nil, err
	}

	q := buildQuery(opts...)

	if err := validateSize(q, 1, 50); err != nil {
		return nil, err
	}

	channelSubscriberPages, err := get[ChannelSubscriberPages](c, ctx, "/open/v1/channels/subscribers", q)

	if err != nil {
		return nil, err
	}

	return channelSubscriberPages, nil
}
