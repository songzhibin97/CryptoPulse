package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog"
	"github.com/songzhibin97/CryptoPulse/analyzer"
	"github.com/songzhibin97/CryptoPulse/config"
	"github.com/songzhibin97/CryptoPulse/report"
)

type analyzerRegistry struct {
	analyzers map[string]*analyzer.MarketAnalyzer
	mu        sync.RWMutex
}

func newAnalyzerRegistry() *analyzerRegistry {
	return &analyzerRegistry{
		analyzers: make(map[string]*analyzer.MarketAnalyzer),
	}
}

func SetupRoutes(r *gin.Engine, cfg config.Config, logger zerolog.Logger, reportMgr *report.ReportManager) {
	registry := newAnalyzerRegistry()

	r.GET("/api/pairs", func(c *gin.Context) {
		start := time.Now()
		query := c.Query("query")
		client := resty.New()
		if cfg.ProxyURL != "" {
			proxy, err := url.Parse(cfg.ProxyURL)
			if err == nil {
				client.SetTransport(&http.Transport{Proxy: http.ProxyURL(proxy)})
			} else {
				logger.Error().Err(err).Str("proxy_url", cfg.ProxyURL).Msg("Invalid proxy URL")
			}
		}
		client.SetTimeout(10 * time.Second).SetRetryCount(3).SetRetryWaitTime(2 * time.Second)
		resp, err := client.R().Get("https://api1.binance.com/api/v3/exchangeInfo")
		if err != nil {
			logger.Error().Err(err).Msg("Fetch exchange info error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		var data map[string]interface{}
		json.Unmarshal(resp.Body(), &data)
		symbols := data["symbols"].([]interface{})
		var pairs []string
		for _, s := range symbols {
			symbol := s.(map[string]interface{})["symbol"].(string)
			if query == "" || strings.Contains(strings.ToLower(symbol), strings.ToLower(query)) {
				pairs = append(pairs, symbol)
			}
		}
		logger.Info().Dur("duration_ms", time.Since(start)).Msg("Processed /api/pairs")
		c.JSON(http.StatusOK, pairs)
	})

	r.POST("/api/monitor", func(c *gin.Context) {
		start := time.Now()
		var req struct {
			Symbol    string   `json:"symbol"`
			Intervals []string `json:"intervals"`
			Cycle     string   `json:"cycle"`
		}
		if err := c.BindJSON(&req); err != nil {
			logger.Error().Err(err).Msg("Invalid request body")
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		logger.Debug().Interface("request_body", req).Msg("Received /api/monitor request")
		if req.Symbol == "" {
			logger.Warn().Msg("Symbol is required")
			c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
			return
		}
		if len(req.Intervals) == 0 {
			logger.Warn().Msg("Intervals are required")
			c.JSON(http.StatusBadRequest, gin.H{"error": "intervals are required"})
			return
		}
		validIntervals := map[string]bool{"1m": true, "5m": true, "15m": true, "1h": true, "4h": true, "1d": true}
		for _, interval := range req.Intervals {
			if !validIntervals[interval] {
				logger.Warn().Str("interval", interval).Msg("Invalid interval")
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid interval: %s", interval)})
				return
			}
		}
		if req.Cycle == "" {
			req.Cycle = "30s"
		}
		duration, err := time.ParseDuration(req.Cycle)
		if err != nil || duration < 10*time.Second {
			logger.Warn().Str("cycle", req.Cycle).Msg("Invalid or too short cycle")
			c.JSON(http.StatusBadRequest, gin.H{"error": "cycle must be at least 10s"})
			return
		}

		ma := analyzer.NewMarketAnalyzer(req.Symbol, req.Intervals, cfg.AIEndpoint, cfg.ExtEndpoint, cfg.ProxyURL, cfg.WSProxyURL, logger, reportMgr)
		if err := ma.ConnectWebSocket(); err != nil {
			logger.Error().Err(err).Msg("WebSocket connection error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		monitorID := uuid.New().String()
		registry.mu.Lock()
		registry.analyzers[monitorID] = ma
		registry.mu.Unlock()

		go ma.RunMonitor(req.Cycle)

		chartData := ma.GenerateChartData()
		logger.Info().
			Str("symbol", req.Symbol).
			Strs("intervals", req.Intervals).
			Str("cycle", req.Cycle).
			Str("monitor_id", monitorID).
			Dur("duration_ms", time.Since(start)).
			Msg("Processed /api/monitor")

		c.JSON(http.StatusOK, gin.H{
			"chart_data": chartData,
			"monitor_id": monitorID,
		})
	})

	r.POST("/api/monitor/stop", func(c *gin.Context) {
		start := time.Now()
		var req struct {
			MonitorID string `json:"monitor_id"`
		}
		if err := c.BindJSON(&req); err != nil {
			logger.Error().Err(err).Msg("Invalid request body")
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		registry.mu.Lock()
		ma, ok := registry.analyzers[req.MonitorID]
		if ok {
			ma.Stop()
			delete(registry.analyzers, req.MonitorID)
		}
		registry.mu.Unlock()
		if !ok {
			logger.Warn().Str("monitor_id", req.MonitorID).Msg("Monitor not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "monitor not found"})
			return
		}
		logger.Info().Dur("duration_ms", time.Since(start)).Msg("Processed /api/monitor/stop")
		c.JSON(http.StatusOK, gin.H{"message": "Monitoring stopped"})
	})

	r.GET("/api/chart", func(c *gin.Context) {
		start := time.Now()
		symbol := c.Query("symbol")
		if symbol == "" {
			logger.Warn().Msg("Symbol is required")
			c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
			return
		}

		ma := analyzer.NewMarketAnalyzer(symbol, []string{"15m"}, cfg.AIEndpoint, cfg.ExtEndpoint, cfg.ProxyURL, cfg.WSProxyURL, logger, reportMgr)
		if err := ma.FetchRealtimeData(); err != nil {
			logger.Error().Err(err).Str("symbol", symbol).Msg("Failed to fetch real-time data")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch market data"})
			return
		}

		chartData := ma.GenerateChartData()
		logger.Info().
			Str("symbol", symbol).
			Dur("duration_ms", time.Since(start)).
			Msg("Processed /api/chart")
		c.JSON(http.StatusOK, gin.H{"chart_data": chartData})
	})

	r.GET("/api/prompt", func(c *gin.Context) {
		start := time.Now()
		symbol := c.Query("symbol")
		if symbol == "" {
			logger.Warn().Msg("Symbol is required")
			c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
			return
		}

		ma := analyzer.NewMarketAnalyzer(symbol, []string{"15m"}, cfg.AIEndpoint, cfg.ExtEndpoint, cfg.ProxyURL, cfg.WSProxyURL, logger, reportMgr)
		if err := ma.FetchRealtimeData(); err != nil {
			logger.Error().Err(err).Str("symbol", symbol).Msg("Failed to fetch real-time data")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch market data"})
			return
		}

		prompt := ma.GeneratePrompt()
		logger.Info().
			Str("symbol", symbol).
			Dur("duration_ms", time.Since(start)).
			Msg("Processed /api/prompt")
		c.JSON(http.StatusOK, gin.H{
			"symbol": symbol,
			"prompt": prompt,
		})
	})

	r.POST("/api/submit_response", func(c *gin.Context) {
		start := time.Now()
		var req struct {
			AnalysisID   string `json:"analysis_id"`
			ResponseJSON string `json:"response_json"`
		}
		if err := c.BindJSON(&req); err != nil {
			logger.Error().Err(err).Msg("Invalid request body")
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ma := analyzer.NewMarketAnalyzer("", []string{}, cfg.AIEndpoint, cfg.ExtEndpoint, cfg.ProxyURL, cfg.WSProxyURL, logger, reportMgr)
		resp, err := ma.SubmitManualResponse(req.AnalysisID, req.ResponseJSON)
		if err != nil {
			logger.Error().Err(err).Msg("Submit manual response error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		logger.Info().Dur("duration_ms", time.Since(start)).Msg("Processed /api/submit_response")
		c.JSON(http.StatusOK, gin.H{"report_id": resp.ReportID})
	})

	r.GET("/api/report", func(c *gin.Context) {
		start := time.Now()
		reportID := c.Query("report_id")
		filePath, ok := reportMgr.GetReportPath(reportID)
		if !ok {
			logger.Warn().Str("report_id", reportID).Msg("Report not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
			return
		}
		logger.Info().Dur("duration_ms", time.Since(start)).Msg("Processed /api/report")
		c.FileAttachment(filePath, filepath.Base(filePath))
	})
}
