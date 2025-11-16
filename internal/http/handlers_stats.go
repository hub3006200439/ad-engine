package http

import (
	"context"
	"encoding/json"
	"time"

	"ad-engine/internal/logger"

	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
)

func (s *Server) handleStatsOverview(ctx *fasthttp.RequestCtx) {
	q := ctx.QueryArgs()

	fromStr := string(q.Peek("from"))
	toStr := string(q.Peek("to"))
	campaignIDStr := string(q.Peek("campaign_id"))

	if fromStr == "" || toStr == "" {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		logger.LogWarning("bad stats from param: {val}", "val", fromStr)
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		logger.LogWarning("bad stats to param: {val}", "val", toStr)
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	var campaignID *uuid.UUID
	if campaignIDStr != "" {
		id, err := uuid.Parse(campaignIDStr)
		if err != nil {
			logger.LogWarning("bad campaign_id uuid: {val}", "val", campaignIDStr)
			ctx.SetStatusCode(fasthttp.StatusBadRequest)
			return
		}
		campaignID = &id
	}

	res, err := s.statsSvc.Overview(context.Background(), from, to, campaignID)
	if err != nil {
		logger.LogError("failed stats overview: {err}", "err", err)
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(res)
	if err != nil {
		logger.LogError("failed marshal stats overview: {err}", "err", err)
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_, _ = ctx.Write(data)
}
