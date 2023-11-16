package html

import (
	_ "embed"
	"net/http"
	"text/template" // do not use html/template here, we sanitize the message before passing it to the template

	"codeberg.org/codeberg/pages/server/context"
	"github.com/microcosm-cc/bluemonday"
	"github.com/rs/zerolog/log"
)

//go:embed templates/error.html
var errorPage string

var (
	errorTemplate = template.Must(template.New("error").Parse(errorPage))
	sanitizer     = createBlueMondayPolicy()
)

type TemplateContext struct {
	StatusCode int
	StatusText string
	Message    string
}

// ReturnErrorPage sets the response status code and writes the error page to the response body.
// The error page contains a sanitized version of the message and the statusCode both in text and numeric form.
//
// Currently, only the following html tags are supported: <code>
func ReturnErrorPage(ctx *context.Context, msg string, statusCode int) {
	ctx.RespWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
	ctx.RespWriter.WriteHeader(statusCode)

	templateContext := TemplateContext{
		StatusCode: statusCode,
		StatusText: http.StatusText(statusCode),
		Message:    sanitizer.Sanitize(msg),
	}

	err := errorTemplate.Execute(ctx.RespWriter, templateContext)
	if err != nil {
		log.Err(err).Str("message", msg).Int("status", statusCode).Msg("could not write response")
	}
}

func createBlueMondayPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()

	p.AllowElements("code")

	return p
}
