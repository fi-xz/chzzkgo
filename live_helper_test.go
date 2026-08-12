//go:build live

package chzzkgo_test

import (
	"os"
	"testing"

	"github.com/fi-xz/chzzkgo"
	"github.com/joho/godotenv"
)

func TestMain(m *testing.M) {
	_ = godotenv.Load(".test.env")
	os.Exit(m.Run())
}

func requireEnv(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)

	if v == "" {
		t.Skipf("%s not set, skipping test", key)
	}

	return v
}

func newTestClient(t *testing.T, scopes chzzkgo.Scopes) *chzzkgo.Client {
	t.Helper()

	clientID := requireEnv(t, "CLIENT_ID")
	clientSecret := requireEnv(t, "CLIENT_SECRET")

	accessToken := requireEnv(t, "ACCESS_TOKEN")
	refreshToken := requireEnv(t, "REFRESH_TOKEN")

	chzzk := chzzkgo.New(clientID, clientSecret, "http://localhost:12940/callback")
	chzzk.SetTokens(accessToken, refreshToken, scopes)

	return chzzk
}

func newTestClientNoAuth(t *testing.T) *chzzkgo.Client {
	t.Helper()
	clientID := requireEnv(t, "CLIENT_ID")
	clientSecret := requireEnv(t, "CLIENT_SECRET")
	return chzzkgo.New(clientID, clientSecret, "http://localhost:12940/callback")
}
