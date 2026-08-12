package chzzkgo

import (
	"context"
)

// Category는 치지직에서 사용되는 카테고리 정보를 나타낸다.
type Category struct {
	// 카테고리 타입. [CategoryType] 상수 참고.
	CategoryType CategoryType `json:"categoryType"`
	// 내부 카테고리 ID, 영문/숫자/특수문자 조합
	CategoryID string `json:"categoryId"`
	// 한국어로 표시되는 카테고리 이름
	CategoryValue string `json:"categoryValue"`
	// 카테고리 포스트 이미지 URL
	PosterImageURL string `json:"posterImageUrl"`
}

// CategoryPages는 카테고리 검색 결과를 담는 구조체이다.
type CategoryPages struct {
	// 카테고리 정보 목록
	Data []Category `json:"data"`
}

// CategoryType은 카테고리의 유형을 나타낸다.
type CategoryType string

const (
	// CategoryTypeGame은 게임 카테고리이다.
	CategoryTypeGame CategoryType = "GAME"
	// CategoryTypeSports는 스포츠 카테고리이다.
	CategoryTypeSports CategoryType = "SPORTS"
	// CategoryTypeEtc는 기타 카테고리이다.
	CategoryTypeEtc CategoryType = "ETC"
)

// SearchCategory는 입력된 query에 대해 카테고리 정보를 검색한다.
//
// Client 인증을 사용하므로 OAuth 토큰 없이 호출 가능하다.
// 선택적 파라미터로 size를 지정할 수 있다. [WithSize]를 참고.
// size가 지정되지 않았다면 서버에서 기본값 20을 사용하며, 최소 1에서 최대 50까지 지정 가능하다.
func (c *Client) SearchCategory(ctx context.Context, query string, opts ...QueryOption) (*CategoryPages, error) {
	q := buildQuery(opts...)

	if err := validateSize(q, 1, 50); err != nil {
		return nil, err
	}

	q.Set("query", query)

	categoryPages, err := getWithClient[CategoryPages](c, ctx, "/open/v1/categories/search", q)

	if err != nil {
		return nil, err
	}

	return categoryPages, nil
}
