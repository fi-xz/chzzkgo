package chzzkgo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// LoginServer는 치지직 OAuth 로그인 흐름을 처리하는 로컬 HTTP 서버이다.
//
// [ChzzkClient.NewLoginServer]로 생성하고 [LoginServer.Start]로 실행한다.
// /login 경로에서 인증 페이지로 리디렉션하고, RedirectURI 경로에서 인증 코드를
// 받아 토큰으로 교환한다. state는 crypto/rand로 생성되어 10분간 유효하며 일회용으로 소비된다.
type LoginServer struct {
	client      *ChzzkClient
	keepAlive   bool         // true면 로그인 후에도 서버 유지 (상시 endpoint)
	setTokens   bool         // true면 로그인 성공 시 client에 토큰 주입
	onLogin     func(Tokens) // 로그인 성공마다 호출 (상시 모드의 토큰 전달 통로)
	successHTML string

	mu     sync.Mutex
	states map[string]time.Time
}

// LoginServerOption은 [LoginServer]의 동작을 설정하는 함수이다.
// [WithKeepAlive], [WithOnLogin], [WithSuccessPage] 참고.
type LoginServerOption func(*LoginServer)

// WithKeepAlive는 로그인 성공 후에도 서버를 유지한다.
// 여러 사용자의 로그인을 받는 상시 endpoint에 사용한다.
func WithKeepAlive() LoginServerOption {
	return func(s *LoginServer) {
		s.keepAlive = true
		s.setTokens = false // 상시 모드 = 다중 계정 전제, client 토큰 오염 방지
	}
}

// WithOnLogin은 로그인 성공 시마다 발급된 토큰을 전달받을 콜백을 등록한다.
func WithOnLogin(fn func(Tokens)) LoginServerOption {
	return func(s *LoginServer) { s.onLogin = fn }
}

// WithSuccessPage는 로그인 완료 시 브라우저에 표시할 HTML을 교체한다.
func WithSuccessPage(html string) LoginServerOption {
	return func(s *LoginServer) { s.successHTML = html }
}

// NewLoginServer는 새 [LoginServer]를 생성한다.
//
// 기본값은 일회용 모드로, 첫 로그인 성공 시 서버가 종료되고
// 발급된 토큰이 클라이언트에 주입된다. 상시 모드는 [WithKeepAlive]를 참고.
func (c *ChzzkClient) NewLoginServer(opts ...LoginServerOption) *LoginServer {
	s := &LoginServer{
		client:      c,
		setTokens:   true, // 일회용 모드 기본값: client에 주입
		successHTML: "<html><body>로그인 완료. 이 창을 닫아도 됩니다.</body></html>",
		states:      make(map[string]time.Time),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Start는 RedirectURI에서 파싱한 포트로 서버를 열고 블록한다.
// 일회용 모드: 첫 로그인 성공 후 자동 종료, 발급 토큰 반환.
// 상시 모드(WithKeepAlive): ctx 취소 전까지 유지, 토큰은 WithOnLogin 콜백으로만 전달.
func (s *LoginServer) Start(ctx context.Context) (*Tokens, error) {
	redirect, err := url.Parse(s.client.RedirectURI)
	if err != nil {
		return nil, err
	}

	if redirect.Port() == "" {
		return nil, errors.New("chzzkgo: RedirectURI must include an explicit port")
	}

	done := make(chan *Tokens, 1)
	mux := http.NewServeMux()

	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		state, err := s.issueState()
		if err != nil {
			http.Error(w, "failed to issue state", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, s.client.GetAuthorizationURL(state), http.StatusFound)
	})

	mux.HandleFunc(redirect.Path, func(w http.ResponseWriter, r *http.Request) {
		code, state := r.URL.Query().Get("code"), r.URL.Query().Get("state")
		if code == "" || state == "" || !s.consumeState(state) {
			http.Error(w, "invalid code or state", http.StatusBadRequest)
			return
		}

		tokens, err := s.client.ExchangeCode(r.Context(), code, state)
		if err != nil {
			http.Error(w, "token exchange failed", http.StatusBadGateway)
			return
		}

		if s.setTokens {
			s.client.SetTokens(tokens.AccessToken, tokens.RefreshToken, tokens.Scope)
		}
		if s.onLogin != nil {
			go s.onLogin(*tokens)
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(s.successHTML))

		if !s.keepAlive {
			select {
			case done <- tokens:
			default: // 이미 완료됨
			}
		}
	})

	srv := &http.Server{Addr: ":" + redirect.Port(), Handler: mux}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case tokens := <-done: // 일회용 모드 완료
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
		return tokens, nil
	case <-ctx.Done(): // 취소 또는 상시 모드 종료 지시
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	}
}

func (s *LoginServer) issueState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := hex.EncodeToString(b)

	s.mu.Lock()
	// 겸사겸사 만료 state 청소 (별도 goroutine 불필요)
	now := time.Now()
	for k, exp := range s.states {
		if now.After(exp) {
			delete(s.states, k)
		}
	}
	s.states[state] = now.Add(10 * time.Minute)
	s.mu.Unlock()
	return state, nil
}

func (s *LoginServer) consumeState(state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.states[state]
	if !ok {
		return false
	}
	delete(s.states, state)
	return time.Now().Before(exp)
}
