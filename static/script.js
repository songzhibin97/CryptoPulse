/**
 * CryptoPulse Frontend Script
 * Manages UI interactions, API calls, and chart updates for market monitoring.
 */

let selectedPair = '';
let currentMonitorID = '';
let isMonitoring = false;
let chartUpdateInterval = null;
let charts = {};

// Initialize application state
function initializeState() {
    selectedPair = '';
    currentMonitorID = '';
    isMonitoring = false;
    if (chartUpdateInterval) {
        clearInterval(chartUpdateInterval);
        chartUpdateInterval = null;
        console.log('Cleared chart update interval');
    }
    const monitorStatus = document.getElementById('monitor-status');
    if (monitorStatus) monitorStatus.style.display = 'none';
    const chartStatus = document.getElementById('chart-status');
    if (chartStatus) chartStatus.textContent = 'Inactive';
    const monitorId = document.getElementById('monitor-id');
    if (monitorId) monitorId.textContent = 'None';
    const stopMonitorBtn = document.getElementById('stop-monitor');
    if (stopMonitorBtn) stopMonitorBtn.disabled = true;
    const selectedPairSpan = document.getElementById('selected-pair');
    if (selectedPairSpan) selectedPairSpan.textContent = 'None';
    const chartsContainer = document.getElementById('charts');
    if (chartsContainer) chartsContainer.innerHTML = '';
    charts = {};
    console.log('Application state initialized');
}

// Search trading pairs
async function searchPairs() {
    const query = document.getElementById('pair-search')?.value.trim() || '';
    const pairList = document.getElementById('pair-list');
    if (!pairList) {
        console.error('Pair list element not found');
        alert('UI error: Pair list not found');
        return;
    }
    pairList.innerHTML = '';
    pairList.style.display = 'none';
    if (query.length < 2) return; // 至少 2 个字符才搜索
    try {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 5000);
        const response = await fetch(`/api/pairs?query=${encodeURIComponent(query)}`, {
            signal: controller.signal
        });
        clearTimeout(timeoutId);
        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`API error: ${response.status} ${errorText}`);
        }
        const pairs = await response.json();
        pairs.forEach(pair => {
            const li = document.createElement('li');
            li.textContent = pair;
            li.onclick = () => {
                selectedPair = pair;
                document.getElementById('selected-pair').textContent = pair;
                pairList.style.display = 'none';
                console.log('Selected pair:', pair);
            };
            pairList.appendChild(li);
        });
        pairList.style.display = pairs.length > 0 ? 'block' : 'none';
    } catch (error) {
        console.error('Search pairs error:', error);
        alert(`Failed to fetch pairs: ${error.message}`);
    }
}

// Start monitoring
async function startMonitor() {
    if (!selectedPair) {
        alert('Please select a trading pair!');
        return;
    }
    const intervals = Array.from(document.getElementById('intervals')?.selectedOptions || []).map(o => o.value);
    if (intervals.length === 0) {
        alert('Please select at least one interval!');
        return;
    }
    const cycle = document.getElementById('cycle')?.value.trim();
    if (!cycle || !/^\d+[smh]$/.test(cycle)) {
        alert('Invalid cycle format! Use format like "30s", "5m", "1h".');
        return;
    }

    const payload = { symbol: selectedPair, intervals, cycle };
    console.log('Starting monitor with payload:', payload);

    document.getElementById('loading').style.display = 'inline';
    try {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 30000);
        const response = await fetch('/api/monitor', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
            signal: controller.signal
        });
        clearTimeout(timeoutId);
        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`API error: ${response.status} ${errorText}`);
        }
        const result = await response.json();
        console.log('Monitor started:', result);

        currentMonitorID = result.monitor_id || '';
        isMonitoring = true;
        document.getElementById('monitor-id').textContent = currentMonitorID;
        document.getElementById('monitor-status').style.display = 'block';
        document.getElementById('chart-status').textContent = 'Monitoring active, updating charts...';
        document.getElementById('stop-monitor').disabled = false;
        if (result.chart_data) {
            plotCharts(result.chart_data);
        }
        subscribeChartUpdates();
        await fetchPrompt();
    } catch (error) {
        console.error('Start monitor error:', error);
        alert(`Failed to start monitor: ${error.message}`);
    } finally {
        document.getElementById('loading').style.display = 'none';
    }
}

// Stop monitor
async function stopMonitor() {
    if (!currentMonitorID) {
        alert('No active monitor to stop!');
        return;
    }
    document.getElementById('loading').style.display = 'inline';
    try {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 5000);
        const response = await fetch('/api/monitor/stop', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ monitor_id: currentMonitorID }),
            signal: controller.signal
        });
        clearTimeout(timeoutId);
        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`API error: ${response.status} ${errorText}`);
        }
        const result = await response.json();
        console.log('Monitor stopped:', result);
        initializeState();
    } catch (error) {
        console.error('Stop monitor error:', error);
        alert(`Failed to stop monitor: ${error.message}`);
    } finally {
        document.getElementById('loading').style.display = 'none';
    }
}

// Fetch AI prompt (overridden in index.html)
async function fetchPrompt() {
    console.log('fetchPrompt is overridden in index.html');
}

// Submit AI response
async function submitResponse() {
    const promptResponse = document.getElementById('prompt-response')?.value.trim();
    if (!promptResponse) {
        alert('Please provide a response!');
        return;
    }
    if (!currentMonitorID) {
        alert('No active monitor! Please start a monitor first.');
        return;
    }
    document.getElementById('loading').style.display = 'inline';
    try {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 5000);
        const response = await fetch('/api/submit_response', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                analysis_id: currentMonitorID,
                response_json: promptResponse
            }),
            signal: controller.signal
        });
        clearTimeout(timeoutId);
        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`API error: ${response.status} ${errorText}`);
        }
        const result = await response.json();
        console.log('Response submitted:', result);
        if (result.report_id) {
            await downloadReport(result.report_id);
            document.getElementById('prompt-response').value = '';
        }
    } catch (error) {
        console.error('Submit response error:', error);
        alert(`Failed to submit response: ${error.message}`);
    } finally {
        document.getElementById('loading').style.display = 'none';
    }
}

// Download report
async function downloadReport(reportID) {
    console.log('Downloading report:', reportID);
    try {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 5000);
        const response = await fetch(`/api/report?report_id=${encodeURIComponent(reportID)}`, {
            signal: controller.signal
        });
        clearTimeout(timeoutId);
        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`API error: ${response.status} ${errorText}`);
        }
        const blob = await response.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `report-${reportID}.json`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        window.URL.revokeObjectURL(url);
        console.log('Report downloaded successfully:', reportID);
    } catch (error) {
        console.error('Download report error:', error);
        alert(`Failed to download report: ${error.message}`);
    }
}

// Subscribe to chart updates
function subscribeChartUpdates() {
    if (!selectedPair) {
        console.error('Cannot start chart updates: no selected pair');
        return;
    }
    console.log('Subscribing to chart updates for:', selectedPair);
    if (chartUpdateInterval) {
        clearInterval(chartUpdateInterval);
        console.log('Cleared previous chart update interval');
    }

    const cycle = document.getElementById('cycle')?.value.trim() || '30s';
    let intervalMs = 30000;
    const match = cycle.match(/^(\d+)([smh])$/);
    if (match) {
        const value = parseInt(match[1]);
        const unit = match[2];
        if (unit === 's') intervalMs = value * 1000;
        else if (unit === 'm') intervalMs = value * 60 * 1000;
        else if (unit === 'h') intervalMs = value * 3600 * 1000;
    }

    chartUpdateInterval = setInterval(async () => {
        if (!isMonitoring) {
            console.log('Stopping chart updates: monitoring stopped');
            clearInterval(chartUpdateInterval);
            chartUpdateInterval = null;
            return;
        }
        try {
            const controller = new AbortController();
            const timeoutId = setTimeout(() => controller.abort(), 5000);
            const response = await fetch(`/api/chart?symbol=${encodeURIComponent(selectedPair)}`, {
                signal: controller.signal
            });
            clearTimeout(timeoutId);
            if (!response.ok) {
                const errorText = await response.text();
                throw new Error(`API error: ${response.status} ${errorText}`);
            }
            const data = await response.json();
            if (data.chart_data) {
                plotCharts(data.chart_data, true);
                await fetchPrompt();
            } else {
                console.warn('No chart_data received');
            }
        } catch (error) {
            console.error('Chart update error:', error);
            if (!error.message.includes('404')) {
                alert(`Chart update failed: ${error.message}`);
            }
        }
    }, intervalMs);
}

// Plot charts using Plotly
function plotCharts(data, update = false) {
    console.log('Plotting charts, update:', update);
    const chartsContainer = document.getElementById('charts');
    if (!chartsContainer) {
        console.error('Charts container not found');
        alert('UI error: Charts container not found');
        return;
    }

    for (const interval in data.kline) {
        const klines = data.kline[interval];
        if (!klines || klines.length === 0) {
            console.warn(`No kline data for interval: ${interval}`);
            continue;
        }

        const times = klines.map(k => new Date(k.open_time).toISOString());
        const opens = klines.map(k => parseFloat(k.open));
        const highs = klines.map(k => parseFloat(k.high));
        const lows = klines.map(k => parseFloat(k.low));
        const closes = klines.map(k => parseFloat(k.close));
        const volumes = klines.map(k => parseFloat(k.volume));

        const candlestickTrace = {
            x: times,
            open: opens,
            high: highs,
            low: lows,
            close: closes,
            type: 'candlestick',
            name: `${interval} K-line`,
            increasing: { line: { color: '#28a745' } },
            decreasing: { line: { color: '#dc3545' } }
        };

        const volumeTrace = {
            x: times,
            y: volumes,
            type: 'bar',
            name: `${interval} Volume`,
            yaxis: 'y2',
            marker: { color: '#007bff', opacity: 0.4 }
        };

        const layout = {
            title: `${selectedPair} - ${interval} K-line`,
            xaxis: { title: 'Time', type: 'date' },
            yaxis: { title: 'Price', side: 'left' },
            yaxis2: {
                title: 'Volume',
                overlaying: 'y',
                side: 'right',
                showgrid: false
            },
            showlegend: true,
            margin: { t: 50, b: 50, l: 50, r: 50 },
            height: 500
        };

        const chartDivId = `chart-${interval}`;
        let chartDiv = document.getElementById(chartDivId);
        if (!chartDiv) {
            chartDiv = document.createElement('div');
            chartDiv.id = chartDivId;
            chartDiv.style.marginBottom = '20px';
            chartsContainer.appendChild(chartDiv);
        }

        if (!charts[interval] || !update) {
            Plotly.newPlot(chartDivId, [candlestickTrace, volumeTrace], layout);
            charts[interval] = true;
            console.log(`Created new chart for interval: ${interval}`);
        } else {
            Plotly.react(chartDivId, [candlestickTrace, volumeTrace], layout);
            console.log(`Updated chart for interval: ${interval}`);
        }
    }
}

// Initialize on page load
document.addEventListener('DOMContentLoaded', () => {
    console.log('DOM loaded, initializing...');
    initializeState();

    // Bind events
    const pairSearch = document.getElementById('pair-search');
    if (pairSearch) {
        pairSearch.addEventListener('input', () => {
            searchPairs();
        });
    } else {
        console.error('Pair search input not found');
    }

    const runAnalysisBtn = document.getElementById('run-analysis');
    if (runAnalysisBtn) {
        runAnalysisBtn.addEventListener('click', startMonitor);
    } else {
        console.error('Run analysis button not found');
    }

    const stopMonitorBtn = document.getElementById('stop-monitor');
    if (stopMonitorBtn) {
        stopMonitorBtn.addEventListener('click', stopMonitor);
    } else {
        console.error('Stop monitor button not found');
    }

    const submitResponseBtn = document.getElementById('submit-response');
    if (submitResponseBtn) {
        submitResponseBtn.addEventListener('click', submitResponse);
    } else {
        console.error('Submit response button not found');
    }
});

// Clean up on page unload
window.addEventListener('beforeunload', () => {
    if (chartUpdateInterval) {
        clearInterval(chartUpdateInterval);
        chartUpdateInterval = null;
        console.log('Cleared chart update interval on unload');
    }
});