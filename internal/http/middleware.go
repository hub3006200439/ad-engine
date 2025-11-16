package http

import (
	"strings"
	"time"

	"ad-engine/internal/logger"

	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
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

		// ---------- Request ID ----------
		reqID := string(ctx.Request.Header.Peek("X-Request-Id"))
		if reqID == "" {
			reqID = uuid.NewString()
		}
		// кладём в контекст, чтобы дальше в хендлерах можно было достать
		ctx.SetUserValue("request_id", reqID)
		// и в ответ — чтобы клиент мог коррелировать
		ctx.Response.Header.Set("X-Request-Id", reqID)

		// Request Info
		var request = struct {
			ReqId       string
			Method      string
			URI         string
			UserIP      string
			Headers     string
			RequestSize int
			Timestamp   string
		}{
			reqID,
			string(ctx.Method()),
			string(ctx.Request.URI().PathOriginal()),
			ctx.RemoteIP().String(),
			strings.Join(strings.Fields(ctx.Request.Header.String()), " ") + ",",
			len(ctx.Request.Body()),
			start.Format(time.RFC3339),
		}

		logger.LogInformation("HTTP request received: {@Request}", request)

		// выполняем следующий обработчик
		next(ctx)

		// Response Info

		var response = struct {
			ReqId        string
			URI          string
			UserIP       string
			Headers      string
			StatusCode   int
			ResponseSize int
			Duration     string
		}{
			reqID,
			string(ctx.Request.URI().PathOriginal()),
			ctx.RemoteIP().String(),
			strings.Join(strings.Fields(ctx.Response.Header.String()), " ") + ",",
			ctx.Response.StatusCode(),
			len(ctx.Response.Body()),
			time.Since(start).String(),
		}

		logger.LogInformation("HTTP response send: {@Response}", response)
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

/*
func (m *LoggingMiddleware) Wrap(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		start := time.Now()

		var request = struct {
			Method      string
			URI         string
			UserIP      string
			UserAgent   string
			RequestSize int
			RequestId   uint64
			Timestamp   string
		}{
			string(ctx.Method()),
			string(ctx.Request.URI().PathOriginal()),
			ctx.RemoteIP().String(),
			string(ctx.Request.Header.Peek("User-Agent")),
			len(ctx.Request.Body()),
			ctx.ID(),
			start.Format(time.RFC3339),
		}

		logger.LogInformation("HTTP request received: {@Request}", request)

		next(ctx)


	}
}

*/
