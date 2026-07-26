package chzzkgo_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/fi-xz/chzzkgo"
)

// fakeSessionServer는 CHZZK 세션 서버를 흉내내는 Socket.IO 2.x 서버이다.
//
// Engine.IO revision 3의 open 패킷과 루트 네임스페이스 CONNECT를 보낸 뒤,
// 테스트가 요청한 이벤트를 순서대로 전송한다. 클라이언트의 ping에는 pong으로 답한다.
type fakeSessionServer struct {
	srv *httptest.Server

	mu   sync.Mutex
	conn *websocket.Conn

	ready chan struct{}
	once  sync.Once
}

// newFakeSessionServer는 서버를 띄우고 세션 URL을 반환한다.
func newFakeSessionServer(t *testing.T) (*fakeSessionServer, string) {
	t.Helper()

	f := &fakeSessionServer{ready: make(chan struct{})}

	mux := http.NewServeMux()
	mux.HandleFunc("/socket.io/", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("EIO"); got != "3" {
			t.Errorf("EIO = %q, want 3", got)
		}

		if got := r.URL.Query().Get("auth"); got != "test-token" {
			t.Errorf("auth = %q, want test-token", got)
		}

		conn, err := websocket.Accept(w, r, nil)

		if err != nil {
			t.Errorf("websocket accept: %v", err)
			return
		}

		defer conn.CloseNow()

		f.mu.Lock()
		f.conn = conn
		f.mu.Unlock()

		ctx := r.Context()

		// Engine.IO open 패킷과 루트 네임스페이스 CONNECT.
		f.write(ctx, `0{"sid":"test-sid","upgrades":[],"pingInterval":25000,"pingTimeout":60000}`)
		f.write(ctx, "40")

		f.once.Do(func() { close(f.ready) })

		// 클라이언트가 보내는 ping에 pong으로 답한다. 연결이 끊길 때까지 유지한다.
		for {
			_, data, err := conn.Read(ctx)

			if err != nil {
				return
			}

			if string(data) == "2" {
				f.write(ctx, "3")
			}
		}
	})

	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)

	return f, f.srv.URL + "/?auth=test-token"
}

// write는 프레임 하나를 보낸다. 여러 고루틴에서 호출해도 안전하다.
func (f *fakeSessionServer) write(ctx context.Context, frame string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.conn == nil {
		return
	}

	_ = f.conn.Write(ctx, websocket.MessageText, []byte(frame))
}

// emit은 이벤트를 보낸다. payload는 CHZZK 서버와 같이 JSON 문자열로 한 겹 더 감싼다.
func (f *fakeSessionServer) emit(t *testing.T, event string, payload []byte) {
	t.Helper()

	<-f.ready

	// 고루틴에서도 호출되므로 t.Fatal을 쓰지 않는다.
	frame, err := json.Marshal([]any{event, string(payload)})

	if err != nil {
		t.Errorf("marshal %s frame: %v", event, err)
		return
	}

	f.write(context.Background(), "42"+string(frame))
}

// emitRaw는 이중 인코딩 없이 인자를 그대로 보낸다.
func (f *fakeSessionServer) emitRaw(t *testing.T, event string, arg string) {
	t.Helper()

	<-f.ready
	f.write(context.Background(), `42["`+event+`",`+arg+`]`)
}

// connectFakeSession은 가짜 서버에 연결된 소켓을 만든다.
// setup에서 핸들러를 등록하고, 서버는 곧바로 SYSTEM connected를 보낸다.
func connectFakeSession(t *testing.T, setup func(*chzzkgo.SessionSocket)) (*fakeSessionServer, *chzzkgo.SessionSocket) {
	t.Helper()

	server, sessionURL := newFakeSessionServer(t)
	socket := chzzkgo.NewSessionSocket(sessionURL)

	if setup != nil {
		setup(socket)
	}

	go server.emit(t, "SYSTEM", loadFixture(t, "session_system_connected.json"))

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	if err := socket.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	t.Cleanup(func() { socket.Close() })

	return server, socket
}

func TestSessionSocketConnect(t *testing.T) {
	_, socket := connectFakeSession(t, nil)

	// Connect는 SYSTEM connected가 도착할 때까지 기다린 뒤 반환한다.
	if got := socket.SessionKey(); got != "00000000-0000-4000-8000-000000000001" {
		t.Errorf("SessionKey = %q", got)
	}
}

func TestSessionSocketConnectTwice(t *testing.T) {
	_, socket := connectFakeSession(t, nil)

	if err := socket.Connect(t.Context()); err == nil {
		t.Error("want error on second Connect, got nil")
	}
}

func TestSessionSocketChatEvent(t *testing.T) {
	events := make(chan chzzkgo.ChatEvent, 1)

	server, _ := connectFakeSession(t, func(s *chzzkgo.SessionSocket) {
		s.OnChat(func(e chzzkgo.ChatEvent) { events <- e })
	})

	server.emit(t, "CHAT", loadFixture(t, "session_chat_event.json"))

	select {
	case e := <-events:
		if e.Content != "Test Chat" || e.ChatChannelID != "TEST_1" {
			t.Errorf("event = %+v", e)
		}

		// userRoleCode는 문서와 달리 profile 안에 있다.
		if e.Profile.UserRoleCode != chzzkgo.UserRoleCodeStreamer {
			t.Errorf("UserRoleCode = %q, want %q", e.Profile.UserRoleCode, chzzkgo.UserRoleCodeStreamer)
		}

		if len(e.Profile.Badges) != 1 || e.Profile.Badges[0].ImageURL == "" {
			t.Errorf("Badges = %+v", e.Profile.Badges)
		}

		// 블라인드 요청에 필요한 값이 그대로 옮겨져야 한다.
		want := chzzkgo.ChatBlindRequest{
			ChatChannelID:   e.ChatChannelID,
			MessageTime:     e.MessageTime,
			SenderChannelID: e.SenderChannelID,
		}

		if e.BlindRequest() != want {
			t.Errorf("BlindRequest() = %+v, want %+v", e.BlindRequest(), want)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("CHAT event not received")
	}
}

func TestSessionSocketDonationEvent(t *testing.T) {
	events := make(chan chzzkgo.DonationEvent, 1)

	server, _ := connectFakeSession(t, func(s *chzzkgo.SessionSocket) {
		s.OnDonation(func(e chzzkgo.DonationEvent) { events <- e })
	})

	server.emit(t, "DONATION", loadFixture(t, "session_donation_event.json"))

	select {
	case e := <-events:
		// payAmount는 문서와 달리 숫자로 온다.
		if e.PayAmount != 1000 {
			t.Errorf("PayAmount = %d, want 1000", e.PayAmount)
		}

		if e.DonationType != chzzkgo.DonationTypeChat || e.DonatorNickname != "TEST" {
			t.Errorf("event = %+v", e)
		}

		if e.EventSentAt.IsZero() {
			t.Error("EventSentAt is zero")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("DONATION event not received")
	}
}

// TestSessionSocketSystemRevoked는 서버가 구독을 취소했을 때
// 그 사실이 핸들러까지 전달되는지 검증한다.
func TestSessionSocketSystemRevoked(t *testing.T) {
	events := make(chan chzzkgo.SystemEvent, 2)

	server, _ := connectFakeSession(t, func(s *chzzkgo.SessionSocket) {
		s.OnSystem(func(e chzzkgo.SystemEvent) { events <- e })
	})

	// connected는 이미 전달되었으므로 걷어낸다.
	select {
	case <-events:
	case <-time.After(10 * time.Second):
		t.Fatal("connected event not received")
	}

	server.emit(t, "SYSTEM", loadFixture(t, "session_system_revoked.json"))

	select {
	case e := <-events:
		if e.Type != chzzkgo.SystemEventTypeRevoked {
			t.Errorf("Type = %q, want %q", e.Type, chzzkgo.SystemEventTypeRevoked)
		}

		if e.Data.EventType != chzzkgo.EventTypeChat || e.Data.ChannelID == "" {
			t.Errorf("Data = %+v", e.Data)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("revoked event not received")
	}
}

// TestSessionSocketUnknownEventsDoNotBreak은 문서에 없는 이벤트나
// 알 수 없는 시스템 타입이 와도 연결이 유지되는지 검증한다.
func TestSessionSocketUnknownEventsDoNotBreak(t *testing.T) {
	chats := make(chan chzzkgo.ChatEvent, 1)

	server, socket := connectFakeSession(t, func(s *chzzkgo.SessionSocket) {
		s.OnChat(func(e chzzkgo.ChatEvent) { chats <- e })
	})

	server.emitRaw(t, "UNKNOWN_EVENT", `"{\"foo\":1}"`)
	server.emit(t, "SYSTEM", []byte(`{"type":"brand_new_type","data":{}}`))

	// 이후에도 정상 이벤트가 처리되어야 한다.
	server.emit(t, "CHAT", loadFixture(t, "session_chat_event.json"))

	select {
	case <-chats:
	case <-socket.Done():
		t.Fatalf("socket closed: %v", socket.Err())
	case <-time.After(10 * time.Second):
		t.Fatal("CHAT event not received after unknown events")
	}
}

// TestSessionSocketMalformedPayload는 본문을 해석할 수 없을 때
// 오류 핸들러로 알리고 연결은 유지하는지 검증한다.
func TestSessionSocketMalformedPayload(t *testing.T) {
	errs := make(chan error, 1)

	server, socket := connectFakeSession(t, func(s *chzzkgo.SessionSocket) {
		s.OnError(func(err error) { errs <- err })
	})

	server.emit(t, "CHAT", []byte(`{"messageTime":"숫자가 아님"}`))

	select {
	case err := <-errs:
		if !strings.Contains(err.Error(), "CHAT") {
			t.Errorf("error = %v, want to mention CHAT", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("error not reported")
	}

	select {
	case <-socket.Done():
		t.Errorf("socket closed after malformed payload: %v", socket.Err())
	default:
	}
}

// TestSessionSocketOnAny는 원본 본문이 한 겹 벗겨진 채로 전달되는지 검증한다.
func TestSessionSocketOnAny(t *testing.T) {
	type received struct {
		event   string
		payload json.RawMessage
	}

	events := make(chan received, 4)

	server, _ := connectFakeSession(t, func(s *chzzkgo.SessionSocket) {
		s.OnAny(func(event string, payload json.RawMessage) {
			events <- received{event: event, payload: payload}
		})
	})

	server.emit(t, "CHAT", loadFixture(t, "session_chat_event.json"))

	for {
		select {
		case got := <-events:
			if got.event != "CHAT" {
				continue
			}

			// 이중 인코딩이 벗겨져 곧바로 객체로 해석되어야 한다.
			var chat chzzkgo.ChatEvent

			if err := json.Unmarshal(got.payload, &chat); err != nil {
				t.Fatalf("payload is not unwrapped: %v (%s)", err, got.payload)
			}

			if chat.Content != "Test Chat" {
				t.Errorf("Content = %q", chat.Content)
			}

			return
		case <-time.After(10 * time.Second):
			t.Fatal("CHAT event not received")
		}
	}
}

// TestSessionSocketErrAfterClose는 정상 종료 후 Err이 [chzzkgo.ErrSessionClosed]인지
// 검증한다. 하위 라이브러리의 센티널이 그대로 새어 나가면 안 된다.
func TestSessionSocketErrAfterClose(t *testing.T) {
	_, socket := connectFakeSession(t, nil)

	if err := socket.Err(); err != nil {
		t.Errorf("Err() before close = %v, want nil", err)
	}

	if err := socket.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case <-socket.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("Done not closed after Close")
	}

	if err := socket.Err(); !errors.Is(err, chzzkgo.ErrSessionClosed) {
		t.Errorf("Err() = %v, want ErrSessionClosed", err)
	}
}

// TestSessionSocketDoneOnServerClose는 서버가 연결을 끊으면
// Done이 닫히는지 검증한다. 재연결은 호출자의 몫이다.
func TestSessionSocketDoneOnServerClose(t *testing.T) {
	server, socket := connectFakeSession(t, nil)

	server.mu.Lock()
	conn := server.conn
	server.mu.Unlock()

	conn.Close(websocket.StatusNormalClosure, "bye")

	select {
	case <-socket.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("Done not closed after server disconnect")
	}
}
