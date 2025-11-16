package http

import (
	"time"

	"ad-engine/internal/logger"

	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
)

const (
	maxLoggedBody = 2048 // максимум байт тела, которые пишем в лог
)

// withMiddlewares — общий конвейер для всех HTTP-хендлеров.
func withMiddlewares(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	lm := NewLoggingMiddleware()
	return recoverMiddleware(lm.Wrap(next))
}

// Logging middleware

type LoggingMiddleware struct{}

func NewLoggingMiddleware() *LoggingMiddleware {
	return &LoggingMiddleware{}
}

// Wrap — оборачиваем fasthttp.Handler логированием request/response.
func (m *LoggingMiddleware) Wrap(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		start := time.Now()

		// Request ID
		reqID := string(ctx.Request.Header.Peek("X-Request-Id"))
		if reqID == "" {
			reqID = uuid.NewString()
		}

		ctx.SetUserValue("request_id", reqID)
		ctx.Response.Header.Set("X-Request-Id", reqID)

		// Request Info
		method := string(ctx.Method())
		path := string(ctx.Path())
		ip := ctx.RemoteIP().String()

		// headers / body (ограниченный)
		reqHeaders := ctx.Request.Header.String()
		reqBody := ctx.PostBody()
		if len(reqBody) > maxLoggedBody {
			reqBody = reqBody[:maxLoggedBody]
		}

		// выполняем следующий обработчик
		next(ctx)

		// Response Info
		status := ctx.Response.StatusCode()
		duration := time.Since(start).Milliseconds()

		respHeaders := ctx.Response.Header.String()
		respBody := ctx.Response.Body()
		if len(respBody) > maxLoggedBody {
			respBody = respBody[:maxLoggedBody]
		}

		// Logging
		logger.LogInfo(
			"http access log",
			"req_id", reqID,
			"method", method,
			"path", path,
			"status", status,
			"duration_ms", duration,
			"ip", ip,
			"req_headers", reqHeaders,
			"req_body", string(reqBody),
			"resp_headers", respHeaders,
			"resp_body", string(respBody),
		)
	}
}

// Recover middleware

func recoverMiddleware(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		defer func() {
			if r := recover(); r != nil {
				// попробуем вытащить req_id, если его уже создал логгер
				rid, _ := ctx.UserValue("request_id").(string)

				logger.LogError(
					"panic recovered",
					"panic", r,
					"req_id", rid,
					"path", string(ctx.Path()),
				)
				ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			}
		}()
		next(ctx)
	}
}
