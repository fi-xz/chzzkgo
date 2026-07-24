package chzzkgo

import (
	"encoding/json"
	"slices"
	"strings"
)

// Scopes는 [Scope]의 목록이다.
type Scopes []Scope

// Scope는 치지직 Open API의 권한 단위를 나타낸다.
// 치지직은 권한 이름으로 한국어 문자열을 사용한다.
type Scope string

const (
	// ChannelInfoRead는 채널 정보 조회 권한이다.
	ChannelInfoRead Scope = "채널 정보 조회"
	// ChannelManagerRead는 채널 관리자 조회 권한이다.
	ChannelManagerRead Scope = "채널 관리자 조회"
	// LiveStreamKeyRead는 방송 스트림키 조회 권한이다.
	LiveStreamKeyRead Scope = "방송 스트림키 조회"
	// LiveSettingRead는 방송 설정 조회 권한이다.
	LiveSettingRead Scope = "방송 설정 조회"
	// LiveSettingWrite는 방송 설정 변경 권한이다.
	LiveSettingWrite Scope = "방송 설정 변경"
	// UserRead는 유저 조회 권한이다.
	UserRead Scope = "유저 조회"
	// ChatMessageRead는 채팅 메시지 조회 권한이다.
	ChatMessageRead Scope = "채팅 메시지 조회"
	// ChatMessageWrite는 채팅 메시지 쓰기 권한이다.
	ChatMessageWrite Scope = "채팅 메시지 쓰기"
	// ChatNoticeWrite는 채팅 공지 쓰기 권한이다.
	ChatNoticeWrite Scope = "채팅 공지 쓰기"
	// ChatSettingsRead는 채팅 설정 조회 권한이다.
	ChatSettingsRead Scope = "채팅 설정 조회"
	// ChatSettingsWrite는 채팅 설정 변경 권한이다.
	ChatSettingsWrite Scope = "채팅 설정 변경"
	// RestrictionWrite는 활동제한 쓰기 권한이다.
	RestrictionWrite Scope = "활동제한 쓰기"
	// RestrictionRead는 활동제한 조회 권한이다.
	RestrictionRead Scope = "활동제한 조회"
	// DonationRead는 후원 조회 권한이다.
	DonationRead Scope = "후원 조회"
	// SubscriptionRead는 구독 조회 권한이다.
	SubscriptionRead Scope = "구독 조회"
)

var scopeTerminators = map[string]bool{
	"조회": true,
	"변경": true,
	"쓰기": true,
}

// ParseScopes는 공백으로 구분된 권한 문자열을 [Scopes]로 파싱한다.
//
// 치지직 API는 권한 목록을 "채널 정보 조회 유저 조회"처럼 하나의 문자열로 반환하므로,
// 종결자(조회/변경/쓰기)를 기준으로 개별 [Scope]를 분리한다.
func ParseScopes(raw string) Scopes {
	words := strings.Fields(raw)

	var out Scopes
	var current []string

	for _, w := range words {
		current = append(current, w)

		if scopeTerminators[w] {
			out = append(out, Scope(strings.Join(current, " ")))
			current = nil
		}
	}

	// 종결자 없이 끝난 잔여분 (미래에 규칙 벗어난 scope 대비)
	if len(current) > 0 {
		out = append(out, Scope(strings.Join(current, " ")))
	}

	return out
}

// UnmarshalJSON은 공백으로 구분된 권한 문자열을 [ParseScopes]로 파싱하여 담는다.
func (s *Scopes) UnmarshalJSON(b []byte) error {
	var raw string
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*s = ParseScopes(raw)
	return nil
}

// MarshalJSON은 권한 목록을 API 응답과 동일한 공백 구분 문자열로 직렬화한다.
// [Tokens]를 JSON으로 저장했다가 복원해도 원형이 유지된다.
func (s Scopes) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// String은 권한 목록을 공백으로 구분된 하나의 문자열로 반환한다.
func (s Scopes) String() string {
	parts := make([]string, len(s))
	for i, sc := range s {
		parts[i] = string(sc)
	}
	return strings.Join(parts, " ")
}

// Has는 권한 목록에 scope가 포함되어 있는지 반환한다.
func (s Scopes) Has(scope Scope) bool {
	return slices.Contains(s, scope)
}
