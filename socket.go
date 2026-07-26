package chzzkgo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/fi-xz/socketio2"
)

// ErrSessionClosed는 세션 소켓이 닫힌 뒤에 사용하려 할 때 반환된다.
var ErrSessionClosed = errors.New("chzzkgo: session socket closed")

// SessionSocket은 세션 서버에 접속해 실시간 이벤트를 수신한다.
//
// [ChzzkClient.CreateSessionWithClient]나 [ChzzkClient.CreateSessionWithUser]가
// 반환한 세션 URL로 [NewSessionSocket]을 만들어 사용한다.
//
//	socket := chzzkgo.NewSessionSocket(session.URL)
//	socket.OnChat(func(e chzzkgo.ChatEvent) { ... })
//
//	if err := socket.Connect(ctx); err != nil {
//		return err
//	}
//	defer socket.Close()
//
//	// 연결만으로는 이벤트가 오지 않는다. 세션 키로 구독해야 한다.
//	if err := chzzk.SubscribeChatEvent(ctx, socket.SessionKey()); err != nil {
//		return err
//	}
//
//	<-socket.Done()
//	return socket.Err()
//
// 핸들러는 [SessionSocket.Connect] 전에 등록해야 한다. 서버가 연결 직후
// SYSTEM 이벤트를 보내므로 연결한 뒤에 등록하면 놓칠 수 있다.
//
// 재연결은 하지 않는다. 세션 URL은 한 번만 쓸 수 있어 같은 URL로 다시 접속하는
// 것이 옳지 않기 때문이다. [SessionSocket.Done]이 닫히면 [SessionSocket.Err]로
// 원인을 확인하고, 세션을 새로 발급받아 소켓을 다시 만들어야 한다.
// [ChzzkClient.ConnectSessionWithUser]와 [ChzzkClient.ConnectSessionWithClient]가
// 이 과정을 대신해 준다.
type SessionSocket struct {
	conn *socketio2.Client

	mu             sync.Mutex
	onChat         []func(ChatEvent)
	onDonation     []func(DonationEvent)
	onSubscription []func(SubscriptionEvent)
	onSystem       []func(SystemEvent)
	onAny          []func(event string, payload json.RawMessage)
	onError        []func(error)
	onDisconnect   []func(reason string)
	sessionKey     string

	keyReady chan struct{}
	keyOnce  sync.Once

	connectOnce sync.Once
}

// sessionSocketOptions는 [SessionSocket]의 설정값을 담는다.
type sessionSocketOptions struct {
	socket []socketio2.Option
}

// SessionSocketOption은 [NewSessionSocket]에 넘기는 설정이다.
type SessionSocketOption func(*sessionSocketOptions)

// WithSocketOptions는 하위 Socket.IO 클라이언트에 전달할 설정을 지정한다.
// HTTP 클라이언트나 읽기 제한 등을 바꿀 때 사용한다.
//
//	chzzkgo.NewSessionSocket(url, chzzkgo.WithSocketOptions(socketio2.WithReadLimit(4<<20)))
func WithSocketOptions(opts ...socketio2.Option) SessionSocketOption {
	return func(o *sessionSocketOptions) { o.socket = append(o.socket, opts...) }
}

// NewSessionSocket은 세션 URL에 접속할 소켓을 만든다.
// 실제 접속은 [SessionSocket.Connect]에서 이루어진다.
func NewSessionSocket(sessionURL string, opts ...SessionSocketOption) *SessionSocket {
	var o sessionSocketOptions

	for _, opt := range opts {
		opt(&o)
	}

	s := &SessionSocket{
		conn:     socketio2.New(sessionURL, o.socket...),
		keyReady: make(chan struct{}),
	}

	s.conn.OnAny(s.dispatch)
	s.conn.OnError(s.fireError)
	s.conn.OnDisconnect(func(reason string) {
		s.mu.Lock()
		handlers := slices.Clone(s.onDisconnect)
		s.mu.Unlock()

		for _, h := range handlers {
			h(reason)
		}
	})

	return s
}

// OnChat은 채팅 이벤트 핸들러를 등록한다.
//
// [ChatMessageRead](채팅 메시지 조회) [Scope]로 채팅 이벤트를 구독해야 호출된다.
// [ChzzkClient.SubscribeChatEvent] 참고.
func (s *SessionSocket) OnChat(fn func(ChatEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onChat = append(s.onChat, fn)
}

// OnDonation은 후원 이벤트 핸들러를 등록한다.
//
// [DonationRead](후원 조회) [Scope]로 후원 이벤트를 구독해야 호출된다.
// [ChzzkClient.SubscribeDonationEvent] 참고.
func (s *SessionSocket) OnDonation(fn func(DonationEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onDonation = append(s.onDonation, fn)
}

// OnSubscription은 구독 이벤트 핸들러를 등록한다.
//
// [SubscriptionRead](구독 조회) [Scope]로 구독 이벤트를 구독해야 호출된다.
// [ChzzkClient.SubscribeSubscriptionEvent] 참고.
//
// [SubscriptionEvent]는 실제 페이로드를 관측하지 못한 미검증 구조체이다.
func (s *SessionSocket) OnSubscription(fn func(SubscriptionEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onSubscription = append(s.onSubscription, fn)
}

// OnSystem은 시스템 이벤트 핸들러를 등록한다.
//
// 구독이 시작·해제되거나 서버가 구독을 취소할 때 호출된다.
// [SystemEventTypeRevoked]는 서버가 구독을 끊은 것이므로 그대로 두면 이후
// 이벤트가 오지 않는다.
//
// 연결 직후 오는 [SystemEventTypeConnected]는 [SessionSocket.SessionKey]로도
// 확인할 수 있으므로 이 핸들러에서 따로 처리하지 않아도 된다.
func (s *SessionSocket) OnSystem(fn func(SystemEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onSystem = append(s.onSystem, fn)
}

// OnAny는 모든 이벤트의 원본 본문을 받는 핸들러를 등록한다.
// payload는 이중 인코딩을 한 겹 벗긴 JSON이다.
//
// 문서에 없는 필드나 알 수 없는 이벤트를 확인할 때 사용한다.
// 이름별 핸들러보다 먼저 호출된다.
func (s *SessionSocket) OnAny(fn func(event string, payload json.RawMessage)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onAny = append(s.onAny, fn)
}

// OnError는 이벤트 본문을 해석하지 못했을 때와 연결을 유지한 채 발생한
// 하위 계층 오류를 알리는 핸들러를 등록한다.
// 연결이 끊기는 오류는 [SessionSocket.Err]로 확인한다.
func (s *SessionSocket) OnError(fn func(error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onError = append(s.onError, fn)
}

// OnDisconnect는 서버가 연결을 끊었을 때 호출될 핸들러를 등록한다.
func (s *SessionSocket) OnDisconnect(fn func(reason string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onDisconnect = append(s.onDisconnect, fn)
}

// Connect는 세션 서버에 접속하고, 서버가 세션 키를 보낼 때까지 기다린 뒤 반환한다.
//
// 반환 후에는 [SessionSocket.SessionKey]로 세션 키를 얻어 구독 API를 호출할 수 있다.
// ctx는 접속이 끝날 때까지만 사용되며, 연결 수명은 [SessionSocket.Close]를
// 호출하거나 서버가 연결을 끊을 때까지이다.
//
// 한 소켓은 한 번만 연결할 수 있다.
func (s *SessionSocket) Connect(ctx context.Context) error {
	err := errors.New("chzzkgo: session socket already connected")

	s.connectOnce.Do(func() {
		err = s.connect(ctx)
	})

	return err
}

// connect는 접속과 세션 키 수신 대기를 수행한다.
func (s *SessionSocket) connect(ctx context.Context) error {
	if err := s.conn.Connect(ctx); err != nil {
		return fmt.Errorf("chzzkgo: connect session socket: %w", err)
	}

	select {
	case <-s.keyReady:
		return nil
	case <-s.conn.Done():
		if err := s.Err(); err != nil {
			return fmt.Errorf("chzzkgo: session socket closed before the session key arrived: %w", err)
		}

		return ErrSessionClosed
	case <-ctx.Done():
		s.conn.Close()
		return ctx.Err()
	}
}

// SessionKey는 서버가 보낸 세션 키를 반환한다.
//
// 이 값으로 [ChzzkClient.SubscribeChatEvent] 등을 호출해야 실제 이벤트가 오기
// 시작한다. [SessionSocket.Connect]가 성공했다면 항상 채워져 있다.
func (s *SessionSocket) SessionKey() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionKey
}

// Close는 연결을 닫는다.
func (s *SessionSocket) Close() error {
	return s.conn.Close()
}

// Done은 연결이 끝나면 닫히는 채널을 반환한다.
// 끝난 이유는 [SessionSocket.Err]로 확인한다.
func (s *SessionSocket) Done() <-chan struct{} {
	return s.conn.Done()
}

// Err은 연결이 끝난 이유를 반환한다. 아직 연결 중이면 nil이고,
// [SessionSocket.Close]로 정상 종료했다면 [ErrSessionClosed]이다.
func (s *SessionSocket) Err() error {
	if err := s.conn.Err(); err != nil {
		if errors.Is(err, socketio2.ErrClosed) {
			return ErrSessionClosed
		}

		return err
	}

	return nil
}

// dispatch는 수신한 이벤트를 타입에 맞게 해석해 핸들러에 전달한다.
// 알 수 없는 이벤트나 해석할 수 없는 본문이 와도 연결을 끊지 않는다.
func (s *SessionSocket) dispatch(event string, args []json.RawMessage) {
	var payload json.RawMessage

	if len(args) > 0 {
		payload = unwrapEventPayload(args[0])
	}

	s.mu.Lock()
	anyHandlers := slices.Clone(s.onAny)
	s.mu.Unlock()

	for _, h := range anyHandlers {
		h(event, payload)
	}

	if len(payload) == 0 {
		return
	}

	switch event {
	case string(EventTypeChat):
		if parsed, ok := decodeEvent[ChatEvent](s, event, payload); ok {
			s.mu.Lock()
			handlers := slices.Clone(s.onChat)
			s.mu.Unlock()

			for _, h := range handlers {
				h(parsed)
			}
		}
	case string(EventTypeDonation):
		if parsed, ok := decodeEvent[DonationEvent](s, event, payload); ok {
			s.mu.Lock()
			handlers := slices.Clone(s.onDonation)
			s.mu.Unlock()

			for _, h := range handlers {
				h(parsed)
			}
		}
	case string(EventTypeSubscription):
		if parsed, ok := decodeEvent[SubscriptionEvent](s, event, payload); ok {
			s.mu.Lock()
			handlers := slices.Clone(s.onSubscription)
			s.mu.Unlock()

			for _, h := range handlers {
				h(parsed)
			}
		}
	case systemEventName:
		s.dispatchSystem(payload)
	}
}

// systemEventName은 세션 상태를 알리는 이벤트의 이름이다.
// [Event] 상수와 달리 구독 대상이 아니라 항상 수신된다.
const systemEventName = "SYSTEM"

// decodeEvent는 이벤트 본문을 T로 해석한다.
// 해석에 실패하면 오류 핸들러에 알리고 false를 반환한다.
func decodeEvent[T any](s *SessionSocket, event string, payload json.RawMessage) (T, bool) {
	var parsed T

	if err := json.Unmarshal(payload, &parsed); err != nil {
		s.fireError(fmt.Errorf("chzzkgo: decode %s event: %w", event, err))
		return parsed, false
	}

	return parsed, true
}

// dispatchSystem은 시스템 이벤트를 해석해 세션 키를 갱신하고 핸들러에 전달한다.
func (s *SessionSocket) dispatchSystem(payload json.RawMessage) {
	parsed, ok := decodeEvent[SystemEvent](s, systemEventName, payload)

	if !ok {
		return
	}

	if parsed.Type == SystemEventTypeConnected && parsed.Data.SessionKey != "" {
		s.mu.Lock()
		s.sessionKey = parsed.Data.SessionKey
		s.mu.Unlock()

		s.keyOnce.Do(func() { close(s.keyReady) })
	}

	s.mu.Lock()
	handlers := slices.Clone(s.onSystem)
	s.mu.Unlock()

	for _, h := range handlers {
		h(parsed)
	}
}

// fireError는 등록된 오류 핸들러에 오류를 알린다.
func (s *SessionSocket) fireError(err error) {
	s.mu.Lock()
	handlers := slices.Clone(s.onError)
	s.mu.Unlock()

	for _, h := range handlers {
		h(err)
	}
}

// ConnectSessionWithUser는 사용자 인증으로 세션을 새로 발급받아 소켓을 연결한다.
//
// setup은 접속 전에 호출되므로 여기에서 핸들러를 등록한다.
// 연결이 끊긴 뒤 다시 붙을 때도 이 함수를 다시 호출하면 된다.
// 세션 URL은 한 번만 쓸 수 있어 같은 URL로 재접속할 수 없기 때문이다.
//
//	register := func(s *chzzkgo.SessionSocket) {
//		s.OnChat(func(e chzzkgo.ChatEvent) { ... })
//	}
//
//	for {
//		socket, err := chzzk.ConnectSessionWithUser(ctx, register)
//		if err != nil {
//			return err
//		}
//
//		if err := chzzk.SubscribeChatEvent(ctx, socket.SessionKey()); err != nil {
//			return err
//		}
//
//		<-socket.Done()
//	}
func (c *ChzzkClient) ConnectSessionWithUser(ctx context.Context, setup func(*SessionSocket), opts ...SessionSocketOption) (*SessionSocket, error) {
	session, err := c.CreateSessionWithUser(ctx)

	if err != nil {
		return nil, err
	}

	return connectSession(ctx, session.URL, setup, opts)
}

// ConnectSessionWithClient는 클라이언트 인증으로 세션을 새로 발급받아 소켓을 연결한다.
//
// Client 인증을 사용하므로 OAuth 토큰 없이 호출 가능하다.
// setup은 접속 전에 호출되므로 여기에서 핸들러를 등록한다.
// [ChzzkClient.ConnectSessionWithUser] 참고.
func (c *ChzzkClient) ConnectSessionWithClient(ctx context.Context, setup func(*SessionSocket), opts ...SessionSocketOption) (*SessionSocket, error) {
	session, err := c.CreateSessionWithClient(ctx)

	if err != nil {
		return nil, err
	}

	return connectSession(ctx, session.URL, setup, opts)
}

// connectSession은 세션 URL로 소켓을 만들어 핸들러를 등록하고 연결한다.
func connectSession(ctx context.Context, sessionURL string, setup func(*SessionSocket), opts []SessionSocketOption) (*SessionSocket, error) {
	socket := NewSessionSocket(sessionURL, opts...)

	if setup != nil {
		setup(socket)
	}

	if err := socket.Connect(ctx); err != nil {
		return nil, err
	}

	return socket, nil
}
