package server

import (
	"net/http"
	"strings"

	"codeberg.org/codeberg/pages/server/cache"
	"codeberg.org/codeberg/pages/server/context"
	"codeberg.org/codeberg/pages/server/utils"
)

func SetupHTTPACMEChallengeServer(challengeCache cache.SetGetKey) http.HandlerFunc {
	challengePath := "/.well-known/acme-challenge/"

	return func(w http.ResponseWriter, req *http.Request) {
		ctx := context.New(w, req)
		if strings.HasPrefix(ctx.Path(), challengePath) {
			challenge, ok := challengeCache.Get(utils.TrimHostPort(ctx.Host()) + "/" + string(strings.TrimPrefix(ctx.Path(), challengePath)))
			if !ok || challenge == nil {
				ctx.String("no challenge for this token", http.StatusNotFound)
			}
			ctx.String(challenge.(string))
		} else {
			ctx.Redirect("https://"+string(ctx.Host())+string(ctx.Path()), http.StatusMovedPermanently)
		}
	}
}
