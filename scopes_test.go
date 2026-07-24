package chzzkgo_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/fi-xz/chzzkgo"
)

func TestParseScopes(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want chzzkgo.Scopes
	}{
		{"단일 scope", "유저 조회", chzzkgo.Scopes{chzzkgo.UserRead}},
		{
			"복수 scope",
			"채널 정보 조회 유저 조회 방송 설정 변경 채팅 메시지 쓰기",
			chzzkgo.Scopes{chzzkgo.ChannelInfoRead, chzzkgo.UserRead, chzzkgo.LiveSettingWrite, chzzkgo.ChatMessageWrite},
		},
		{"빈 문자열", "", nil},
		{"종결자 없는 잔여분", "새로운 권한", chzzkgo.Scopes{chzzkgo.Scope("새로운 권한")}},
		{"종결자 뒤 잔여분", "유저 조회 새로운", chzzkgo.Scopes{chzzkgo.UserRead, chzzkgo.Scope("새로운")}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := chzzkgo.ParseScopes(tc.raw)

			if !slices.Equal(got, tc.want) {
				t.Errorf("ParseScopes(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

// TestParseScopesAllConstants는 정의된 모든 Scope 상수가
// String() 직렬화 후 ParseScopes로 원형 복원되는지 검증한다.
func TestParseScopesAllConstants(t *testing.T) {
	all := chzzkgo.Scopes{
		chzzkgo.ChannelInfoRead,
		chzzkgo.ChannelManagerRead,
		chzzkgo.LiveStreamKeyRead,
		chzzkgo.LiveSettingRead,
		chzzkgo.LiveSettingWrite,
		chzzkgo.UserRead,
		chzzkgo.ChatMessageRead,
		chzzkgo.ChatMessageWrite,
		chzzkgo.ChatNoticeWrite,
		chzzkgo.ChatSettingsRead,
		chzzkgo.ChatSettingsWrite,
		chzzkgo.RestrictionWrite,
		chzzkgo.RestrictionRead,
		chzzkgo.DonationRead,
		chzzkgo.SubscriptionRead,
	}

	got := chzzkgo.ParseScopes(all.String())

	if !slices.Equal(got, all) {
		t.Errorf("ParseScopes(all.String()) = %v, want %v", got, all)
	}
}

func TestScopesJSONRoundTrip(t *testing.T) {
	original := chzzkgo.Scopes{chzzkgo.ChannelInfoRead, chzzkgo.UserRead}

	b, err := json.Marshal(original)

	if err != nil {
		t.Fatal(err)
	}

	if want := `"채널 정보 조회 유저 조회"`; string(b) != want {
		t.Errorf("Marshal = %s, want %s", b, want)
	}

	var restored chzzkgo.Scopes

	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(restored, original) {
		t.Errorf("round trip = %v, want %v", restored, original)
	}
}

// TestTokensJSONRoundTrip은 Tokens를 JSON으로 저장했다가 복원해도
// 전 필드가 원형 유지되는지 검증한다. (토큰 저장은 사용자 책임이므로 핵심 계약)
func TestTokensJSONRoundTrip(t *testing.T) {
	original := chzzkgo.Tokens{
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresIn:    86400,
		Scope:        chzzkgo.Scopes{chzzkgo.UserRead, chzzkgo.ChatMessageWrite},
	}

	b, err := json.Marshal(original)

	if err != nil {
		t.Fatal(err)
	}

	var restored chzzkgo.Tokens

	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatal(err)
	}

	if restored.AccessToken != original.AccessToken ||
		restored.RefreshToken != original.RefreshToken ||
		restored.ExpiresIn != original.ExpiresIn ||
		!slices.Equal(restored.Scope, original.Scope) {
		t.Errorf("round trip = %+v, want %+v", restored, original)
	}
}

func TestScopesHas(t *testing.T) {
	s := chzzkgo.Scopes{chzzkgo.UserRead, chzzkgo.ChatMessageRead}

	if !s.Has(chzzkgo.UserRead) {
		t.Error("Has(UserRead) = false, want true")
	}

	if s.Has(chzzkgo.RestrictionWrite) {
		t.Error("Has(RestrictionWrite) = true, want false")
	}
}

func TestScopesString(t *testing.T) {
	s := chzzkgo.Scopes{chzzkgo.UserRead, chzzkgo.DonationRead}

	if got, want := s.String(), "유저 조회 후원 조회"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}

	if got := (chzzkgo.Scopes{}).String(); got != "" {
		t.Errorf("empty String() = %q, want empty", got)
	}
}
