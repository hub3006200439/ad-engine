package http

import (
	"ad-engine/internal/config"
	"ad-engine/internal/usecase"
	"time"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
)

type Server struct {
	adSvc    *usecase.AdService
	statsSvc *usecase.StatsService
}

func NewServer(ad *usecase.AdService, stats *usecase.StatsService) *Server {
	return &Server{adSvc: ad, statsSvc: stats}
}

func (s *Server) Handler() fasthttp.RequestHandler {
	r := router.New()

	r.POST("/ad/request", s.handleAdRequest)
	r.GET("/ad/click/{token}", s.handleAdClick)
	r.GET("/ad/view/{token}", s.handleAdView)
	r.GET("/stats/overview", s.handleStatsOverview)

	return withMiddlewares(r.Handler)
}

// BuildHTTPServer собирает fasthttp.Server с продуктовой конфигурацией.
func BuildHTTPServer(cfg config.ServerConfig, apiServer *Server) *fasthttp.Server {
	s := &fasthttp.Server{
		Handler:                      apiServer.Handler(),
		Name:                         cfg.Name,
		ReadTimeout:                  time.Duration(cfg.Params.ReadTimeout) * time.Second,
		WriteTimeout:                 time.Duration(cfg.Params.WriteTimeout) * time.Second,
		IdleTimeout:                  time.Duration(cfg.Params.IdleTimeout) * time.Second,
		ReadBufferSize:               cfg.Params.ReadBufferSize,
		WriteBufferSize:              cfg.Params.WriteBufferSize,
		MaxRequestBodySize:           cfg.Params.MaxRequestBodySize,
		DisablePreParseMultipartForm: cfg.Params.DisablePreParseMultipart,
		NoDefaultServerHeader:        cfg.Params.NoDefaultServerHeader,
		NoDefaultDate:                cfg.Params.NoDefaultDate,
		NoDefaultContentType:         cfg.Params.NoDefaultContentType,
		CloseOnShutdown:              cfg.Params.CloseOnShutdown,
	}

	// Если keep_alive = false в конфиге — выключаем keepalive в fasthttp
	if !cfg.Params.KeepAlive {
		s.DisableKeepalive = true
	}

	return s
}
