package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/songzhibin97/CryptoPulse/models"
	"github.com/songzhibin97/CryptoPulse/report"
)

// Global storage for pending prompts
var globalPendingPrompts = make(map[string]string)
var globalPromptsMu sync.RWMutex

// MarketAnalyzer handles market data analysis
type MarketAnalyzer struct {
	httpClient      *resty.Client
	wsConn          *websocket.Conn
	aiEndpoint      string
	extEndpoint     string
	proxyURL        string
	wsProxyURL      string
	symbol          string
	intervals       []string
	orderBook       models.OrderBook
	klines          map[string][]models.Kline
	trades          []map[string]interface{}
	sentiment       string
	ctx             context.Context
	cancel          context.CancelFunc
	logger          zerolog.Logger
	reportMgr       *report.ReportManager
	mu              sync.RWMutex
	latestChartData map[string]interface{}
}

// AnalysisResponse holds the response from AI analysis
type AnalysisResponse struct {
	AnalysisID string
	ReportID   string
}

// NewMarketAnalyzer creates a new MarketAnalyzer instance
func NewMarketAnalyzer(symbol string, intervals []string, aiEndpoint, extEndpoint, proxyURL, wsProxyURL string, logger zerolog.Logger, reportMgr *report.ReportManager) *MarketAnalyzer {
	logger.Debug().Str("proxy_url", proxyURL).Str("ws_proxy_url", wsProxyURL).Msg("Configuring proxies")
	var transport *http.Transport
	if proxyURL != "" {
		proxy, err := url.Parse(proxyURL)
		if err != nil {
			logger.Error().Err(err).Str("proxy_url", proxyURL).Msg("Invalid HTTP proxy URL")
			transport = &http.Transport{}
		} else {
			logger.Info().Str("proxy_url", proxyURL).Msg("Using HTTP proxy")
			transport = &http.Transport{Proxy: http.ProxyURL(proxy)}
		}
	} else {
		transport = &http.Transport{}
	}
	httpClient := resty.New().
		SetTransport(transport).
		SetTimeout(10 * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(2 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	return &MarketAnalyzer{
		httpClient:  httpClient,
		aiEndpoint:  aiEndpoint,
		extEndpoint: extEndpoint,
		proxyURL:    proxyURL,
		wsProxyURL:  wsProxyURL,
		symbol:      symbol,
		intervals:   intervals,
		orderBook: models.OrderBook{
			Bids: make(map[string]float64),
			Asks: make(map[string]float64),
		},
		klines:          make(map[string][]models.Kline),
		trades:          make([]map[string]interface{}, 0),
		ctx:             ctx,
		cancel:          cancel,
		logger:          logger,
		reportMgr:       reportMgr,
		latestChartData: make(map[string]interface{}),
	}
}

// ConnectWebSocket establishes a WebSocket connection to Binance
func (ma *MarketAnalyzer) ConnectWebSocket() error {
	ma.logger.Info().Msg("Skipping WebSocket, using HTTP fallback for debugging")
	if err := ma.FetchRealtimeData(); err != nil {
		ma.logger.Error().Err(err).Msg("Failed to fetch real-time data via HTTP")
		return err
	}
	return nil
}

// GenerateChartData generates chart data for frontend
func (ma *MarketAnalyzer) GenerateChartData() map[string]interface{} {
	ma.mu.Lock()
	defer ma.mu.Unlock()

	ma.logger.Debug().
		Int("kline_intervals", len(ma.klines)).
		Int("bids_count", len(ma.orderBook.Bids)).
		Int("trades_count", len(ma.trades)).
		Msg("Generating chart data")

	limitedKlines := make(map[string][]models.Kline)
	for interval, klines := range ma.klines {
		filteredKlines := klines
		if len(klines) > 20 { // 限制到 20 条
			filteredKlines = klines[len(klines)-20:]
		}
		limitedKlines[interval] = filteredKlines
	}

	klineCount := 0
	for interval, klines := range limitedKlines {
		klineCount += len(klines)
		ma.logger.Debug().Str("interval", interval).Int("count", len(klines)).Msg("Kline data for interval")
	}

	chartData := map[string]interface{}{
		"kline": limitedKlines,
		"depth": map[string]interface{}{
			"bids": ma.orderBook.Bids,
			"asks": ma.orderBook.Asks,
		},
	}
	ma.logger.Info().
		Int("kline_count", klineCount).
		Int("bids_count", len(ma.orderBook.Bids)).
		Msg("Generated chart data")
	return chartData
}

// GetLatestChartData retrieves the latest chart data
func (ma *MarketAnalyzer) GetLatestChartData() map[string]interface{} {
	ma.mu.RLock()
	defer ma.mu.RUnlock()
	return ma.latestChartData
}

// FetchRealtimeData fetches real-time data via HTTP API
func (ma *MarketAnalyzer) FetchRealtimeData() error {
	ma.logger.Info().Msg("Fetching real-time data via HTTP")
	for _, interval := range ma.intervals {
		url := fmt.Sprintf("https://api1.binance.com/api/v3/klines?symbol=%s&interval=%s&limit=100", ma.symbol, interval)
		resp, err := ma.httpClient.R().Get(url)
		if err != nil {
			ma.logger.Error().Err(err).Str("url", url).Msg("Fetch klines error")
			return fmt.Errorf("fetch klines failed: %w", err)
		}
		ma.logger.Debug().Str("url", url).Int("status", resp.StatusCode()).Msg("Fetched klines via HTTP")
		var klines [][]interface{}
		if err := json.Unmarshal(resp.Body(), &klines); err != nil {
			ma.logger.Error().Err(err).Msg("Unmarshal klines error")
			return fmt.Errorf("unmarshal klines failed: %w", err)
		}
		ma.mu.Lock()
		ma.klines[interval] = make([]models.Kline, 0, len(klines))
		for _, k := range klines {
			ma.klines[interval] = append(ma.klines[interval], models.Kline{
				OpenTime:  int64(k[0].(float64)),
				Open:      k[1].(string),
				High:      k[2].(string),
				Low:       k[3].(string),
				Close:     k[4].(string),
				Volume:    k[5].(string),
				CloseTime: int64(k[6].(float64)),
			})
		}
		ma.mu.Unlock()
		ma.logger.Info().Int("kline_count", len(klines)).Str("interval", interval).Msg("Fetched klines")
	}

	url := fmt.Sprintf("https://api1.binance.com/api/v3/depth?symbol=%s&limit=1000", ma.symbol)
	resp, err := ma.httpClient.R().Get(url)
	if err != nil {
		ma.logger.Error().Err(err).Str("url", url).Msg("Fetch depth error")
		return fmt.Errorf("fetch depth failed: %w", err)
	}
	ma.logger.Debug().Str("url", url).Int("status", resp.StatusCode()).Msg("Fetched depth via HTTP")
	var depth struct {
		LastUpdateID int64       `json:"lastUpdateId"`
		Bids         [][2]string `json:"bids"`
		Asks         [][2]string `json:"asks"`
	}
	if err := json.Unmarshal(resp.Body(), &depth); err != nil {
		ma.logger.Error().Err(err).Msg("Unmarshal depth error")
		return fmt.Errorf("unmarshal depth failed: %w", err)
	}
	ma.mu.Lock()
	ma.orderBook.LastUpdateID = depth.LastUpdateID
	ma.orderBook.Bids = make(map[string]float64)
	ma.orderBook.Asks = make(map[string]float64)
	for _, bid := range depth.Bids {
		price, qty := bid[0], bid[1]
		ma.orderBook.Bids[price], _ = strconv.ParseFloat(qty, 64)
	}
	for _, ask := range depth.Asks {
		price, qty := ask[0], ask[1]
		ma.orderBook.Asks[price], _ = strconv.ParseFloat(qty, 64)
	}
	ma.mu.Unlock()
	ma.logger.Info().Int("bids_count", len(depth.Bids)).Int("asks_count", len(depth.Asks)).Msg("Fetched order book")

	url = fmt.Sprintf("https://api1.binance.com/api/v3/aggTrades?symbol=%s&limit=500", ma.symbol)
	resp, err = ma.httpClient.R().Get(url)
	if err != nil {
		ma.logger.Error().Err(err).Str("url", url).Msg("Fetch trades error")
		return fmt.Errorf("fetch trades failed: %w", err)
	}
	ma.logger.Debug().Str("url", url).Int("status", resp.StatusCode()).Msg("Fetched trades via HTTP")
	var trades []map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &trades); err != nil {
		ma.logger.Error().Err(err).Msg("Unmarshal trades error")
		return fmt.Errorf("unmarshal trades failed: %w", err)
	}
	ma.mu.Lock()
	ma.trades = trades
	ma.sentiment = "neutral"
	ma.mu.Unlock()
	ma.logger.Info().Int("trades_count", len(trades)).Msg("Fetched trades")

	return nil
}

// CallAIAnalysis calls AI for analysis
func (ma *MarketAnalyzer) CallAIAnalysis() (AnalysisResponse, error) {
	ma.mu.Lock()
	analysisID := uuid.New().String()
	prompt := ma.GeneratePrompt()
	globalPromptsMu.Lock()
	globalPendingPrompts[analysisID] = prompt
	ma.logger.Info().Str("analysis_id", analysisID).Msg("Stored pending prompt")
	globalPromptsMu.Unlock()
	ma.mu.Unlock()

	if ma.aiEndpoint == "manual" {
		ma.logger.Info().Str("analysis_id", analysisID).Msg("Manual AI mode")
		return AnalysisResponse{AnalysisID: analysisID}, nil
	}
	return AnalysisResponse{}, fmt.Errorf("unsupported AI endpoint: %s", ma.aiEndpoint)
}

// GeneratePrompt generates the AI analysis prompt
func (ma *MarketAnalyzer) GeneratePrompt() string {
	analysisType := "monitor"
	cycle := "continuous"
	startTs, endTs := int64(0), int64(0)

	promptStr := ma.generatePrompt(analysisType, cycle, startTs, endTs)
	data := map[string]interface{}{
		"prompt": promptStr,
	}
	promptBytes, err := json.Marshal(data)
	if err != nil {
		ma.logger.Error().Err(err).Msg("Failed to marshal prompt data")
		return promptStr
	}
	return string(promptBytes)
}

// GetPendingPrompt retrieves a pending prompt
func (ma *MarketAnalyzer) GetPendingPrompt(analysisID string) (string, bool) {
	globalPromptsMu.RLock()
	defer globalPromptsMu.RUnlock()
	prompt, ok := globalPendingPrompts[analysisID]
	return prompt, ok
}

// SubmitManualResponse submits a manual AI response
func (ma *MarketAnalyzer) SubmitManualResponse(analysisID, responseJSON string) (AnalysisResponse, error) {
	globalPromptsMu.Lock()
	delete(globalPendingPrompts, analysisID)
	globalPromptsMu.Unlock()

	reportID := uuid.New().String()
	if err := ma.reportMgr.SaveReport(reportID, responseJSON); err != nil {
		return AnalysisResponse{}, err
	}
	return AnalysisResponse{ReportID: reportID}, nil
}

// generatePrompt generates the AI prompt with real-time data
func (ma *MarketAnalyzer) generatePrompt(analysisType, cycle string, startTime, endTime int64) string {
	ma.mu.Lock()
	defer ma.mu.Unlock()

	limitedKlines := make(map[string][]models.Kline)
	for interval, klines := range ma.klines {
		filteredKlines := klines
		if len(klines) > 10 {
			filteredKlines = klines[len(klines)-10:]
		}
		limitedKlines[interval] = filteredKlines
	}

	limitedTrades := ma.trades
	if len(ma.trades) > 50 {
		limitedTrades = ma.trades[len(ma.trades)-50:]
	}

	limitedBids := make(map[string]float64)
	limitedAsks := make(map[string]float64)
	bidCount, askCount := 0, 0
	for price, qty := range ma.orderBook.Bids {
		if bidCount < 50 {
			limitedBids[price] = qty
			bidCount++
		}
	}
	for price, qty := range ma.orderBook.Asks {
		if askCount < 50 {
			limitedAsks[price] = qty
			askCount++
		}
	}

	klinesJSON, _ := json.Marshal(limitedKlines)
	orderBookJSON, _ := json.Marshal(map[string]interface{}{
		"bids": limitedBids,
		"asks": limitedAsks,
	})
	tradesJSON, _ := json.Marshal(limitedTrades)

	return fmt.Sprintf(`## 数字资产市场动态分析报告

**输入数据**:
- 交易对: %s
- K线数据: 周期包括 %v
  - 数据: %s
- 订单簿深度: %s
- 成交数据: %s
- 外部情绪: %s
- 分析类型: %s
- 监控周期: %s
## 分析任务	
1. 资金流动态势
- 主动买卖方向识别
- 大额交易行为追踪
- 净流入/流出趋势研判

2. 技术形态研判
- 多维指标协同研判
- 关键价格区间定位
- 趋势拐点预警机制

3. 订单簿深度解析
- 买卖盘力量对比
- 关键价格区支撑/阻力

4. 市场情绪指标
- 成交量分布特征
- 波动率变化监测

5. 风险预警系统
- 价格异动实时预警
- 市场操纵识别模型
`,
		ma.symbol, ma.intervals, string(klinesJSON),
		string(orderBookJSON), string(tradesJSON), ma.sentiment, analysisType, cycle)
}

// RunMonitor runs the monitoring loop
func (ma *MarketAnalyzer) RunMonitor(cycle string) error {
	ma.logger.Info().Str("cycle", cycle).Msg("Starting monitor")
	duration, err := time.ParseDuration(cycle)
	if err != nil {
		ma.logger.Error().Err(err).Str("cycle", cycle).Msg("Invalid cycle duration")
		return err
	}

	ticker := time.NewTicker(duration)
	defer ticker.Stop()

	for {
		select {
		case <-ma.ctx.Done():
			ma.logger.Info().Msg("Monitor stopped")
			return nil
		case <-ticker.C:
			ma.logger.Debug().Msg("Running monitor cycle")
			if err := ma.FetchRealtimeData(); err != nil {
				ma.logger.Error().Err(err).Msg("Monitor fetch data failed")
				continue
			}
			chartData := ma.GenerateChartData()
			ma.mu.Lock()
			ma.latestChartData = chartData
			ma.mu.Unlock()
			resp, err := ma.CallAIAnalysis()
			if err != nil {
				ma.logger.Error().Err(err).Msg("Monitor AI analysis failed")
				continue
			}
			ma.logger.Info().Str("analysis_id", resp.AnalysisID).Msg("Monitor cycle completed")
		}
	}
}

// Stop stops the MarketAnalyzer
func (ma *MarketAnalyzer) Stop() {
	ma.cancel()
}
