package models

// Kline represents a candlestick data point
type Kline struct {
	OpenTime  int64  `json:"open_time"`
	Open      string `json:"open"`
	High      string `json:"high"`
	Low       string `json:"low"`
	Close     string `json:"close"`
	Volume    string `json:"volume"`
	CloseTime int64  `json:"close_time"`
}

// OrderBook represents the order book
type OrderBook struct {
	LastUpdateID int64              `json:"last_update_id"`
	Bids         map[string]float64 `json:"bids"`
	Asks         map[string]float64 `json:"asks"`
}
