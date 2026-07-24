package chzzkgo

import (
	"io"
	"net/http"
	"strings"
)

// authTransport는 /open/v1 경로의 요청에 인증 정보를 부착하는 [http.RoundTripper]이다.
// Bearer 인증 요청이 401로 실패하면 토큰을 1회 갱신한 뒤 원요청을 재시도한다.
type authTransport struct {
	base http.RoundTripper
	auth *authManager
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !strings.HasPrefix(req.URL.Path, "/open/v1") {
		return t.base.RoundTrip(req)
	}

	if authModeOf(req.Context()) == authClient {
		r := req.Clone(req.Context())
		r.Header.Set("Client-Id", t.auth.client.ClientID)
		r.Header.Set("Client-Secret", t.auth.client.ClientSecret)
		return t.base.RoundTrip(r)
	}

	if tok, ok := tokenOverrideOf(req.Context()); ok {
		return t.doWithToken(req, tok)
	}

	tokens := t.auth.current()

	if tokens == nil || tokens.AccessToken == "" {
		return nil, ErrNotAuthenticated
	}

	resp, err := t.doWithToken(req, tokens.AccessToken)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	newAccessToken, err := t.auth.refresh(req.Context(), tokens.AccessToken)

	if err != nil {
		return resp, nil // Return the original 401 response if the token refresh fails
	}

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	retry := req.Clone(req.Context())

	if req.Body != nil && req.GetBody != nil {
		body, err := req.GetBody()

		if err != nil {
			return nil, err
		}

		retry.Body = body
	}

	return t.doWithToken(retry, newAccessToken)
}

func (t *authTransport) doWithToken(req *http.Request, accessToken string) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+accessToken)
	return t.base.RoundTrip(req)
}
