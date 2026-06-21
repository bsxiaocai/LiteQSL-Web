/**
 * 通联记录表格管理
 */

import {
    escapeHtml, showToast, statusColor,
    formatQsoDateTime, formatTypeBadge, formatFreqCell,
    renderPagination, QSL_STATUSES
} from '../common/index.js';
import { csrfHeaders } from './auth.js';

// 状态变量
let currentPage = 1;
let currentFilters = {};
let allLogs = [];
let displayTimezone = 'Asia/Shanghai';
let displayTimezoneLoaded = false;

function timezoneAbbreviation() {
    return displayTimezone === 'UTC' ? 'UTC' : 'BJT';
}

function updateTimezoneHeaders() {
    const abbreviation = timezoneAbbreviation();
    document.querySelectorAll('.admin-date-timezone-label').forEach(element => {
        element.textContent = `日期（${abbreviation}）`;
    });
    document.querySelectorAll('.admin-time-timezone-label').forEach(element => {
        element.textContent = `时间（${abbreviation}）`;
    });
}

async function ensureDisplayTimezone() {
    if (displayTimezoneLoaded) return;
    try {
        const response = await fetch('/api/admin/settings');
        if (response.ok) {
            const data = await response.json();
            displayTimezone = data.settings?.visitor_timezone || 'Asia/Shanghai';
        }
    } catch (error) {
        console.error('Failed to load table timezone:', error);
    }
    displayTimezoneLoaded = true;
    updateTimezoneHeaders();
}

// 加载日志
export async function loadLogs(page = 1, filters = {}) {
    await ensureDisplayTimezone();
    currentPage = page;
    currentFilters = filters;

    const params = new URLSearchParams();
    params.set('page', page);
    params.set('page_size', 50);

    if (filters.call) params.set('call', filters.call);
    if (filters.band) params.set('band', filters.band);
    if (filters.mode) params.set('mode', filters.mode);
    if (filters.qsl_status) params.set('qsl_status', filters.qsl_status);
    if (filters.qso_type) params.set('qso_type', filters.qso_type);
    if (filters.date_from) params.set('date_from', filters.date_from);
    if (filters.date_to) params.set('date_to', filters.date_to);
    if (filters.is_sk !== undefined && filters.is_sk !== '') params.set('is_sk', filters.is_sk);

    try {
        const resp = await fetch(`/api/admin/logs?${params}`);

        // 检查响应状态码
        if (!resp.ok) {
            if (resp.status === 401) {
                // 未登录，跳转到登录页面
                document.getElementById('loginPage').classList.remove('hidden');
                document.getElementById('adminPage').classList.add('hidden');
                showToast('请先登录', 'error');
                return { logs: [], total: 0, page: 1, page_size: 50 };
            }
            showToast('加载记录失败', 'error');
            return { logs: [], total: 0, page: 1, page_size: 50 };
        }

        const data = await resp.json();

        allLogs = data.logs || [];
        renderLogsTable(data);
        renderLogsPagination(data);

        return data;
    } catch (err) {
        console.error('Failed to load logs:', err);
        showToast('加载记录失败', 'error');
        return { logs: [], total: 0, page: 1, page_size: 50 };
    }
}

// 渲染表格
function renderLogsTable(data) {
    const tbody = document.getElementById('logsTableBody');
    if (!tbody) return;

    if (data.logs.length === 0) {
        tbody.innerHTML = '<tr><td colspan="10" class="px-4 py-4 text-center text-gray-500">暂无记录</td></tr>';
        return;
    }

    tbody.innerHTML = data.logs.map(log => {
        const escapedCall = escapeHtml(log.call);
        let callHtml = `<a href="https://www.qrz.com/db/${encodeURIComponent(log.call)}" target="_blank" class="text-blue-600 hover:text-blue-800 hover:underline">${escapedCall}</a>`;
        if (log.is_sk) {
            callHtml += ' <span class="px-1.5 py-0.5 rounded text-xs bg-gray-200 text-gray-600 ml-1">SK</span>';
        }

        const dateTime = formatQsoDateTime(log.qso_date, log.time_on, displayTimezone);
        const timeDisplay = log.qso_type === 'EYEBALL' ? '-' : dateTime.time;
        const rstDisplay = `${escapeHtml(log.rst_sent) || '-'}/${escapeHtml(log.rst_rcvd) || '-'}`;

        return `<tr class="border-b hover:bg-gray-50">
            <td class="px-3 py-2 text-center">
                <input type="checkbox" class="log-checkbox w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500" value="${log.id}" onchange="window.updateBatchToolbar()">
            </td>
            <td class="px-3 py-2 font-medium whitespace-nowrap">${callHtml}</td>
            <td class="px-3 py-2 whitespace-nowrap">${escapeHtml(dateTime.date)}</td>
            <td class="px-3 py-2 whitespace-nowrap">${escapeHtml(timeDisplay)}</td>
            <td class="px-3 py-2 text-gray-600">${formatFreqCell(log)}</td>
            <td class="px-3 py-2 whitespace-nowrap">${escapeHtml(log.mode) || '-'}</td>
            <td class="px-3 py-2 whitespace-nowrap">${formatTypeBadge(log.qso_type, log.qth)}</td>
            <td class="px-3 py-2 text-center whitespace-nowrap">${rstDisplay}</td>
            <td class="px-3 py-2 whitespace-nowrap">
                <select onchange="window.updateLogStatus(${log.id}, this.value)" class="text-xs border rounded px-1 py-0.5">
                    ${QSL_STATUSES.map(s => `<option value="${s}" ${s === log.qsl_status ? 'selected' : ''}>${s}</option>`).join('')}
                </select>
            </td>
            <td class="px-3 py-2 whitespace-nowrap text-xs">
                <button onclick="window.openEditModal(${log.id})" class="text-blue-600 hover:text-blue-800 mr-2">编辑</button>
                <button onclick="window.deleteLog(${log.id})" class="text-red-600 hover:text-red-800">删除</button>
            </td>
        </tr>`;
    }).join('');
}

// 渲染分页
function renderLogsPagination(data) {
    renderPagination('logsPagination', 'logsPaginationInfo', data, 'goToPage');
}

// 更新 QSL 状态
export async function updateLogStatus(id, status) {
    try {
        const resp = await fetch(`/api/admin/logs/${id}/status`, {
            method: 'PUT',
            headers: csrfHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify({ qsl_status: status })
        });

        if (resp.ok) {
            showToast('状态已更新', 'success');
        } else {
            showToast('更新失败', 'error');
        }
    } catch (err) {
        showToast('请求失败', 'error');
    }
}

// 删除记录
export async function deleteLog(id) {
    if (!confirm('确定要删除这条记录吗？此操作不可撤销。')) return;

    try {
        const resp = await fetch(`/api/admin/logs/${id}`, {
            method: 'DELETE',
            headers: csrfHeaders()
        });

        if (resp.ok) {
            showToast('记录已删除', 'success');
            loadLogs(currentPage, currentFilters);
        } else {
            showToast('删除失败', 'error');
        }
    } catch (err) {
        showToast('请求失败', 'error');
    }
}

// 获取所有日志（供编辑弹窗使用）
export function getAllLogs() {
    return allLogs;
}

// 获取当前筛选条件
export function getCurrentFilters() {
    return currentFilters;
}

// 获取当前页码
export function getCurrentPage() {
    return currentPage;
}

// 初始化筛选控件
export function initFilters() {
    const filterApply = document.getElementById('filterApply');
    const filterClear = document.getElementById('filterClear');

    if (filterApply) {
        filterApply.addEventListener('click', function() {
            const filters = getFilterValues();
            loadLogs(1, filters);
            syncFiltersToURL(filters, 1);
        });
    }

    if (filterClear) {
        filterClear.addEventListener('click', function() {
            clearFilterInputs();
            loadLogs(1, {});
            syncFiltersToURL({}, 1);
        });
    }

    // 从 URL 加载筛选条件
    loadFiltersFromURL();
}

// 获取筛选条件值
function getFilterValues() {
    return {
        call: document.getElementById('filterCall')?.value || '',
        band: document.getElementById('filterBand')?.value || '',
        mode: document.getElementById('filterMode')?.value || '',
        qsl_status: document.getElementById('filterStatus')?.value || '',
        qso_type: document.getElementById('filterQsoType')?.value || '',
        date_from: document.getElementById('filterDateFrom')?.value || '',
        date_to: document.getElementById('filterDateTo')?.value || '',
        is_sk: document.getElementById('filterSK')?.value || ''
    };
}

// 清空筛选输入框
function clearFilterInputs() {
    document.getElementById('filterCall').value = '';
    document.getElementById('filterBand').value = '';
    document.getElementById('filterMode').value = '';
    document.getElementById('filterStatus').value = '';
    document.getElementById('filterQsoType').value = '';
    document.getElementById('filterDateFrom').value = '';
    document.getElementById('filterDateTo').value = '';
    document.getElementById('filterSK').value = '';
}

// 设置筛选输入框值
function setFilterValues(filters) {
    if (filters.call) document.getElementById('filterCall').value = filters.call;
    if (filters.band) document.getElementById('filterBand').value = filters.band;
    if (filters.mode) document.getElementById('filterMode').value = filters.mode;
    if (filters.qsl_status) document.getElementById('filterStatus').value = filters.qsl_status;
    if (filters.qso_type) document.getElementById('filterQsoType').value = filters.qso_type;
    if (filters.date_from) document.getElementById('filterDateFrom').value = filters.date_from;
    if (filters.date_to) document.getElementById('filterDateTo').value = filters.date_to;
    if (filters.is_sk) document.getElementById('filterSK').value = filters.is_sk;
}

// 同步筛选条件到 URL
function syncFiltersToURL(filters, page) {
    const params = new URLSearchParams();
    if (filters.call) params.set('call', filters.call);
    if (filters.band) params.set('band', filters.band);
    if (filters.mode) params.set('mode', filters.mode);
    if (filters.qsl_status) params.set('qsl_status', filters.qsl_status);
    if (filters.qso_type) params.set('qso_type', filters.qso_type);
    if (filters.date_from) params.set('date_from', filters.date_from);
    if (filters.date_to) params.set('date_to', filters.date_to);
    if (filters.is_sk) params.set('is_sk', filters.is_sk);
    if (page > 1) params.set('page', page);

    const queryString = params.toString();
    const newURL = queryString ? `${window.location.pathname}?${queryString}` : window.location.pathname;
    history.replaceState(null, '', newURL);
}

// 从 URL 加载筛选条件
function loadFiltersFromURL() {
    const params = new URLSearchParams(window.location.search);
    if (params.toString() === '') return;

    const filters = {
        call: params.get('call') || '',
        band: params.get('band') || '',
        mode: params.get('mode') || '',
        qsl_status: params.get('qsl_status') || '',
        qso_type: params.get('qso_type') || '',
        date_from: params.get('date_from') || '',
        date_to: params.get('date_to') || '',
        is_sk: params.get('is_sk') || ''
    };

    const page = parseInt(params.get('page')) || 1;

    // 设置筛选输入框
    setFilterValues(filters);

    // 加载数据
    loadLogs(page, filters);
}

// ===== 批量操作 =====

// 获取选中的记录 ID
function getSelectedIds() {
    const checkboxes = document.querySelectorAll('.log-checkbox:checked');
    return Array.from(checkboxes).map(cb => parseInt(cb.value));
}

// 更新批量操作工具栏显示
function updateBatchToolbar() {
    const ids = getSelectedIds();
    const toolbar = document.getElementById('batchToolbar');
    const countEl = document.getElementById('selectedCount');

    if (ids.length > 0) {
        toolbar.classList.remove('hidden');
        countEl.textContent = ids.length;
    } else {
        toolbar.classList.add('hidden');
    }
}

// 全选/取消全选
function toggleSelectAll() {
    const selectAll = document.getElementById('selectAll');
    const checkboxes = document.querySelectorAll('.log-checkbox');
    checkboxes.forEach(cb => {
        cb.checked = selectAll.checked;
    });
    updateBatchToolbar();
}

// 批量删除
async function batchDelete() {
    const ids = getSelectedIds();
    if (!ids.length) {
        showToast('请先选择要删除的记录', 'error');
        return;
    }
    if (!confirm(`确定要删除 ${ids.length} 条记录吗？此操作不可撤销。`)) return;

    try {
        const resp = await fetch('/api/admin/logs/batch-delete', {
            method: 'POST',
            headers: csrfHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify({ ids })
        });
        const data = await resp.json();
        if (resp.ok && data.ok) {
            showToast(`成功删除 ${data.deleted} 条记录`, 'success');
            loadLogs(currentPage, currentFilters);
        } else {
            showToast(data.detail || '批量删除失败', 'error');
        }
    } catch (err) {
        showToast('请求失败', 'error');
    }
}

// 批量修改 QSL 状态
async function batchUpdateStatus() {
    const ids = getSelectedIds();
    const status = document.getElementById('batchStatusSelect').value;

    if (!ids.length) {
        showToast('请先选择要修改的记录', 'error');
        return;
    }
    if (!status) {
        showToast('请选择目标状态', 'error');
        return;
    }

    try {
        const resp = await fetch('/api/admin/logs/batch-status', {
            method: 'POST',
            headers: csrfHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify({ ids, status })
        });
        const data = await resp.json();
        if (resp.ok && data.ok) {
            showToast(`成功更新 ${data.updated} 条记录状态`, 'success');
            loadLogs(currentPage, currentFilters);
        } else {
            showToast(data.detail || '批量更新失败', 'error');
        }
    } catch (err) {
        showToast('请求失败', 'error');
    }
}

// 批量修改 SK 标记
async function batchMarkSK(is_sk) {
    const ids = getSelectedIds();
    if (!ids.length) {
        showToast('请先选择要修改的记录', 'error');
        return;
    }

    const action = is_sk ? '标记为 SK' : '取消 SK 标记';
    if (!confirm(`确定要将 ${ids.length} 条记录${action}吗？`)) return;

    try {
        const resp = await fetch('/api/admin/logs/batch-sk', {
            method: 'POST',
            headers: csrfHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify({ ids, is_sk })
        });
        const data = await resp.json();
        if (resp.ok && data.ok) {
            showToast(`成功${action} ${data.updated} 条记录`, 'success');
            loadLogs(currentPage, currentFilters);
        } else {
            showToast(data.detail || '批量更新失败', 'error');
        }
    } catch (err) {
        showToast('请求失败', 'error');
    }
}

// 批量导出
async function batchExport(format) {
    const ids = getSelectedIds();
    if (!ids.length) {
        showToast('请先选择要导出的记录', 'error');
        return;
    }

    try {
        const resp = await fetch('/api/admin/logs/batch-export', {
            method: 'POST',
            headers: csrfHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify({ ids, format })
        });

        if (resp.ok) {
            const blob = await resp.blob();
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = `qsl_export_${new Date().toISOString().slice(0, 10)}.${format === 'adif' ? 'adi' : 'csv'}`;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            window.URL.revokeObjectURL(url);
            showToast(`成功导出 ${ids.length} 条记录`, 'success');
        } else {
            const data = await resp.json();
            showToast(data.detail || '导出失败', 'error');
        }
    } catch (err) {
        showToast('请求失败', 'error');
    }
}

// 注册全局函数
export function registerGlobalFunctions() {
    window.addEventListener('table-timezone-changed', event => {
        displayTimezone = event.detail?.timezone || 'Asia/Shanghai';
        displayTimezoneLoaded = true;
        updateTimezoneHeaders();
        renderLogsTable({ logs: allLogs });
    });

    window.goToPage = function(page) {
        loadLogs(page, currentFilters);
        syncFiltersToURL(currentFilters, page);
    };

    window.updateLogStatus = updateLogStatus;
    window.deleteLog = deleteLog;
    window.openEditModal = function(id) {
        // 由 edit-modal.js 注册
    };

    // 批量操作相关
    window.updateBatchToolbar = updateBatchToolbar;
    window.batchDelete = batchDelete;
    window.batchUpdateStatus = batchUpdateStatus;
    window.batchMarkSK = batchMarkSK;
    window.batchExport = batchExport;

    // 全选复选框事件
    const selectAll = document.getElementById('selectAll');
    if (selectAll) {
        selectAll.addEventListener('change', toggleSelectAll);
    }
}
