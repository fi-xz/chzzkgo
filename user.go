package chzzkgo

import "context"

// User는 현재 인증된 사용자의 정보를 나타낸다.
type User struct {
	// 사용자의 채널 ID
	ChannelID string `json:"channelId"`
	// 사용자의 채널 이름
	ChannelName string `json:"channelName"`
	// 사용자의 닉네임 (공식 문서에는 없으나 실응답에 포함됨)
	Nickname string `json:"nickname"`
}

// GetUser는 현재 인증된 사용자의 정보를 조회한다.
//
// [UserRead](유저 조회) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
func (c *Client) GetUser(ctx context.Context) (*User, error) {
	if err := c.requireScope(UserRead); err != nil {
		return nil, err
	}

	return get[User](c, ctx, "/open/v1/users/me", nil)
}
