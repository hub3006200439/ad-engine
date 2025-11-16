package http

import (
	"time"

	"ad-engine/internal/logger"

	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
)

func withMiddlewares(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return recoverMiddleware(requestIDMiddleware(loggingMiddleware(next)))
}

func loggingMiddleware(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		start := time.Now()
		next(ctx)
		latency := time.Since(start)

		rid, _ := ctx.UserValue("request_id").(string)

		l := logger.LogWith(
			"RequestId", rid,
			"Method", string(ctx.Method()),
			"Path", string(ctx.Path()),
			"StatusCode", ctx.Response.StatusCode(),
			"LatencyMs", float64(latency.Milliseconds()),
			"RemoteIP", ctx.RemoteIP().String(),
			"UserAgent", string(ctx.UserAgent()),
		)
		l.Information("http request handled")
	}
}

func requestIDMiddleware(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		rid := string(ctx.Request.Header.Peek("X-Request-ID"))
		if rid == "" {
			rid = uuid.NewString()
		}
		ctx.SetUserValue("request_id", rid)
		ctx.Response.Header.Set("X-Request-ID", rid)
		next(ctx)
	}
}

func recoverMiddleware(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		defer func() {
			if r := recover(); r != nil {
				logger.LogError("panic recovered: {panic}", "panic", r)
				ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			}
		}()
		next(ctx)
	}
}
