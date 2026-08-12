package chzzkgo

import (
	"log/slog"
	"net/http"
	"time"
)

// Client는 치지직 Open API 호출을 위한 클라이언트이다.
//
// [New]로 생성한다. OAuth 토큰이 필요한 API를 호출하려면
// [Client.SetTokens]로 토큰을 주입하거나 [LoginServer]를 통해 로그인해야 한다.
// 토큰의 저장과 복원은 사용자 책임이며, 갱신된 토큰은 [Client.OnTokenRefresh]
// 콜백으로 전달받아 저장할 수 있다.
type Client struct {
	// ClientID는 치지직 개발자 센터에서 발급받은 클라이언트 ID이다.
	ClientID string
	// ClientSecret은 치지직 개발자 센터에서 발급받은 클라이언트 시크릿이다.
	ClientSecret string
	// RedirectURI는 애플리케이션에 등록한 OAuth 리디렉션 URI이다.
	RedirectURI string

	http    *http.Client
	auth    *authManager
	logger  *slog.Logger
	baseURL string
}

// defaultBaseURL은 치지직 Open API의 기본 URL이다.
const defaultBaseURL = "https://openapi.chzzk.naver.com"

// apiEnvelope는 치지직 Open API의 공통 응답 형식이다.
type apiEnvelope[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Content T      `json:"content"`
}

// New는 새 [Client]를 생성한다.
//
// clientID와 clientSecret은 치지직 개발자 센터에서 발급받은 값을,
// redirectURI는 애플리케이션에 등록한 리디렉션 URI를 전달한다.
func New(clientID, clientSecret, redirectURI string) *Client {
	c := &Client{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		baseURL:      defaultBaseURL,
	}

	c.auth = &authManager{client: c}
	c.http = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &authTransport{
			base: http.DefaultTransport,
			auth: c.auth,
		},
	}

	return c
}

// SetBaseURL은 API 요청의 기본 URL을 교체한다.
// 테스트 서버나 프록시를 경유할 때 사용하며, 기본값은 https://openapi.chzzk.naver.com 이다.
// API 호출을 시작하기 전에 설정해야 한다.
func (c *Client) SetBaseURL(u string) {
	c.baseURL = u
}

// SetLogger는 클라이언트 내부 동작(토큰 갱신 등)을 기록할 로거를 설정한다.
// 설정하지 않으면 아무것도 기록하지 않는다.
func (c *Client) SetLogger(l *slog.Logger) {
	c.auth.mu.Lock()
	defer c.auth.mu.Unlock()
	c.logger = l
}
