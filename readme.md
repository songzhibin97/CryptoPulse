# CryptoPulse

CryptoPulse 是一个基于 Web 的加密货币市场监控应用，从 Binance 获取实时市场数据，生成 AI 分析提示，允许用户提交手动 AI 响应并生成可下载的报告。目前应用支持 **Monitor** 模式，定期获取并展示市场数据（K 线、订单簿、交易数据）并动态更新图表。

## 功能

* **实时市场监控**：定期获取所选交易对的 K 线数据、订单簿和交易数据。
* **动态图表展示**：使用 Plotly.js 显示 K 线和成交量图表，根据用户定义的周期（如每 30 秒）更新。
* **AI 提示生成**：基于市场数据生成 AI 分析提示。
* **报告生成**：保存并下载分析报告为 JSON 文件。
* **交易对搜索**：通过 Binance 的交易所信息 API 搜索并选择交易对（如 `BTCUSDT`）。
* **可配置周期和间隔**：支持多种 K 线间隔（如 1m、5m、1h）和用户定义的监控周期（如 30s、5m）。

## 前置条件

* **Go**：1.18 或更高版本（用于后端）。
* **Node.js**：无需安装，前端使用 CDN 托管的 Plotly.js。
* **Binance API 访问**：无需 API 密钥，使用公开的 Binance API 接口。
* **Git**：用于克隆仓库。

## 安装

1. **克隆仓库**：

```bash
git clone https://github.com/yourusername/cryptopulse.git
cd cryptopulse
```

2. **安装依赖**： 确保已安装 Go，然后获取 Go 模块依赖：

```bash
go mod tidy
```

3. **配置应用**： 在项目根目录创建或修改 `config.yaml` 文件：

```yaml
port: 8080
ai_endpoint: "manual"
ext_endpoint: ""
proxy_url: ""
ws_proxy_url: ""
```

   * `port`：HTTP 服务器端口（默认 8080）。
   * `ai_endpoint`：AI 服务端点，当前仅支持 `"manual"`（手动模式）。
   * `ext_endpoint`：外部数据端点（可选，当前未使用）。
   * `proxy_url`：HTTP 代理地址（可选）。
   * `ws_proxy_url`：WebSocket 代理地址（可选）。

4. **运行应用**：

```bash
go run main.go
```

应用将在 `http://localhost:8080` 启动。

## 使用方法

1. **访问页面**： 打开浏览器，访问 `http://localhost:8080`。
2. **选择交易对**：
   * 在"Search Pair"输入框输入交易对（如 `BTCUSDT`）。
   * 从下拉列表选择一个交易对。
3. **配置监控**：
   * 选择 K 线间隔（如 `1m`、`5m`），可多选。
   * 设置监控周期（如 `30s`、`5m`、`1h`）。
4. **启动监控**：
   * 点击"Start Monitor"按钮。
   * 查看动态更新的 K 线和成交量图表。
   * 检查"AI Prompt"文本框中的分析提示。
5. **停止监控**：
   * 点击"Stop Monitor"按钮，停止数据更新并重置状态。


## 开发注意事项

* **Binance API 限频**：
   * Binance 公开 API 有请求限制（1200 次/分钟）。短周期监控（如 `10s`）可能触发 429 错误。
   * 建议设置合理的监控周期（如 `30s` 或更长）或实现数据缓存。
* **前端调试**：
   * 打开浏览器开发者工具（F12），检查 `Console` 日志以排查 JavaScript 错误。
   * 确保 Plotly.js CDN（`https://cdn.plot.ly/plotly-latest.min.js`）可访问。
* **后端日志**：
   * 使用 `zerolog` 输出日志，检查 `/api/monitor` 和 `/api/chart` 的 `duration_ms` 是否正常（应低于 10 秒）。
   * 如果遇到超时或连接错误，检查 `proxy_url` 配置。
* **扩展功能**：
   * 可添加自动 AI 分析（替换 `ai_endpoint: "manual"`）。
   * 可扩展支持更多交易所或数据源。

## 问题排查

* **界面无法加载**：
   * 确保 `static/index.html` 和 `static/script.js` 文件存在。
   * 检查浏览器控制台是否有 JavaScript 错误。
* **图表不更新**：
   * 确认 `cycle` 格式正确（如 `30s`）。
   * 检查后端日志，确认 `/api/chart` 请求是否成功。
* **API 请求失败**：
   * 验证网络连接和 Binance API 可用性。
   * 检查 `config.yaml` 中的代理设置。

## 贡献

欢迎提交 Issue 或 Pull Request！请遵循以下步骤：
1. Fork 仓库。
2. 创建特性分支（`git checkout -b feature/xxx`）。
3. 提交更改（`git commit -m 'Add xxx feature'`）。
4. 推送到远程（`git push origin feature/xxx`）。
5. 提交 Pull Request。

## 许可证

MIT License