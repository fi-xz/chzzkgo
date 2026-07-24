package chzzkgo_test

import (
	"context"
	"net/http"
	"testing"
)

func TestSearchCategory(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/categories/search", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("query"); got != "이터널 리턴" {
			t.Errorf("query = %q, want 이터널 리턴", got)
		}

		serveFixture(t, w, "search_category.json")
	})

	// Client 인증 — OAuth 토큰 미주입
	chzzk := newMockClient(t, mux)

	categories, err := chzzk.SearchCategory(context.Background(), "이터널 리턴")

	if err != nil {
		t.Fatal(err)
	}

	if len(categories.Data) == 0 {
		t.Fatal("no categories decoded")
	}

	cat := categories.Data[0]

	if cat.CategoryType == "" || cat.CategoryID == "" || cat.CategoryValue == "" {
		t.Errorf("Data[0] = %+v", cat)
	}
}
