package chzzkgo

import "fmt"

// APIError는 치지직 Open API가 2xx 외의 상태 코드를 반환했을 때의 오류이다.
type APIError struct {
	// StatusCode는 HTTP 상태 코드이다.
	StatusCode int
	// Code는 응답 본문에 포함된 치지직 API 오류 코드이다.
	Code int
	// Message는 응답 본문에 포함된 오류 메시지이다.
	Message string
	// Path는 요청한 API 경로이다.
	Path string
	// Method는 요청한 HTTP 메서드이다.
	Method string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("chzzkgo: %s %s failed (http %d, code %d): %s", e.Method, e.Path, e.StatusCode, e.Code, e.Message)
}

// MissingScopeError는 API 호출에 필요한 [Scope]가 토큰에 없을 때의 오류이다.
//
// 이 검사는 클라이언트에 설정된 권한 목록 기준의 사전 검사이며,
// 실제 권한 판정은 서버가 수행한다.
type MissingScopeError struct {
	// Scope는 부족한 권한이다.
	Scope Scope
}

func (e *MissingScopeError) Error() string {
	return fmt.Sprintf("chzzkgo: missing required scope: %s", e.Scope)
}
