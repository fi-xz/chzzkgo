package chzzkgo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
)

// TokenRequest는 토큰 발급/갱신 요청의 본문을 나타낸다.
// 일반적으로 직접 사용할 일은 없으며, [Client.ExchangeCode]와
// 자동 토큰 갱신이 내부적으로 사용한다.
type TokenRequest struct {
	// GrantType은 발급 방식이다. ("authorization_code" 또는 "refresh_token")
	GrantType string `json:"grantType"`
	// ClientID는 치지직 개발자 센터에서 발급받은 클라이언트 ID이다.
	ClientID string `json:"clientId"`
	// ClientSecret은 치지직 개발자 센터에서 발급받은 클라이언트 시크릿이다.
	ClientSecret string `json:"clientSecret"`
	// Code는 OAuth 인증 후 리디렉션으로 전달받은 인증 코드이다. (authorization_code 방식)
	Code string `json:"code,omitempty"`
	// State는 인증 요청 시 전달한 state 값이다. (authorization_code 방식)
	State string `json:"state,omitempty"`
	// RefreshToken은 토큰 갱신에 사용할 리프레시 토큰이다. (refresh_token 방식)
	RefreshToken string `json:"refreshToken,omitempty"`
}

// RevokeTokenRequest는 토큰 제거 요청의 본문을 나타낸다.
// [Client.RevokeToken]에 전달한다.
type RevokeTokenRequest struct {
	// ClientID는 치지직 개발자 센터에서 발급받은 클라이언트 ID이다.
	// 비어 있으면 클라이언트에 설정된 값이 사용된다.
	ClientID string `json:"clientId"`
	// ClientSecret은 치지직 개발자 센터에서 발급받은 클라이언트 시크릿이다.
	// 비어 있으면 클라이언트에 설정된 값이 사용된다.
	ClientSecret string `json:"clientSecret"`
	// Token은 제거할 토큰이다. (액세스 토큰 또는 리프레시 토큰)
	Token string `json:"token"`
	// TokenTypeHint는 제거할 토큰의 종류이다. ("access_token" 또는 "refresh_token")
	TokenTypeHint string `json:"tokenTypeHint,omitempty"`
}

// Tokens는 발급된 OAuth 토큰 정보를 나타낸다.
type Tokens struct {
	// AccessToken은 API 호출에 사용하는 액세스 토큰이다.
	AccessToken string `json:"accessToken"`
	// RefreshToken은 액세스 토큰 갱신에 사용하는 리프레시 토큰이다.
	RefreshToken string `json:"refreshToken"`
	// ExpiresIn은 액세스 토큰의 유효 기간(초)이다.
	ExpiresIn int `json:"expiresIn"`
	// Scope는 토큰에 부여된 권한 목록이다.
	Scope Scopes `json:"scope"`
}

type authManager struct {
	client    *Client
	onRefresh func(Tokens)

	mu     sync.Mutex
	tokens *Tokens
}

// ErrNotAuthenticated는 OAuth 토큰이 필요한 API를 토큰 없이 호출했을 때 반환된다.
var ErrNotAuthenticated = errors.New("chzzkgo: not authenticated, please complete OAuth flow first")

func (a *authManager) current() *Tokens {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.tokens
}

func (a *authManager) refresh(ctx context.Context, oldAccessToken string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.tokens != nil && a.tokens.AccessToken != "" && a.tokens.AccessToken != oldAccessToken {
		return a.tokens.AccessToken, nil // Already refreshed by another goroutine
	}

	if a.tokens == nil || a.tokens.RefreshToken == "" {
		a.tokens = nil
		return "", ErrNotAuthenticated
	}

	if a.client.logger != nil {
		a.client.logger.Info("[Auth] Access token expired. Attempting to refresh...")
	}

	newTokens, err := a.client.RequestToken(ctx, TokenRequest{
		GrantType:    "refresh_token",
		ClientID:     a.client.ClientID,
		ClientSecret: a.client.ClientSecret,
		RefreshToken: a.tokens.RefreshToken,
	})

	if err != nil {
		a.tokens = nil
		return "", err
	}

	a.tokens = newTokens
	if a.onRefresh != nil {
		go a.onRefresh(*newTokens)
	}

	return a.tokens.AccessToken, nil
}

func (c *Client) requireScope(scope Scope) error {
	tokens := c.auth.current()
	if tokens == nil {
		return ErrNotAuthenticated
	}
	if !tokens.Scope.Has(scope) {
		return &MissingScopeError{Scope: scope}
	}
	return nil
}

// GetAuthorizationURL은 사용자를 이동시킬 치지직 OAuth 인증 페이지 URL을 반환한다.
//
// state는 CSRF 방지를 위한 값으로, 호출자가 생성하고 리디렉션 시 검증해야 한다.
func (c *Client) GetAuthorizationURL(state string) string {
	q := url.Values{}

	q.Set("clientId", c.ClientID)
	q.Set("redirectUri", c.RedirectURI)
	q.Set("state", state)
	return "https://chzzk.naver.com/account-interlock?" + q.Encode()
}

// ExchangeCode는 OAuth 리디렉션으로 전달받은 인증 코드를 토큰으로 교환하여 반환한다.
//
// 반환된 토큰은 클라이언트에 자동으로 주입되지 않는다.
// 이 클라이언트로 API를 호출하려면 [Client.SetTokens]로 직접 주입해야 한다.
// state의 생성과 검증은 호출자가 관리한다.
func (c *Client) ExchangeCode(ctx context.Context, code, state string) (*Tokens, error) {
	tokens, err := c.RequestToken(ctx, TokenRequest{
		GrantType:    "authorization_code",
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		Code:         code,
		State:        state,
	})

	if err != nil {
		return nil, err
	}

	return tokens, nil
}

// postAuthToken은 /auth/v1 토큰 API에 body를 전송하고 응답 토큰을 반환한다.
// opName은 오류 메시지에 사용된다.
//
// c.http를 사용하지만 /auth/v1 경로는 authTransport가 관여하지 않으므로 안전하다.
// 주의: 이 함수는 authManager.refresh가 mutex를 잡은 채 호출한다 —
// 토큰 endpoint가 authTransport의 bearer 경로를 타게 되면 데드락이다.
func (c *Client) postAuthToken(ctx context.Context, path, opName string, body any) (*Tokens, error) {
	b, err := json.Marshal(body)

	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(b))

	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var env apiEnvelope[Tokens]

	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK || env.Code != 200 {
		return nil, fmt.Errorf("chzzkgo: %s failed with status code %d: %s", opName, resp.StatusCode, env.Message)
	}

	return &env.Content, nil
}

// RequestToken은 토큰 발급/갱신 API를 직접 호출한다.
//
// 일반적으로는 [Client.ExchangeCode]와 자동 토큰 갱신을 사용하면 되며,
// 발급 흐름을 직접 제어해야 하는 경우에만 사용한다.
func (c *Client) RequestToken(ctx context.Context, body TokenRequest) (*Tokens, error) {
	tokens, err := c.postAuthToken(ctx, "/auth/v1/token", "token request", body)

	if err != nil {
		return nil, err
	}

	return tokens, nil
}

// RevokeToken은 Access Token 또는 Refresh Token을 제거한다.
//
// body에는 제거할 토큰(Token)과 토큰 종류(TokenTypeHint)를 담은 [RevokeTokenRequest]를 전달한다.
// ClientID와 ClientSecret이 비어 있으면 클라이언트에 설정된 값이 자동으로 채워진다.
// 제거된 토큰은 더 이상 API 호출에 사용할 수 없으며, 리프레시 토큰 제거 시 액세스 토큰도 함께 무효화된다.
func (c *Client) RevokeToken(ctx context.Context, body RevokeTokenRequest) error {
	if body.ClientID == "" {
		body.ClientID = c.ClientID
	}

	if body.ClientSecret == "" {
		body.ClientSecret = c.ClientSecret
	}

	_, err := c.postAuthToken(ctx, "/auth/v1/token/revoke", "token revoke request", body)

	if err != nil {
		return err
	}

	return nil
}

// SetTokens은 클라이언트가 API 호출에 사용할 인증 토큰 데이터를 주입한다.
//
// 저장해 둔 토큰을 복원하거나 [Client.ExchangeCode]로 발급받은 토큰을
// 등록할 때 사용한다. scope는 토큰 발급 시 부여된 권한 목록을 전달한다.
func (c *Client) SetTokens(accessToken, refreshToken string, scope Scopes) {
	c.auth.mu.Lock()
	defer c.auth.mu.Unlock()
	c.auth.tokens = &Tokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Scope:        scope,
	}
}

// SetAccessToken은 클라이언트가 API 호출에 사용할 Access Token 데이터만을 주입한다.
func (c *Client) SetAccessToken(accessToken string) {
	c.auth.mu.Lock()
	defer c.auth.mu.Unlock()

	if c.auth.tokens == nil {
		c.auth.tokens = &Tokens{}
	}

	c.auth.tokens.AccessToken = accessToken
}

// SetRefreshToken은 클라이언트가 API 호출에 사용할 Refresh Token 데이터만을 주입한다.
func (c *Client) SetRefreshToken(refreshToken string) {
	c.auth.mu.Lock()
	defer c.auth.mu.Unlock()

	if c.auth.tokens == nil {
		c.auth.tokens = &Tokens{}
	}

	c.auth.tokens.RefreshToken = refreshToken
}

// SetScope는 클라이언트가 API 호출에 사용할 Scope 데이터만을 주입한다.
func (c *Client) SetScope(scope Scopes) {
	c.auth.mu.Lock()
	defer c.auth.mu.Unlock()

	if c.auth.tokens == nil {
		c.auth.tokens = &Tokens{}
	}

	c.auth.tokens.Scope = scope
}

// OnTokenRefresh는 자동 토큰 갱신 성공 시 새 토큰을 전달받을 콜백을 등록한다.
//
// 갱신된 토큰을 저장소에 반영하는 용도로 사용한다.
// 콜백은 별도 goroutine에서 호출되므로 클라이언트 메서드를 자유롭게 호출할 수 있다.
func (c *Client) OnTokenRefresh(callback func(Tokens)) {
	c.auth.mu.Lock()
	defer c.auth.mu.Unlock()
	c.auth.onRefresh = callback
}
