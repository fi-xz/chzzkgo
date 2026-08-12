package chzzkgo

import (
	"context"
)

// Restriction은 치지직에서 사용되는 활동 제한 정보를 나타낸다.
type Restriction struct {
	// 활동 제한된 채널 ID
	RestrictedChannelID string `json:"restrictedChannelId"`
	// 활동 제한된 채널 이름
	RestrictedChannelName string `json:"restrictedChannelName"`
	// 생성 날짜
	CreatedDate string `json:"createdDate"`
	// 해제 날짜, 영구 제한의 경우 null
	ReleaseDate string `json:"releaseDate,omitempty"`
}

// RestrictionPages는 활동 제한 검색 결과를 담는 구조체이다.
type RestrictionPages struct {
	// 활동 제한 정보 목록
	Data []Restriction `json:"data"`
	// 다음 페이지 조회를 위한 구조체, 마지막 페이지인 경우 null
	Page struct {
		// 다음 페이지 조회를 위한 토큰
		Next string `json:"next"`
	} `json:"page"`
}

// AddRestriction은 targetChannelId에 대해 활동 제한을 추가한다.
//
// [RestrictionWrite](활동제한 쓰기) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
func (c *Client) AddRestriction(ctx context.Context, targetChannelID string) error {
	if err := c.requireScope(RestrictionWrite); err != nil {
		return err
	}

	// Result: {"code": 200, "message": "SUCCESS", "content": null}
	_, err := post[empty](c, ctx, "/open/v1/restrict-channels", map[string]string{
		"targetChannelId": targetChannelID,
	})

	if err != nil {
		return err
	}

	return nil
}

// RemoveRestriction은 targetChannelId에 대해 활동 제한을 해제한다.
//
// [RestrictionWrite](활동제한 쓰기) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
func (c *Client) RemoveRestriction(ctx context.Context, targetChannelID string) error {
	if err := c.requireScope(RestrictionWrite); err != nil {
		return err
	}

	// Result: {"code": 200, "message": "SUCCESS", "content": null}
	_, err := del[empty](c, ctx, "/open/v1/restrict-channels", map[string]string{
		"targetChannelId": targetChannelID,
	})

	if err != nil {
		return err
	}

	return nil
}

// GetRestrictions는 활동 제한된 채널 목록을 조회한다.
//
// [RestrictionRead](활동제한 조회) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
// 선택적 파라미터로 size, next를 지정할 수 있다. [WithSize], [WithNext]를 참고.
// size의 경우, 지정되지 않았다면 서버에서 기본값 30을 사용하며, 최소 1에서 최대 30까지 지정 가능하다.
func (c *Client) GetRestrictions(ctx context.Context, opts ...QueryOption) (*RestrictionPages, error) {
	if err := c.requireScope(RestrictionRead); err != nil {
		return nil, err
	}

	q := buildQuery(opts...)

	if err := validateSize(q, 1, 30); err != nil {
		return nil, err
	}

	restrictions, err := get[RestrictionPages](c, ctx, "/open/v1/restrict-channels", q)
	if err != nil {
		return nil, err
	}

	return restrictions, nil
}

// AddTemporaryRestriction은 targetChannelId에 대해 임시 제한을 추가한다.
//
// 임시 제한을 위해서는 chatChannelID(채팅 채널 ID)가 필요하다. 이는 Session의 채팅 구독 이벤트 메시지 값에서 확인할 수 있다. (https://chzzk.gitbook.io/chzzk/chzzk-api/session#message-event-subscribe-chat)
//
// [RestrictionWrite](활동제한 쓰기) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
func (c *Client) AddTemporaryRestriction(ctx context.Context, targetChannelID, chatChannelID string) error {
	if err := c.requireScope(RestrictionWrite); err != nil {
		return err
	}

	// Result: {"code": 200, "message": "SUCCESS", "content": null}
	_, err := post[empty](c, ctx, "/open/v1/temporary-restrict-channels", map[string]string{
		"targetChannelId": targetChannelID,
		"chatChannelId":   chatChannelID,
	})

	if err != nil {
		return err
	}

	return nil
}

// RemoveTemporaryRestriction은 targetChannelId에 대해 임시 제한을 해제한다.
//
// 임시 제한 해제를 위해서는 chatChannelID(채팅 채널 ID)가 필요하다. 이는 Session의 채팅 구독 이벤트 메시지 값에서 확인할 수 있다. (https://chzzk.gitbook.io/chzzk/chzzk-api/session#message-event-subscribe-chat)
//
// [RestrictionWrite](활동제한 쓰기) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
func (c *Client) RemoveTemporaryRestriction(ctx context.Context, targetChannelID, chatChannelID string) error {
	if err := c.requireScope(RestrictionWrite); err != nil {
		return err
	}

	// Result: {"code": 200, "message": "SUCCESS", "content": null}
	_, err := del[empty](c, ctx, "/open/v1/temporary-restrict-channels", map[string]string{
		"targetChannelId": targetChannelID,
		"chatChannelId":   chatChannelID,
	})

	if err != nil {
		return err
	}

	return nil
}
