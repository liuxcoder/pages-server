package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"codeberg.org/codeberg/pages/config"
	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/gitea"
	"github.com/rs/zerolog/log"
)

func TestHandlerPerformance(t *testing.T) {
	cfg := config.GiteaConfig{
		Root:           "https://codeberg.org",
		Token:          "",
		LFSEnabled:     false,
		FollowSymlinks: false,
	}
	giteaClient, _ := gitea.NewClient(cfg, cache.NewInMemoryCache())
	serverCfg := config.ServerConfig{
		MainDomain: "codeberg.page",
		RawDomain:  "raw.codeberg.page",
		BlacklistedPaths: []string{
			"/.well-known/acme-challenge/",
		},
		AllowedCorsDomains: []string{"raw.codeberg.org", "fonts.codeberg.org", "design.codeberg.org"},
		PagesBranches:      []string{"pages"},
	}
	testHandler := Handler(serverCfg, giteaClient, cache.NewInMemoryCache(), cache.NewInMemoryCache(), cache.NewInMemoryCache())

	testCase := func(uri string, status int) {
		t.Run(uri, func(t *testing.T) {
			req := httptest.NewRequest("GET", uri, http.NoBody)
			w := httptest.NewRecorder()

			log.Printf("Start: %v\n", time.Now())
			start := time.Now()
			testHandler(w, req)
			end := time.Now()
			log.Printf("Done: %v\n", time.Now())

			resp := w.Result()

			if resp.StatusCode != status {
				t.Errorf("request failed with status code %d", resp.StatusCode)
			} else {
				t.Logf("request took %d milliseconds", end.Sub(start).Milliseconds())
			}
		})
	}

	testCase("https://mondstern.codeberg.page/", 404) // TODO: expect 200
	testCase("https://codeberg.page/", 404)           // TODO: expect 200
	testCase("https://example.momar.xyz/", 424)
}
