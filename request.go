package chzzkgo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type authMode int

const (
	authBearer authMode = iota
	authClient          // Client-Id, Client-Secret 헤더
)

type authModeKey struct{}

// empty는 content가 null인 응답을 디코딩할 때 사용하는 빈 구조체이다.
type empty struct{}

type tokenOverrideKey struct{}

// WithAccessToken은 반환된 context로 실행되는 요청에 한해
// 클라이언트에 설정된 토큰 대신 강제 지정한 액세스 토큰을 사용한다.
//
// Client 인증으로 발급받은 세션 키에 다른 채널의 토큰으로 이벤트를 구독하는 경우 등에 사용한다.
// [ChzzkClient.SubscribeChatEvent], [ChzzkClient.SubscribeDonationEvent], [ChzzkClient.SubscribeSubscriptionEvent] 참고.
//
// 이 토큰의 [Scope] 검증은 서버에서 수행되며, 권한 부족 시 서버 오류가 반환된다.
func WithAccessToken(ctx context.Context, accessToken string) context.Context {
	return context.WithValue(ctx, tokenOverrideKey{}, accessToken)
}

func tokenOverrideOf(ctx context.Context) (string, bool) {
	tok, ok := ctx.Value(tokenOverrideKey{}).(string)
	return tok, ok && tok != ""
}

func do[T any](c *ChzzkClient, ctx context.Context, method, path string, query url.Values, body any) (*T, error) {
	u := c.baseURL + path

	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reqBody io.Reader

	if body != nil {
		b, err := json.Marshal(body)

		if err != nil {
			return nil, err
		}

		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)

	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	var env apiEnvelope[T]

	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, fmt.Errorf("chzzkgo: %s %s: body decode failed with status code %d: %w", method, path, resp.StatusCode, err)
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Code:       env.Code,
			Message:    env.Message,
			Path:       path,
			Method:     method,
		}
	}

	return &env.Content, nil
}

// QueryOption은 조회 API의 선택적 쿼리 파라미터를 설정하는 함수이다.
// [WithPage], [WithSize], [WithSort], [WithNext] 참고.
type QueryOption func(url.Values)

// WithPage는 조회할 페이지 번호를 지정한다. 페이지는 0부터 시작한다.
func WithPage(page int) QueryOption {
	return func(q url.Values) {
		q.Set("page", strconv.Itoa(page))
	}
}

// WithSize는 한 페이지에 조회할 항목 개수를 지정한다.
// 허용 범위는 API마다 다르며, 각 메서드의 문서를 참고한다.
func WithSize(size int) QueryOption {
	return func(q url.Values) {
		q.Set("size", strconv.Itoa(size))
	}
}

// WithSort는 정렬 기준을 지정한다.
func WithSort(sort string) QueryOption {
	return func(q url.Values) {
		q.Set("sort", sort)
	}
}

// WithNext는 다음 페이지 조회를 위한 토큰을 지정한다.
// 이전 응답의 Page.Next 값을 전달한다.
func WithNext(next string) QueryOption {
	return func(q url.Values) {
		q.Set("next", next)
	}
}

func buildQuery(options ...QueryOption) url.Values {
	q := url.Values{}
	for _, opt := range options {
		opt(q)
	}
	return q
}

// validateSize는 쿼리에 지정된 size 값이 [minSize, maxSize] 범위인지 검사한다.
// size가 지정되지 않은 경우 서버 기본값을 사용하므로 검사하지 않는다.
func validateSize(q url.Values, minSize, maxSize int) error {
	s := q.Get("size")

	if s == "" {
		return nil
	}

	n, err := strconv.Atoi(s)

	if err != nil {
		return fmt.Errorf("chzzkgo: invalid size value: %s", s)
	}

	if n < minSize || n > maxSize {
		return fmt.Errorf("chzzkgo: size must be between %d and %d", minSize, maxSize)
	}

	return nil
}

func withAuthMode(ctx context.Context, m authMode) context.Context {
	return context.WithValue(ctx, authModeKey{}, m)
}

func authModeOf(ctx context.Context) authMode {
	if m, ok := ctx.Value(authModeKey{}).(authMode); ok {
		return m
	}

	return authBearer
}

func get[T any](c *ChzzkClient, ctx context.Context, path string, query url.Values) (*T, error) {
	return do[T](c, ctx, http.MethodGet, path, query, nil)
}

func getWithClient[T any](c *ChzzkClient, ctx context.Context, path string, query url.Values) (*T, error) {
	return do[T](c, withAuthMode(ctx, authClient), http.MethodGet, path, query, nil)
}

func post[T any](c *ChzzkClient, ctx context.Context, path string, body any) (*T, error) {
	return do[T](c, ctx, http.MethodPost, path, nil, body)
}

func put[T any](c *ChzzkClient, ctx context.Context, path string, body any) (*T, error) {
	return do[T](c, ctx, http.MethodPut, path, nil, body)
}

func patch[T any](c *ChzzkClient, ctx context.Context, path string, body any) (*T, error) {
	return do[T](c, ctx, http.MethodPatch, path, nil, body)
}

func del[T any](c *ChzzkClient, ctx context.Context, path string, body any) (*T, error) {
	return do[T](c, ctx, http.MethodDelete, path, nil, body)
}
