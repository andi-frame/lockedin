package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	nethttp "net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/http/api"
)

const problemContentType = "application/problem+json"

// errorHandler turns every error into RFC 9457 problem+json with a stable code
// (ARCHITECTURE §3). Domain errors keep their code, framework errors map by status,
// and anything else is a 500 whose real cause only reaches the logs.
func errorHandler(log *slog.Logger) fiber.ErrorHandler {
	return func(c fiber.Ctx, err error) error {
		p := toProblem(err)
		if p.Status >= 500 {
			log.Error("request failed", "err", err, "request_id", requestid.FromContext(c), "method", c.Method(), "path_pattern", c.Route().Path)
		}
		return writeProblem(c, p)
	}
}

func toProblem(err error) api.Problem {
	var de *domain.Error
	if errors.As(err, &de) {
		status, ok := StatusFor(de.Code)
		if !ok {
			// A code the contract does not know would reach the web app unmapped.
			return problem(api.ServerInternal, 500, "")
		}
		detail := de.Msg
		switch api.ErrorCode(de.Code) {
		case api.PactInvalidTerms, api.ProofInvalidDoc, api.ValidationFailed:
			detail = err.Error() // the wrapped text names the offending rule or field
		}
		return problem(api.ErrorCode(de.Code), status, detail)
	}

	var fe *fiber.Error
	if errors.As(err, &fe) {
		if status, ok := StatusFor(fe.Message); ok && status == fe.Code {
			return problem(api.ErrorCode(fe.Message), status, "") // e.g. the auth middleware's code-only errors
		}
		switch {
		case fe.Code == fiber.StatusNotFound:
			return problem(api.NotFound, fe.Code, "")
		case fe.Code == fiber.StatusMethodNotAllowed:
			return problem(api.MethodNotAllowed, fe.Code, "")
		case fe.Code == fiber.StatusRequestEntityTooLarge:
			return problem(api.RequestTooLarge, fe.Code, "")
		case fe.Code == fiber.StatusUnsupportedMediaType:
			return problem(api.RequestUnsupportedMediaType, fe.Code, "")
		case fe.Code == fiber.StatusTooManyRequests:
			return problem(api.RateLimited, fe.Code, "")
		case fe.Code == fiber.StatusUnauthorized:
			return problem(api.AuthUnauthenticated, fe.Code, "")
		case fe.Code >= 400 && fe.Code < 500:
			return problem(api.ValidationFailed, fe.Code, fe.Message)
		}
	}
	return problem(api.ServerInternal, 500, "")
}

func problem(code api.ErrorCode, status int, detail string) api.Problem {
	p := api.Problem{Type: "about:blank", Title: nethttp.StatusText(status), Status: status, Code: code}
	if detail != "" {
		p.Detail = &detail
	}
	return p
}

func writeProblem(c fiber.Ctx, p api.Problem) error {
	if id := requestid.FromContext(c); id != "" {
		p.RequestId = &id
	}
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if p.Code == api.UploadQueueBusy {
		c.Set(fiber.HeaderRetryAfter, "10") // seconds; the web app backs off and retries
	}
	c.Status(p.Status)
	c.Set(fiber.HeaderContentType, problemContentType)
	return c.Send(body)
}

// problemError lets middleware return a coded error without a domain dependency cycle.
func problemError(code api.ErrorCode, msg string) error {
	return &domain.Error{Code: string(code), Msg: msg}
}
