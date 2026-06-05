/**
 * 公开页面主逻辑
 */

import {
    escapeHtml, showToast, statusColor,
    formatDate, formatTime, formatTypeBadge, formatFreqCell, freqToBand,
    renderPagination, startClock
} from '../common/index.js';

// ===== 状态变量 =====
let recentPage = 1, searchPage = 1;
let lastSearchFilters = {};
let lastRecentBand = '', lastRecentMode = '', lastRecentQsoType = '';
let stationCallsign = 'BH7GUL';
let stationName = 'QSL & Log Management';

// ===== 电台信息加载 =====
async function loadStationInfo() {
    try {
        const resp = await fetch('/api/station-info');
        const data = await resp.json();
        stationCallsign = data.callsign || 'BH7GUL';
        stationName = data.station_name || 'QSL & Log Management';
        document.title = `${stationCallsign} ${stationName}`;
        document.getElementById('stationTitle').textContent = `${stationCallsign} ${stationName}`;
        document.getElementById('searchInput').placeholder = `请输入呼号（如 ${stationCallsign}）`;
    } catch (err) {
        console.error('Failed to load station info:', err);
    }
}

// ===== 表格行渲染 =====
function rowHtml(log) {
    const escapedCall = escapeHtml(log.call);
    let callHtml = `<a href="https://www.qrz.com/db/${encodeURIComponent(log.call)}" target="_blank" class="text-blue-600 hover:text-blue-800 hover:underline">${escapedCall}</a>`;
    if (log.is_sk) {
        callHtml += ' <span class="px-1.5 py-0.5 rounded text-xs bg-gray-200 text-gray-600 ml-1">SK</span>';
    }

    const timeDisplay = log.qso_type === 'EYEBALL' ? '-' : formatTime(log.time_on);

    return `<tr class="border-b hover:bg-gray-50">
        <td class="px-3 py-2 font-medium whitespace-nowrap">${callHtml}</td>
        <td class="px-3 py-2 whitespace-nowrap">${escapeHtml(formatDate(log.qso_date))}</td>
        <td class="px-3 py-2 whitespace-nowrap">${escapeHtml(timeDisplay)}</td>
        <td class="px-3 py-2 text-gray-600">${formatFreqCell(log)}</td>
        <td class="px-3 py-2 whitespace-nowrap">${escapeHtml(log.mode) || '-'}</td>
        <td class="px-3 py-2 whitespace-nowrap">${formatTypeBadge(log.qso_type, log.qth)}</td>
        <td class="px-3 py-2 text-center whitespace-nowrap">${escapeHtml(log.rst_sent) || '-'}</td>
        <td class="px-3 py-2 text-center whitespace-nowrap">${escapeHtml(log.rst_rcvd) || '-'}</td>
        <td class="px-3 py-2 whitespace-nowrap"><span class="px-2 py-1 rounded-full text-xs ${statusColor(log.qsl_status)}">${escapeHtml(log.qsl_status)}</span></td>
    </tr>`;
}

// ===== 最近通联加载 =====
function loadRecent(band, mode, qsoType, page) {
    page = page || 1;
    recentPage = page;
    lastRecentBand = band || '';
    lastRecentMode = mode || '';
    lastRecentQsoType = qsoType || '';

    const params = new URLSearchParams();
    if (band) params.set('band', band);
    if (mode) params.set('mode', mode);
    if (qsoType) params.set('qso_type', qsoType);
    params.set('page', page);
    params.set('page_size', 20);

    fetch(`/api/recent?${params}`)
        .then(r => r.json())
        .then(data => {
            const tbody = document.getElementById('recentTableBody');
            const noData = document.getElementById('noRecentData');

            if (data.logs.length === 0) {
                noData.classList.remove('hidden');
                tbody.innerHTML = '';
                document.getElementById('recentPagination').classList.add('hidden');
                return;
            }

            noData.classList.add('hidden');
            tbody.innerHTML = data.logs.map(rowHtml).join('');
            renderPagination('recentPagination', 'recentPaginationInfo', data, 'goRecentPage');
        })
        .catch(() => {
            document.getElementById('noRecentData').classList.remove('hidden');
        });
}

// ===== 搜索加载 =====
function loadSearch(filters, page) {
    page = page || 1;
    searchPage = page;
    lastSearchFilters = filters;

    const params = new URLSearchParams();
    if (filters.call) params.set('call', filters.call);
    if (filters.band) params.set('band', filters.band);
    if (filters.mode) params.set('mode', filters.mode);
    if (filters.date_from) params.set('date_from', filters.date_from);
    if (filters.date_to) params.set('date_to', filters.date_to);
    params.set('page', page);
    params.set('page_size', 20);

    fetch(`/api/search?${params}`)
        .then(r => r.json())
        .then(data => {
            const resultDiv = document.getElementById('searchResult');
            const tbody = document.getElementById('searchTableBody');
            resultDiv.classList.remove('hidden');

            if (data.logs.length === 0) {
                tbody.innerHTML = '<tr><td colspan="9" class="px-4 py-4 text-center text-gray-500">未找到相关记录</td></tr>';
                document.getElementById('searchPagination').classList.add('hidden');
            } else {
                tbody.innerHTML = data.logs.map(rowHtml).join('');
                renderPagination('searchPagination', 'searchPaginationInfo', data, 'goSearchPage');
            }
        })
        .catch(() => showToast('查询失败，请重试', 'error'));
}

// ===== 全局函数（供分页按钮调用）=====
window.goRecentPage = function(page) {
    loadRecent(lastRecentBand, lastRecentMode, lastRecentQsoType, page);
};

window.goSearchPage = function(page) {
    loadSearch(lastSearchFilters, page);
};

// ===== 初始化 =====
export function init() {
    // 加载电台信息
    loadStationInfo();

    // 加载最近通联
    loadRecent();

    // 启动时钟
    startClock();

    // 筛选控件事件
    document.getElementById('filterApply').addEventListener('click', function() {
        loadRecent(
            document.getElementById('filterBand').value,
            document.getElementById('filterMode').value,
            document.getElementById('filterQsoType').value,
            1
        );
    });

    document.getElementById('filterClear').addEventListener('click', function() {
        document.getElementById('filterBand').value = '';
        document.getElementById('filterMode').value = '';
        document.getElementById('filterQsoType').value = '';
        loadRecent('', '', '', 1);
    });

    // 搜索框自动转大写
    document.getElementById('searchInput').addEventListener('input', function() {
        this.value = this.value.toUpperCase();
    });

    // 搜索表单提交
    document.getElementById('searchForm').addEventListener('submit', function(e) {
        e.preventDefault();
        const filters = {
            call: document.getElementById('searchInput').value.trim(),
            band: document.getElementById('searchBand').value,
            mode: document.getElementById('searchMode').value,
            date_from: document.getElementById('searchDateFrom').value,
            date_to: document.getElementById('searchDateTo').value,
        };
        // 至少需要一个筛选条件
        if (!filters.call && !filters.band && !filters.mode && !filters.date_from && !filters.date_to) {
            showToast('请至少输入一个查询条件', 'error');
            return;
        }
        loadSearch(filters, 1);
    });
}
