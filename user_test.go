package chzzkgo_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/fi-xz/chzzkgo"
)

func TestGetUser(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/users/me", func(w http.ResponseWriter, r *http.Request) {
		serveFixture(t, w, "get_user.json")
	})

	chzzk := newMockClient(t, mux)
	chzzk.SetTokens("access", "refresh", chzzkgo.Scopes{chzzkgo.UserRead})

	user, err := chzzk.GetUser(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	if user.ChannelID == "" {
		t.Error("ChannelID is empty")
	}

	if user.ChannelName == "" || user.Nickname == "" {
		t.Error("ChannelName or Nickname is empty")
	}
}
