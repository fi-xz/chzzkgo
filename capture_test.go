//go:build live

package chzzkgo_test

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

// TestCaptureFixtures는 조회성 엔드포인트의 실제 응답을 testdata/*.json으로 저장한다.
// mock 테스트의 fixture를 문서 기반 더미에서 실캡처로 교체하는 일회성 도구.
//
// 실행: CAPTURE_FIXTURES=1 설정 후 go test -tags live -run '^TestCaptureFixtures$' -v .
//
// 캡처 제외 (비밀값 포함):
//   - /open/v1/streams/key (스트림 키)
//   - /open/v1/sessions/* (세션 URL의 auth 파라미터, 세션 키)
//
// 주의: 캡처 결과에는 실제 채널 ID·팔로워 목록 등이 포함된다.
// 저장된 파일을 검토한 뒤 커밋할 것.
func TestCaptureFixtures(t *testing.T) {
	if os.Getenv("CAPTURE_FIXTURES") != "1" {
		t.Skip("set CAPTURE_FIXTURES=1 to capture fixtures")
	}

	clientID := requireEnv(t, "CLIENT_ID")
	clientSecret := requireEnv(t, "CLIENT_SECRET")
	accessToken := requireEnv(t, "ACCESS_TOKEN")

	clientHeaders := map[string]string{
		"Client-Id":     clientID,
		"Client-Secret": clientSecret,
	}
	bearerHeaders := map[string]string{
		"Authorization": "Bearer " + accessToken,
	}

	captures := []struct {
		file    string
		path    string
		query   url.Values
		headers map[string]string
	}{
		{"get_user.json", "/open/v1/users/me", nil, bearerHeaders},
		{"get_channels.json", "/open/v1/channels", url.Values{"channelIds": {"c42cd75ec4855a9edf204a407c3c1dd2,04b9076004dfe8cb119835eb28dcc747"}}, clientHeaders},
		{"get_channel_managers.json", "/open/v1/channels/streaming-roles", nil, bearerHeaders},
		{"get_channel_followers.json", "/open/v1/channels/followers", nil, bearerHeaders},
		{"get_channel_subscribers.json", "/open/v1/channels/subscribers", nil, bearerHeaders},
		{"search_category.json", "/open/v1/categories/search", url.Values{"query": {"이터널 리턴"}}, clientHeaders},
		{"get_live_list.json", "/open/v1/lives", nil, clientHeaders},
		{"get_live_settings.json", "/open/v1/lives/setting", nil, bearerHeaders},
		{"get_chat_settings.json", "/open/v1/chats/settings", nil, bearerHeaders},
		{"get_restrictions.json", "/open/v1/restrict-channels", nil, bearerHeaders},
	}

	for _, c := range captures {
		t.Run(c.file, func(t *testing.T) {
			u := "https://openapi.chzzk.naver.com" + c.path

			if len(c.query) > 0 {
				u += "?" + c.query.Encode()
			}

			req, err := http.NewRequest(http.MethodGet, u, nil)

			if err != nil {
				t.Fatal(err)
			}

			for k, v := range c.headers {
				req.Header.Set(k, v)
			}

			resp, err := http.DefaultClient.Do(req)

			if err != nil {
				t.Fatal(err)
			}

			defer resp.Body.Close()

			raw, err := io.ReadAll(resp.Body)

			if err != nil {
				t.Fatal(err)
			}

			if resp.StatusCode != http.StatusOK {
				t.Errorf("%s: status %d, body: %s", c.path, resp.StatusCode, raw)
				return
			}

			if err := os.WriteFile(filepath.Join("testdata", c.file), raw, 0o644); err != nil {
				t.Fatal(err)
			}

			t.Logf("captured %s (%d bytes)", c.file, len(raw))
		})
	}
}
