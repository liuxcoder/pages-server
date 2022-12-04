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

	msg = generateResponse(msg, statusCode)

	_, _ = ctx.RespWriter.Write([]byte(msg))
}

// TODO: use template engine
func generateResponse(msg string, statusCode int) string {
	if msg == "" {
		msg = strings.ReplaceAll(NotFoundPage,
			"%status%",
			strconv.Itoa(statusCode)+" "+errorMessage(statusCode))
	} else {
		msg = strings.ReplaceAll(
			strings.ReplaceAll(ErrorPage, "%message%", template.HTMLEscapeString(msg)),
			"%status%",
			http.StatusText(statusCode))
	}

	return msg
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
