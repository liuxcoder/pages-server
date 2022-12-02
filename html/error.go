package html

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"codeberg.org/codeberg/pages/server/context"
)

// ReturnErrorPage sets the response status code and writes NotFoundPage to the response body,
// with "%status%" and %message% replaced with the provided statusCode and msg
func ReturnErrorPage(ctx *context.Context, msg string, statusCode int) {
	ctx.RespWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
	ctx.RespWriter.WriteHeader(statusCode)

	if msg == "" {
		msg = errorBody(statusCode)
	} else {
		// TODO: use template engine
		msg = strings.ReplaceAll(strings.ReplaceAll(ErrorPage, "%message%", msg), "%status%", http.StatusText(statusCode))
	}

	_, _ = ctx.RespWriter.Write([]byte(msg))
}

func errorMessage(statusCode int) string {
	message := http.StatusText(statusCode)

	switch statusCode {
	case http.StatusMisdirectedRequest:
		message += " - domain not specified in <code>.domains</code> file"
	case http.StatusFailedDependency:
		message += " - target repo/branch doesn't exist or is private"
	}

	return message
}

// TODO: use template engine
func errorBody(statusCode int) string {
	return template.HTMLEscapeString(
		strings.ReplaceAll(NotFoundPage,
			"%status%",
			strconv.Itoa(statusCode)+" "+errorMessage(statusCode)))
}
