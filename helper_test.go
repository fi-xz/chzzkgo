package chzzkgo_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/fi-xz/chzzkgo"
)

// newMockClient는 handler로 응답하는 로컬 서버를 띄우고 그쪽을 바라보는 클라이언트를 반환한다.
func newMockClient(t *testing.T, handler http.Handler) *chzzkgo.ChzzkClient {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	chzzk := chzzkgo.NewChzzkClient("test-client-id", "test-client-secret", "http://localhost:12940/callback")
	chzzk.SetBaseURL(srv.URL)

	return chzzk
}

// writeEnvelope는 치지직 공통 응답 형식 {code, message, content}로 응답을 기록한다.
func writeEnvelope(t *testing.T, w http.ResponseWriter, status, code int, message string, content any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	err := json.NewEncoder(w).Encode(map[string]any{
		"code":    code,
		"message": message,
		"content": content,
	})
	if err != nil {
		t.Errorf("Failed to encode envelope: %v", err)
	}
}

// loadFixture는 testdata 디렉토리의 응답 fixture 파일을 읽는다.
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()

	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("Failed to read fixture %s: %v", name, err)
	}

	return b
}

// serveFixture는 fixture 파일 내용을 그대로 응답 본문으로 기록한다.
func serveFixture(t *testing.T, w http.ResponseWriter, name string) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")

	if _, err := w.Write(loadFixture(t, name)); err != nil {
		t.Errorf("Failed to write fixture %s: %v", name, err)
	}
}
