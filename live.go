package chzzkgo

import (
	"context"
)

// Live는 치지직에서 사용되는 생방송 정보를 나타낸다.
type Live struct {
	// 생방송 ID
	LiveID int `json:"liveId"`
	// 생방송 제목
	LiveTitle string `json:"liveTitle"`
	// 생방송 썸네일 이미지 URL
	LiveThumbnailImageURL string `json:"liveThumbnailImageUrl"`
	// 동시 시청자 수
	ConcurrentUserCount int `json:"concurrentUserCount"`
	// 생방송 시작 날짜
	OpenDate string `json:"openDate"`
	// 연령 제한 방송 여부
	Adult bool `json:"adult"`
	// 생방송 태그. 지정되지 않은 경우 빈 배열
	Tags []string `json:"tags"`
	// 생방송 카테고리, [CategoryType] 참고. 지정되지 않은 경우 null
	CategoryType CategoryType `json:"categoryType,omitempty"`
	// 생방송 카테고리 ID, [Category.CategoryID]와 동일. 지정되지 않은 경우 null
	LiveCategory string `json:"liveCategory,omitempty"`
	// 생방송 카테고리 이름, [Category.CategoryValue]와 동일. 지정되지 않은 경우 null
	LiveCategoryValue string `json:"liveCategoryValue,omitempty"`
	// 생방송 채널 ID
	ChannelID string `json:"channelId"`
	// 생방송 채널 이름
	ChannelName string `json:"channelName"`
	// 생방송 채널 프로필 이미지 URL
	ChannelImageURL string `json:"channelImageUrl"`
}

// LivePages는 생방송 검색 결과를 담는 구조체이다.
type LivePages struct {
	// 생방송 정보 목록
	Data []Live `json:"data"`
	// 다음 페이지 조회를 위한 구조체, 마지막 페이지인 경우 null
	Page struct {
		// 다음 페이지 조회를 위한 토큰
		Next string `json:"next"`
	} `json:"page"`
}

// LiveStreamKey는 생방송 스트림 키 정보를 나타낸다.
type LiveStreamKey struct {
	// 생방송 스트림 키
	StreamKey string `json:"streamKey"`
}

// LiveSettings는 생방송 설정 정보를 나타낸다.
type LiveSettings struct {
	// 생방송 기본 제목
	DefaultLiveTitle string `json:"defaultLiveTitle"`
	// 생방송 카테고리, [Category] 참고
	Category Category `json:"category"`
	// 생방송 태그
	Tags []string `json:"tags"`
}

// LiveSettingsPatch는 [Client.SetLiveSettings]로 변경할 방송 설정 정보를 나타낸다.
//
// nil 필드는 전송되지 않으며, 서버는 전송되지 않은 필드의 기존 값을 유지한다.
// 값 지정에는 내장 함수 new를 사용할 수 있다. (예: new("새 제목"))
type LiveSettingsPatch struct {
	// 생방송 기본 제목.
	DefaultLiveTitle *string `json:"defaultLiveTitle,omitempty"`
	// 생방송 카테고리 종류, [CategoryType] 참고.
	CategoryType *CategoryType `json:"categoryType,omitempty"`
	// 생방송 카테고리 ID, [Category.CategoryID]와 동일. 제거를 원하는 경우 빈 문자열("")을 지정한다.
	CategoryID *string `json:"categoryId,omitempty"`
	// 생방송 태그. 공백 및 특수문자 비허용. 제거를 원하는 경우 빈 슬라이스를 지정한다.
	Tags *[]string `json:"tags,omitempty"`
}

// GetLiveList는 현재 치지직에 존재하는 생방송 목록을 조회한다.
//
// Client 인증을 사용하므로 OAuth 토큰 없이 호출 가능하다.
// 선택적 파라미터로 size, next를 지정할 수 있다. [WithSize], [WithNext]를 참고.
// size의 경우, 지정되지 않았다면 서버에서 기본값 20을 사용하며, 최소 1에서 최대 20까지 지정 가능하다.
func (c *Client) GetLiveList(ctx context.Context, opts ...QueryOption) (*LivePages, error) {
	q := buildQuery(opts...)

	if err := validateSize(q, 1, 20); err != nil {
		return nil, err
	}

	livePages, err := getWithClient[LivePages](c, ctx, "/open/v1/lives", q)

	if err != nil {
		return nil, err
	}

	return livePages, nil
}

// GetStreamKey는 방송 스트림 키를 조회한다.
//
// [LiveStreamKeyRead](방송 스트림키 조회) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
func (c *Client) GetStreamKey(ctx context.Context) (*LiveStreamKey, error) {
	if err := c.requireScope(LiveStreamKeyRead); err != nil {
		return nil, err
	}

	return get[LiveStreamKey](c, ctx, "/open/v1/streams/key", nil)
}

// GetLiveSettings는 방송 설정 정보를 조회한다.
//
// [LiveSettingRead](방송 설정 조회) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
func (c *Client) GetLiveSettings(ctx context.Context) (*LiveSettings, error) {
	if err := c.requireScope(LiveSettingRead); err != nil {
		return nil, err
	}

	return get[LiveSettings](c, ctx, "/open/v1/lives/setting", nil)
}

// SetLiveSettings는 방송 설정 정보를 변경한다.
//
// settings에는 변경할 방송 설정 정보를 담은 [LiveSettingsPatch] 구조체를 전달한다.
// nil인 필드는 요청에서 제외되어 변경되지 않는다.
//
// [LiveSettingWrite](방송 설정 변경) [Scope]가 필요하며, 없으면 [MissingScopeError]를 반환한다.
func (c *Client) SetLiveSettings(ctx context.Context, settings LiveSettingsPatch) error {
	if err := c.requireScope(LiveSettingWrite); err != nil {
		return err
	}

	// Result: {"code": 200,"message": "SUCCESS","content": null}
	_, err := patch[empty](c, ctx, "/open/v1/lives/setting", settings)

	if err != nil {
		return err
	}

	return nil
}
