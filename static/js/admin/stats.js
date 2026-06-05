/**
 * 统计仪表盘模块
 */

import { csrfHeaders } from './auth.js';

// 图表实例
let chartTrend = null;
let chartBand = null;
let chartMode = null;
let chartType = null;
let chartHour = null;

// 颜色配置
const COLORS = [
    '#3B82F6', '#10B981', '#F59E0B', '#EF4444', '#8B5CF6',
    '#EC4899', '#14B8A6', '#F97316', '#6366F1', '#84CC16',
    '#06B6D4', '#D946EF', '#0EA5E9', '#22C55E', '#EAB308',
];

// 加载统计数据
export async function loadStats() {
    try {
        // 并行加载所有统计数据
        const [summary, bandData, modeData, typeData, monthData, hourData, topCalls] = await Promise.all([
            fetch('/api/admin/stats/summary', { headers: csrfHeaders() }).then(r => r.json()),
            fetch('/api/admin/stats/by-band', { headers: csrfHeaders() }).then(r => r.json()),
            fetch('/api/admin/stats/by-mode', { headers: csrfHeaders() }).then(r => r.json()),
            fetch('/api/admin/stats/by-type', { headers: csrfHeaders() }).then(r => r.json()),
            fetch('/api/admin/stats/by-month?months=12', { headers: csrfHeaders() }).then(r => r.json()),
            fetch('/api/admin/stats/by-hour', { headers: csrfHeaders() }).then(r => r.json()),
            fetch('/api/admin/stats/top-calls?limit=20', { headers: csrfHeaders() }).then(r => r.json()),
        ]);

        // 更新概览卡片
        renderSummary(summary);

        // 渲染图表
        renderTrendChart(monthData);
        renderPieChart('chartBand', 'chartBand', bandData, 'band', 'count');
        renderPieChart('chartMode', 'chartMode', modeData, 'mode', 'count');
        renderPieChart('chartType', 'chartType', typeData, 'qso_type', 'count');
        renderHourChart(hourData);
        renderTopCallsTable(topCalls, summary.total_logs);

    } catch (err) {
        console.error('Failed to load stats:', err);
    }
}

// 渲染概览卡片
function renderSummary(data) {
    document.getElementById('statTotalLogs').textContent = data.total_logs.toLocaleString();
    document.getElementById('statTotalCallsigns').textContent = data.total_callsigns.toLocaleString();
    document.getElementById('statThisMonth').textContent = data.this_month.toLocaleString();
    document.getElementById('statThisYear').textContent = data.this_year.toLocaleString();
    document.getElementById('statQslPending').textContent = data.qsl_pending.toLocaleString();
}

// 渲染趋势折线图
function renderTrendChart(data) {
    const ctx = document.getElementById('chartTrend');
    if (!ctx) return;

    if (chartTrend) {
        chartTrend.destroy();
    }

    chartTrend = new Chart(ctx, {
        type: 'line',
        data: {
            labels: data.map(d => d.month),
            datasets: [{
                label: '通联数量',
                data: data.map(d => d.count),
                borderColor: '#3B82F6',
                backgroundColor: 'rgba(59, 130, 246, 0.1)',
                fill: true,
                tension: 0.3,
                pointRadius: 4,
                pointHoverRadius: 6,
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    display: false,
                },
                tooltip: {
                    callbacks: {
                        label: (context) => `${context.parsed.y} 次通联`
                    }
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    ticks: {
                        stepSize: 1,
                    }
                }
            }
        }
    });
}

// 渲染饼图
function renderPieChart(canvasId, chartVar, data, labelKey, valueKey) {
    const ctx = document.getElementById(canvasId);
    if (!ctx) return;

    // 销毁旧图表
    if (chartVar === 'chartBand' && chartBand) chartBand.destroy();
    if (chartVar === 'chartMode' && chartMode) chartMode.destroy();
    if (chartVar === 'chartType' && chartType) chartType.destroy();

    const labels = data.map(d => d[labelKey] || '未知');
    const values = data.map(d => d[valueKey]);

    const chart = new Chart(ctx, {
        type: 'doughnut',
        data: {
            labels: labels,
            datasets: [{
                data: values,
                backgroundColor: COLORS.slice(0, data.length),
                borderWidth: 2,
                borderColor: '#ffffff',
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    position: 'bottom',
                    labels: {
                        padding: 10,
                        usePointStyle: true,
                    }
                },
                tooltip: {
                    callbacks: {
                        label: (context) => {
                            const total = context.dataset.data.reduce((a, b) => a + b, 0);
                            const percentage = ((context.parsed / total) * 100).toFixed(1);
                            return `${context.label}: ${context.parsed} (${percentage}%)`;
                        }
                    }
                }
            }
        }
    });

    // 保存图表实例
    if (chartVar === 'chartBand') chartBand = chart;
    if (chartVar === 'chartMode') chartMode = chart;
    if (chartVar === 'chartType') chartType = chart;
}

// 渲染小时分布柱状图
function renderHourChart(data) {
    const ctx = document.getElementById('chartHour');
    if (!ctx) return;

    if (chartHour) {
        chartHour.destroy();
    }

    // 填充 24 小时数据
    const hourData = new Array(24).fill(0);
    data.forEach(d => {
        if (d.hour >= 0 && d.hour < 24) {
            hourData[d.hour] = d.count;
        }
    });

    chartHour = new Chart(ctx, {
        type: 'bar',
        data: {
            labels: Array.from({ length: 24 }, (_, i) => `${i}:00`),
            datasets: [{
                label: '通联次数',
                data: hourData,
                backgroundColor: 'rgba(59, 130, 246, 0.6)',
                borderColor: '#3B82F6',
                borderWidth: 1,
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    display: false,
                },
                tooltip: {
                    callbacks: {
                        label: (context) => `${context.parsed.y} 次通联`
                    }
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    ticks: {
                        stepSize: 1,
                    }
                }
            }
        }
    });
}

// 渲染 Top 通联对象表格
function renderTopCallsTable(data, totalLogs) {
    const tbody = document.getElementById('topCallsBody');
    if (!tbody) return;

    if (data.length === 0) {
        tbody.innerHTML = '<tr><td colspan="4" class="px-4 py-4 text-center text-gray-500">暂无数据</td></tr>';
        return;
    }

    tbody.innerHTML = data.map((item, index) => {
        const percentage = totalLogs > 0 ? ((item.count / totalLogs) * 100).toFixed(1) : 0;
        const rankClass = index < 3 ? 'font-bold text-blue-600' : '';
        return `<tr class="border-b hover:bg-gray-50">
            <td class="px-4 py-2 ${rankClass}">${index + 1}</td>
            <td class="px-4 py-2">
                <a href="https://www.qrz.com/db/${encodeURIComponent(item.call)}" target="_blank" class="text-blue-600 hover:text-blue-800 hover:underline">${item.call}</a>
            </td>
            <td class="px-4 py-2">${item.count}</td>
            <td class="px-4 py-2">
                <div class="flex items-center gap-2">
                    <div class="flex-1 bg-gray-200 rounded-full h-2">
                        <div class="bg-blue-600 h-2 rounded-full" style="width: ${percentage}%"></div>
                    </div>
                    <span class="text-xs text-gray-500">${percentage}%</span>
                </div>
            </td>
        </tr>`;
    }).join('');
}

// 初始化标签页切换
export function initTabs() {
    const tabBtns = document.querySelectorAll('.tab-btn');
    const sections = document.querySelectorAll('main > [data-tab]');

    tabBtns.forEach(btn => {
        btn.addEventListener('click', function() {
            const tab = this.dataset.tab;

            // 更新标签按钮样式
            tabBtns.forEach(b => {
                b.classList.remove('border-blue-500', 'text-blue-600');
                b.classList.add('border-transparent', 'text-gray-500');
            });
            this.classList.remove('border-transparent', 'text-gray-500');
            this.classList.add('border-blue-500', 'text-blue-600');

            // 切换内容显示
            sections.forEach(sec => {
                sec.classList.toggle('hidden', sec.dataset.tab !== tab);
            });

            // 加载统计数据
            if (tab === 'stats') {
                loadStats();
            }
        });
    });
}
