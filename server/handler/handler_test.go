package handler

import (
	"net/http/httptest"
	"testing"
	"time"

	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/gitea"
	"github.com/rs/zerolog/log"
)

func TestHandlerPerformance(t *testing.T) {
	giteaClient, _ := gitea.NewClient("https://codeberg.org", "", cache.NewKeyValueCache(), false, false)
	testHandler := Handler(
		"codeberg.page", "raw.codeberg.org",
		giteaClient,
		"https://docs.codeberg.org/pages/raw-content/",
		[]string{"/.well-known/acme-challenge/"},
		[]string{"raw.codeberg.org", "fonts.codeberg.org", "design.codeberg.org"},
		cache.NewKeyValueCache(),
		cache.NewKeyValueCache(),
	)

	testCase := func(uri string, status int) {
		t.Run(uri, func(t *testing.T) {
			req := httptest.NewRequest("GET", uri, nil)
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
