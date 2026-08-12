package chzzkgo

import (
	"context"
	"fmt"
	"slices"
	"unicode/utf8"
)

// MessageResult는 채팅 메시지 전송 결과를 나타낸다.
type MessageResult struct {
	// 전송된 채팅의 메시지 ID
	MessageID string `json:"messageId"`
}

// ChatSettings는 조회된 채팅 설정 정보를 나타낸다.
type ChatSettings struct {
	// 채팅 허용 조건, [ChatAvailableCondition] 참고
	ChatAvailableCondition ChatAvailableCondition `json:"chatAvailableCondition"`
	// 채팅 허용 그룹, [ChatAvailableGroup] 참고
	ChatAvailableGroup ChatAvailableGroup `json:"chatAvailableGroup"`
	// 최소 채널 팔로우 시간(분), 0이면 제한 없음
	MinFollowerMinute int `json:"minFollowerMinute"`
	// 팔로워 전용 모드에서 구독자 채팅 허용 여부
	AllowSubscriberInFollowerMode bool `json:"allowSubscriberInFollowerMode"`
	// 채팅 슬로우 모드 시간(초), 0이면 제한 없음
	ChatSlowModeSec int `json:"chatSlowModeSec"`
	// 채팅 이모지 전용 모드 여부
	ChatEmojiMode bool `json:"chatEmojiMode"`
}

// ChatSettingsPatch는 [Client.SetChatSettings]로 변경할 채팅 설정 정보를 나타낸다.
//
// nil인 필드는 요청에서 제외되어 변경되지 않는다.
// 값 지정에는 내장 함수 new를 사용할 수 있다. (예: new(true))
type ChatSettingsPatch struct {
	// 채팅 허용 조건, [ChatAvailableCondition] 참고
	ChatAvailableCondition *ChatAvailableCondition `json:"chatAvailableCondition,omitempty"`
	// 채팅 허용 그룹, [ChatAvailableGroup] 참고
	ChatAvailableGroup *ChatAvailableGroup `json:"chatAvailableGroup,omitempty"`
	// 최소 채널 팔로우 시간(분), 0이면 제한 없음.
	// 허용 값: 0, 5, 10, 30, 60, 1440, 10080, 43200, 86400, 129600, 172800, 216000, 259200
	MinFollowerMinute *int `json:"minFollowerMinute,omitempty"`
	// 팔로워 전용 모드에서 구독자 채팅 허용 여부
	AllowSubscriberInFollowerMode *bool `json:"allowSubscriberInFollowerMode,omitempty"`
	// 채팅 슬로우 모드 시간(초), 0이면 제한 없음.
	// 허용 값: 0, 3, 5, 10, 30, 60, 120, 300
	ChatSlowModeSec *int `json:"chatSlowModeSec,omitempty"`
	// 채팅 이모지 전용 모드 여부
	ChatEmojiMode *bool `json:"chatEmojiMode,omitempty"`
}

var chatMinFollowerMinuteValues = []int{0, 5, 10, 30, 60, 1440, 10080, 43200, 86400, 129600, 172800, 216000, 259200}
var chatSlowModeSecValues = []int{0, 3, 5, 10, 30, 60, 120, 300}

// ChatNoticeRequest는 채팅 공지 설정 요청 정보를 나타낸다.
type ChatNoticeRequest struct {
	// 채팅 공지 메시지 내용
	Message string `json:"message"`
	// 채팅 공지 메시지 ID
	MessageID string `json:"messageId"`
}

// ChatAvailableCondition은 채팅 허용 조건을 나타낸다.
type ChatAvailableCondition string

// ChatAvailableGroup은 채팅 허용 그룹을 나타낸다.
type ChatAvailableGroup string

const (
	// ChatAvailableConditionNone은 채팅 허용 조건 없음이다.
	ChatAvailableConditionNone ChatAvailableCondition = "NONE"
	// ChatAvailableConditionRealName은 실명인증 사용자만 채팅을 허용한다.
	ChatAvailableConditionRealName ChatAvailableCondition = "REAL_NAME"

	// ChatAvailableGroupAll은 전체 사용자에게 채팅을 허용한다.
	ChatAvailableGroupAll ChatAvailableGroup = "ALL"
	// ChatAvailableGroupFollower는 팔로워에게만 채팅을 허용한다.
	ChatAvailableGroupFollower ChatAvailableGroup = "FOLLOWER"
	// ChatAvailableGroupManager는 관리자에게만 채팅을 허용한다.
	ChatAvailableGroupManager ChatAvailableGroup = "MANAGER"
	// ChatAvailableGroupSubscriber는 구독자에게만 채팅을 허용한다.
	ChatAvailableGroupSubscriber ChatAvailableGroup = "SUBSCRIBER"
)

var chatAvailableConditionValues = []ChatAvailableCondition{
	ChatAvailableConditionNone,
	ChatAvailableConditionRealName,
}

var chatAvailableGroupValues = []ChatAvailableGroup{
	ChatAvailableGroupAll,
	ChatAvailableGroupFollower,
	ChatAvailableGroupManager,
	ChatAvailableGroupSubscriber,
}

// ChatBlindRequest는 채팅 블라인드 요청 정보를 나타낸다.
type ChatBlindRequest struct {
	// 채팅 채널 ID
	ChatChannelID string `json:"chatChannelId"`
	// 메시지 전송 시간 (long, milliseconds)
	MessageTime int64 `json:"messageTime"`
	// 발신자 채널 ID
	SenderChannelID string `json:"senderChannelId"`
}

// SendChatMessage는 채팅 메시지를 전송한다.
//
// message에는 전송할 채팅 메시지를 담은 문자열을 전달한다. 메시지 길이는 최대 100자까지 허용된다.
//
// [ChatMessageWrite](채팅 메시지 쓰기) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
func (c *Client) SendChatMessage(ctx context.Context, message string) (*MessageResult, error) {
	if err := c.requireScope(ChatMessageWrite); err != nil {
		return nil, err
	}

	if utf8.RuneCountInString(message) > 100 {
		return nil, fmt.Errorf("chzzkgo: message length exceeds 100 characters")
	}

	return post[MessageResult](c, ctx, "/open/v1/chats/send", map[string]string{"message": message})
}

// SetChatNotice는 채팅 공지를 설정한다.
//
// req에는 채팅 공지 메시지 내용과 메시지 ID를 담은 [ChatNoticeRequest] 구조체를 전달한다. 구조체 내의 필드 중 최소한 하나는 설정되어야 한다.
//
// Message를 지정할 경우 채팅 공지 메시지 내용을 새로 전송하여 설정하고, MessageID를 지정할 경우 이미 전송된 채팅 메시지를 공지로 설정한다. 두 필드 모두 설정하지 않으면 에러를 반환한다.
//
// [ChatNoticeWrite](채팅 공지 쓰기) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
func (c *Client) SetChatNotice(ctx context.Context, req ChatNoticeRequest) error {
	if err := c.requireScope(ChatNoticeWrite); err != nil {
		return err
	}

	if req.Message == "" && req.MessageID == "" {
		return fmt.Errorf("chzzkgo: either Message or MessageID must be provided")
	}

	// Result: {"code": 200, "message": "SUCCESS", "content": null}
	_, err := post[empty](c, ctx, "/open/v1/chats/notice", req)

	if err != nil {
		return err
	}

	return nil
}

// GetChatSettings는 채팅 설정 정보를 조회한다.
//
// [ChatSettingsRead](채팅 설정 조회) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
func (c *Client) GetChatSettings(ctx context.Context) (*ChatSettings, error) {
	if err := c.requireScope(ChatSettingsRead); err != nil {
		return nil, err
	}

	return get[ChatSettings](c, ctx, "/open/v1/chats/settings", nil)
}

// SetChatSettings는 채팅 설정 정보를 변경한다.
//
// newChatSettings에는 변경할 채팅 설정 정보를 담은 [ChatSettingsPatch] 구조체를 전달한다.
// nil 필드는 전송되지 않으며, 서버는 전송되지 않은 필드의 기존 값을 유지한다.
//
// [ChatSettingsWrite](채팅 설정 변경) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
func (c *Client) SetChatSettings(ctx context.Context, newChatSettings ChatSettingsPatch) (*ChatSettings, error) {
	if err := c.requireScope(ChatSettingsWrite); err != nil {
		return nil, err
	}

	err := validateChatSettings(newChatSettings)

	if err != nil {
		return nil, err
	}

	modifiedChatSettings, err := put[ChatSettings](c, ctx, "/open/v1/chats/settings", newChatSettings)

	if err != nil {
		return nil, err
	}

	return modifiedChatSettings, nil
}

// BlindChatMessage는 특정 채팅 메시지를 블라인드 처리한다.
//
// req에는 블라인드 처리할 채팅 메시지의 정보를 담은 [ChatBlindRequest] 구조체를 전달한다.
//
// 채팅 블라인드를 위해서는 채팅 채널 ID(ChatChannelID), 메시지 전송 시간(MessageTime), 발신자 채널 ID(SenderChannelID)가 필요하다.
// 이는 Session의 채팅 구독 이벤트 메시지 값에서 확인할 수 있다. (https://chzzk.gitbook.io/chzzk/chzzk-api/session#message-event-subscribe-chat)
//
// [ChatMessageWrite](채팅 메시지 쓰기) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
func (c *Client) BlindChatMessage(ctx context.Context, req ChatBlindRequest) error {
	if err := c.requireScope(ChatMessageWrite); err != nil {
		return err
	}

	if req.ChatChannelID == "" || req.MessageTime == 0 || req.SenderChannelID == "" {
		return fmt.Errorf("chzzkgo: ChatChannelID, MessageTime, and SenderChannelID must be provided")
	}

	// Result: {"code": 200, "message": "SUCCESS", "content": null}
	_, err := post[empty](c, ctx, "/open/v1/chats/blind-message", req)

	if err != nil {
		return err
	}

	return nil
}

func validateChatSettings(settings ChatSettingsPatch) error {
	if settings.ChatAvailableCondition != nil && !slices.Contains(chatAvailableConditionValues, *settings.ChatAvailableCondition) {
		return fmt.Errorf("chzzkgo: invalid ChatAvailableCondition value: %s", *settings.ChatAvailableCondition)
	}

	if settings.ChatAvailableGroup != nil && !slices.Contains(chatAvailableGroupValues, *settings.ChatAvailableGroup) {
		return fmt.Errorf("chzzkgo: invalid ChatAvailableGroup value: %s", *settings.ChatAvailableGroup)
	}

	if settings.MinFollowerMinute != nil && !slices.Contains(chatMinFollowerMinuteValues, *settings.MinFollowerMinute) {
		return fmt.Errorf("chzzkgo: invalid MinFollowerMinute value: %d", *settings.MinFollowerMinute)
	}

	if settings.ChatSlowModeSec != nil && !slices.Contains(chatSlowModeSecValues, *settings.ChatSlowModeSec) {
		return fmt.Errorf("chzzkgo: invalid ChatSlowModeSec value: %d", *settings.ChatSlowModeSec)
	}

	return nil
}
