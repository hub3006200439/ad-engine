package http

import (
	"context"
	"encoding/json"

	"ad-engine/internal/logger"
	"ad-engine/internal/usecase"

	"github.com/valyala/fasthttp"
)

func (s *Server) handleAdRequest(ctx *fasthttp.RequestCtx) {
	var req usecase.AdRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		logger.LogWarning("bad ad request body: {err}", err)
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	resp, err := s.adSvc.RequestAd(context.Background(), req)
	if err != nil {
		logger.LogError("failed RequestAd: {err}", err)
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}
	if resp == nil {
		logger.LogDebug("no-fill for user={user_id}, session={session_id}", req.UserID, req.SessionID)
		ctx.SetStatusCode(fasthttp.StatusNoContent)
		return
	}

	data, err := json.Marshal(resp)
	if err != nil {
		logger.LogError("failed marshal ad response: {err}", err)
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_, _ = ctx.Write(data)
}

func (s *Server) handleAdClick(ctx *fasthttp.RequestCtx) {
	rawToken := ctx.UserValue("token")
	token, _ := rawToken.(string)
	if token == "" {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	landing, err := s.adSvc.TrackClick(context.Background(), token)
	if err != nil {
		logger.LogError("failed TrackClick: {err}", err)
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}
	if landing == "" {
		logger.LogWarning("click with unknown token={token}", token)
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		return
	}

	ctx.Response.Header.Set("Location", landing)
	ctx.SetStatusCode(fasthttp.StatusFound)
}

// /ad/view/{token} — фиксация просмотра (view)
func (s *Server) handleAdView(ctx *fasthttp.RequestCtx) {
	rawToken := ctx.UserValue("token")
	token, _ := rawToken.(string)
	if token == "" {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	if err := s.adSvc.TrackView(context.Background(), token); err != nil {
		logger.LogError("failed TrackView: {err}", err)
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetStatusCode(fasthttp.StatusNoContent)
}
