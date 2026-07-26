package chzzkgo_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/fi-xz/chzzkgo"
)

// TestEventTimeIsKST는 오프셋 없는 eventSentAt이 KST로 해석되는지 검증한다.
//
// 같은 CHAT 메시지의 messageTime(epoch 밀리초, UTC)과 eventSentAt은 서버 처리
// 지연만큼만 차이가 나야 한다. eventSentAt을 UTC로 읽으면 9시간이 어긋나는데,
// 오프셋 표기가 없어 파싱 단계에서는 오류가 나지 않으므로 여기서 잡는다.
func TestEventTimeIsKST(t *testing.T) {
	var chat chzzkgo.ChatEvent

	if err := json.Unmarshal(loadFixture(t, "session_chat_event.json"), &chat); err != nil {
		t.Fatal(err)
	}

	if chat.EventSentAt.IsZero() {
		t.Fatal("EventSentAt is zero")
	}

	// KST 03:14:18.629843820은 UTC 18:14:18.629843820(전날)이다.
	wantUTC := time.Date(2026, 7, 25, 18, 14, 18, 629843820, time.UTC)

	if got := chat.EventSentAt.UTC(); !got.Equal(wantUTC) {
		t.Errorf("EventSentAt.UTC() = %s, want %s", got.Format(time.RFC3339Nano), wantUTC.Format(time.RFC3339Nano))
	}

	// messageTime과의 차이는 서버 처리 지연 수준이어야 한다.
	// UTC로 잘못 읽으면 9시간이 벌어진다.
	diff := chat.EventSentAt.Sub(chat.MessageAt())

	if diff < 0 || diff > time.Second {
		t.Errorf("eventSentAt - messageTime = %v, want a sub-second gap", diff)
	}
}

// TestChatEventMessageAt은 messageTime이 epoch 밀리초(UTC)로 해석되는지 검증한다.
func TestChatEventMessageAt(t *testing.T) {
	var chat chzzkgo.ChatEvent

	if err := json.Unmarshal(loadFixture(t, "session_chat_event.json"), &chat); err != nil {
		t.Fatal(err)
	}

	if chat.MessageTime != 1785003258567 {
		t.Fatalf("MessageTime = %d", chat.MessageTime)
	}

	wantUTC := time.Date(2026, 7, 25, 18, 14, 18, 567000000, time.UTC)

	if got := chat.MessageAt().UTC(); !got.Equal(wantUTC) {
		t.Errorf("MessageAt().UTC() = %s, want %s", got.Format(time.RFC3339Nano), wantUTC.Format(time.RFC3339Nano))
	}

	// 표시용으로는 KST 시각이 나와야 한다.
	if h := chat.MessageAt().Hour(); h != 3 {
		t.Errorf("MessageAt().Hour() = %d, want 3 (KST)", h)
	}
}

// TestEventTimeRespectsExplicitOffset은 서버가 시간대를 명시한 경우
// 그것을 존중하는지 검증한다.
//
// 오프셋이 붙은 값에까지 KST를 씌우면 9시간이 어긋난다.
// 지금까지 관측된 페이로드에는 오프셋이 없었으나, 붙어서 오더라도
// 조용히 틀리지 않아야 한다.
func TestEventTimeRespectsExplicitOffset(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantUTC time.Time
	}{
		{
			"오프셋 없음 — KST로 해석",
			`"2026-07-26T03:14:18.629843820"`,
			time.Date(2026, 7, 25, 18, 14, 18, 629843820, time.UTC),
		},
		{
			"Z — UTC로 해석",
			`"2026-07-26T03:14:18.629843820Z"`,
			time.Date(2026, 7, 26, 3, 14, 18, 629843820, time.UTC),
		},
		{
			"+09:00 — 그대로 해석",
			`"2026-07-26T03:14:18.629843820+09:00"`,
			time.Date(2026, 7, 25, 18, 14, 18, 629843820, time.UTC),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var parsed chzzkgo.EventTime

			if err := json.Unmarshal([]byte(tc.raw), &parsed); err != nil {
				t.Fatal(err)
			}

			if got := parsed.UTC(); !got.Equal(tc.wantUTC) {
				t.Errorf("UTC() = %s, want %s", got.Format(time.RFC3339Nano), tc.wantUTC.Format(time.RFC3339Nano))
			}
		})
	}
}

func TestEventTimeJSONRoundTrip(t *testing.T) {
	const raw = `"2026-07-26T03:14:18.629843820"`

	var parsed chzzkgo.EventTime

	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatal(err)
	}

	b, err := json.Marshal(parsed)

	if err != nil {
		t.Fatal(err)
	}

	if string(b) != raw {
		t.Errorf("round trip = %s, want %s", b, raw)
	}
}

func TestEventTimeEmptyValues(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"null", `null`},
		{"빈 문자열", `""`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var parsed chzzkgo.EventTime

			if err := json.Unmarshal([]byte(tc.raw), &parsed); err != nil {
				t.Fatalf("unmarshal %s: %v", tc.raw, err)
			}

			if !parsed.IsZero() {
				t.Errorf("EventTime = %v, want zero", parsed)
			}

			b, err := json.Marshal(parsed)

			if err != nil {
				t.Fatal(err)
			}

			if string(b) != "null" {
				t.Errorf("marshal = %s, want null", b)
			}
		})
	}
}

func TestDonationEventDecode(t *testing.T) {
	var donation chzzkgo.DonationEvent

	if err := json.Unmarshal(loadFixture(t, "session_donation_event.json"), &donation); err != nil {
		t.Fatal(err)
	}

	if donation.PayAmount != 1000 {
		t.Errorf("PayAmount = %d, want 1000", donation.PayAmount)
	}

	if donation.DonationType != chzzkgo.DonationTypeChat {
		t.Errorf("DonationType = %q", donation.DonationType)
	}

	if donation.DonationText == "" || donation.DonatorChannelID == "" {
		t.Errorf("donation = %+v", donation)
	}

	wantUTC := time.Date(2026, 7, 25, 18, 14, 28, 690447802, time.UTC)

	if got := donation.EventSentAt.UTC(); !got.Equal(wantUTC) {
		t.Errorf("EventSentAt.UTC() = %s, want %s", got.Format(time.RFC3339Nano), wantUTC.Format(time.RFC3339Nano))
	}
}

func TestSystemEventDecode(t *testing.T) {
	t.Run("connected", func(t *testing.T) {
		var system chzzkgo.SystemEvent

		if err := json.Unmarshal(loadFixture(t, "session_system_connected.json"), &system); err != nil {
			t.Fatal(err)
		}

		if system.Type != chzzkgo.SystemEventTypeConnected {
			t.Errorf("Type = %q", system.Type)
		}

		if system.Data.SessionKey == "" {
			t.Error("SessionKey is empty")
		}
	})

	t.Run("revoked", func(t *testing.T) {
		var system chzzkgo.SystemEvent

		if err := json.Unmarshal(loadFixture(t, "session_system_revoked.json"), &system); err != nil {
			t.Fatal(err)
		}

		if system.Type != chzzkgo.SystemEventTypeRevoked {
			t.Errorf("Type = %q", system.Type)
		}

		if system.Data.EventType != chzzkgo.EventTypeChat || system.Data.ChannelID == "" {
			t.Errorf("Data = %+v", system.Data)
		}
	})

	// 문서에 없는 type이 와도 해석 자체는 성공해야 한다.
	t.Run("알 수 없는 type", func(t *testing.T) {
		var system chzzkgo.SystemEvent

		if err := json.Unmarshal([]byte(`{"type":"brand_new_type","data":{}}`), &system); err != nil {
			t.Fatal(err)
		}

		if system.Type != "brand_new_type" {
			t.Errorf("Type = %q", system.Type)
		}
	})
}
