package chzzkgo

import (
	"encoding/json"
	"sync"
	"time"
)

// eventTimeParseLayout은 이벤트 페이로드의 시각 표기 형식이다.
// 오프셋 표기가 없고 소수점 이하가 9자리라 RFC 3339가 아니다.
// 자릿수가 다른 경우에도 받아들일 수 있도록 소수부를 선택적으로 둔다.
const eventTimeParseLayout = "2006-01-02T15:04:05.999999999"

// eventTimeFormatLayout은 직렬화에 쓰는 형식이다.
// 서버가 보내는 것과 같이 소수 이하를 항상 9자리로 채운다.
const eventTimeFormatLayout = "2006-01-02T15:04:05.000000000"

// kst는 이벤트 시각의 기준 시간대(Asia/Seoul)를 반환한다.
//
// 시스템에 tzdata가 없는 환경(주로 Windows)을 대비해 UTC+9 고정 오프셋으로 대체한다.
// 대한민국은 1988년 이후 서머타임을 시행하지 않으므로 현재 시각에 대해서는 결과가 같다.
var kst = sync.OnceValue(func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Seoul"); err == nil {
		return loc
	}

	return time.FixedZone("KST", 9*60*60)
})

// EventTime은 세션 이벤트의 발신 시각을 나타낸다.
//
// 서버는 이 값을 "2026-07-26T03:14:18.629843820"처럼 오프셋 없이 보내지만
// 실제 기준은 KST이다. UTC로 해석하면 9시간이 어긋나며 파싱 단계에서
// 오류도 나지 않으므로, 이 타입이 언마샬 시 [Asia/Seoul] 위치를 지정한다.
//
// [Asia/Seoul]: https://www.iana.org/time-zones
type EventTime struct {
	time.Time
}

// UnmarshalJSON은 시각 문자열을 해석한다.
//
// 오프셋이 붙어 있으면 그대로 따르고, 없을 때만 KST를 씌운다.
// null이나 빈 문자열은 제로 값으로 둔다.
func (t *EventTime) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}

	var raw string

	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	if raw == "" {
		return nil
	}

	// 오프셋이 붙어 있으면 서버가 시간대를 명시한 것이므로 그대로 존중한다.
	if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		t.Time = parsed
		return nil
	}

	parsed, err := time.ParseInLocation(eventTimeParseLayout, raw, kst())

	if err != nil {
		return err
	}

	t.Time = parsed
	return nil
}

// MarshalJSON은 서버가 보낸 것과 같은 형식의 KST 시각 문자열로 직렬화한다.
// 제로 값은 null로 표기한다.
func (t EventTime) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}

	return json.Marshal(t.In(kst()).Format(eventTimeFormatLayout))
}

// unwrapEventPayload는 이중 인코딩된 이벤트 본문을 한 겹 벗긴다.
//
// CHZZK 세션 서버는 이벤트 인자를 JSON 객체가 아니라 JSON 문자열로 한 번 더
// 감싸서 보낸다. 표준 socket.io 서버처럼 객체를 그대로 보내는 경우에도 동작하도록,
// 문자열로 해석되면 벗기고 아니면 원본을 그대로 돌려준다.
func unwrapEventPayload(raw json.RawMessage) json.RawMessage {
	var unwrapped string

	if err := json.Unmarshal(raw, &unwrapped); err != nil {
		return raw
	}

	return json.RawMessage(unwrapped)
}

// UserRoleCode는 채팅 발신자의 역할을 나타낸다.
//
// 채널 관리자 조회에 쓰이는 [UserRole]과는 값 체계가 다르다.
type UserRoleCode string

const (
	// UserRoleCodeStreamer는 방송인 본인이다.
	UserRoleCodeStreamer UserRoleCode = "streamer"
	// UserRoleCodeCommonUser는 일반 시청자이다.
	UserRoleCodeCommonUser UserRoleCode = "common_user"
	// UserRoleCodeStreamingChannelManager는 채널 관리자이다.
	UserRoleCodeStreamingChannelManager UserRoleCode = "streaming_channel_manager"
	// UserRoleCodeStreamingChatManager는 채팅 관리자이다.
	UserRoleCodeStreamingChatManager UserRoleCode = "streaming_chat_manager"
)

// ChatBadge는 채팅 발신자에게 표시되는 뱃지이다.
type ChatBadge struct {
	// 뱃지 이미지 URL
	ImageURL string `json:"imageUrl"`
}

// ChatProfile은 채팅 발신자의 프로필 정보이다.
type ChatProfile struct {
	// 발신자 닉네임
	Nickname string `json:"nickname"`
	// 발신자 채널의 인증 마크 여부
	VerifiedMark bool `json:"verifiedMark"`
	// 발신자에게 표시되는 뱃지 목록
	Badges []ChatBadge `json:"badges"`
	// 발신자 역할. [UserRoleCode] 상수 참고.
	//
	// 공식 문서는 이 필드를 최상위에 두고 있으나 실제 응답에서는 프로필 안에 있다.
	UserRoleCode UserRoleCode `json:"userRoleCode"`
}

// ChatEvent는 세션에서 수신한 채팅 이벤트이다.
type ChatEvent struct {
	// 이벤트가 발생한 채널 ID
	ChannelID string `json:"channelId"`
	// 채팅 채널 ID. 채팅 블라인드와 임시 제한에 사용한다.
	ChatChannelID string `json:"chatChannelId"`
	// 발신자 채널 ID
	SenderChannelID string `json:"senderChannelId"`
	// 발신자 프로필
	Profile ChatProfile `json:"profile"`
	// 채팅 메시지 내용
	Content string `json:"content"`
	// 사용된 이모티콘. 이모티콘 ID를 이미지 URL에 대응시킨다.
	Emojis map[string]string `json:"emojis"`
	// 메시지 전송 시각. epoch 밀리초(UTC)이며 [ChatEvent.MessageAt]으로 변환할 수 있다.
	// 채팅 블라인드에 이 값이 그대로 필요하므로 원본을 유지한다.
	MessageTime int64 `json:"messageTime"`
	// 서버가 이벤트를 보낸 시각. 공식 문서에는 없다.
	EventSentAt EventTime `json:"eventSentAt"`
}

// MessageAt은 [ChatEvent.MessageTime]을 KST 기준 시각으로 변환해 반환한다.
func (e ChatEvent) MessageAt() time.Time {
	return time.UnixMilli(e.MessageTime).In(kst())
}

// BlindRequest는 이 채팅을 블라인드 처리하기 위한 요청을 만든다.
// [Client.BlindChatMessage]에 그대로 전달할 수 있다.
func (e ChatEvent) BlindRequest() ChatBlindRequest {
	return ChatBlindRequest{
		ChatChannelID:   e.ChatChannelID,
		MessageTime:     e.MessageTime,
		SenderChannelID: e.SenderChannelID,
	}
}

// DonationType은 후원의 종류를 나타낸다.
type DonationType string

const (
	// DonationTypeChat은 채팅 후원이다.
	DonationTypeChat DonationType = "CHAT"
	// DonationTypeVideo는 영상 후원이다.
	DonationTypeVideo DonationType = "VIDEO"
)

// DonationEvent는 세션에서 수신한 후원 이벤트이다.
type DonationEvent struct {
	// 후원 종류. [DonationType] 상수 참고.
	DonationType DonationType `json:"donationType"`
	// 이벤트가 발생한 채널 ID
	ChannelID string `json:"channelId"`
	// 후원자 채널 ID
	DonatorChannelID string `json:"donatorChannelId"`
	// 후원자 닉네임
	DonatorNickname string `json:"donatorNickname"`
	// 후원 금액. 공식 문서는 문자열로 표기하고 있으나 실제 응답은 숫자이다.
	PayAmount int `json:"payAmount"`
	// 후원자가 입력한 메시지
	DonationText string `json:"donationText"`
	// 사용된 이모티콘. 이모티콘 ID를 이미지 URL에 대응시킨다.
	Emojis map[string]string `json:"emojis"`
	// 서버가 이벤트를 보낸 시각. 공식 문서에는 없다.
	EventSentAt EventTime `json:"eventSentAt"`
}

// SubscriptionEvent는 세션에서 수신한 구독 이벤트이다.
//
// 구독 기능이 치지직 프로 회원에게만 열려 있어 실제 페이로드를 관측하지 못했다.
// 이 구조체는 공식 문서만을 근거로 정의한 미검증 상태이며, 필드 이름과 타입이
// 실제와 다를 수 있다. 실제로 후원의 payAmount는 문서와 달리 문자열이 아닌
// 숫자였으므로 TierNo와 Month의 타입도 확인이 필요하다.
// 값이 비어 있다면 [SessionSocket.OnAny]로 원본을 확인할 것.
type SubscriptionEvent struct {
	// 이벤트가 발생한 채널 ID
	ChannelID string `json:"channelId"`
	// 구독자 채널 ID
	SubscriberChannelID string `json:"subscriberChannelId"`
	// 구독자 닉네임
	SubscriberNickname string `json:"subscriberNickname"`
	// 구독 티어 (1 또는 2)
	TierNo int `json:"tierNo"`
	// 구독 티어 이름
	TierName string `json:"tierName"`
	// 구독 개월 수
	Month int `json:"month"`
	// 서버가 이벤트를 보낸 시각.
	// 다른 이벤트에 있는 것으로 미루어 넣어 두었을 뿐 확인된 바 없다.
	EventSentAt EventTime `json:"eventSentAt"`
}

// SystemEventType은 시스템 이벤트의 종류를 나타낸다.
type SystemEventType string

const (
	// SystemEventTypeConnected는 세션 연결이 완료되었음을 알린다. 세션 키가 함께 온다.
	SystemEventTypeConnected SystemEventType = "connected"
	// SystemEventTypeSubscribed는 이벤트 구독이 시작되었음을 알린다.
	SystemEventTypeSubscribed SystemEventType = "subscribed"
	// SystemEventTypeUnsubscribed는 이벤트 구독이 해제되었음을 알린다.
	SystemEventTypeUnsubscribed SystemEventType = "unsubscribed"
	// SystemEventTypeRevoked는 서버가 구독을 취소했음을 알린다.
	// 이 이후로는 해당 이벤트가 오지 않으므로 다시 구독하거나 세션을 정리해야 한다.
	SystemEventTypeRevoked SystemEventType = "revoked"
)

// SystemEventData는 시스템 이벤트의 본문이다.
// 채워지는 필드는 [SystemEvent.Type]에 따라 다르다.
type SystemEventData struct {
	// 세션 키. connected에서만 채워진다.
	SessionKey string `json:"sessionKey"`
	// 대상 이벤트 종류. subscribed, unsubscribed, revoked에서 채워진다.
	EventType Event `json:"eventType"`
	// 대상 채널 ID. subscribed, unsubscribed, revoked에서 채워진다.
	ChannelID string `json:"channelId"`
}

// SystemEvent는 세션 상태 변화를 알리는 시스템 이벤트이다.
type SystemEvent struct {
	// 이벤트 종류. [SystemEventType] 상수 참고.
	// 문서에 없는 값이 올 수도 있으므로 알 수 없는 값은 무시하는 편이 안전하다.
	Type SystemEventType `json:"type"`
	// 이벤트 본문
	Data SystemEventData `json:"data"`
}
