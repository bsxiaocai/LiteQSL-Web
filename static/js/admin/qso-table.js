/**
 * 通联记录表格管理
 */

import {
    escapeHtml, showToast, statusColor,
    formatDate, formatTime, formatTypeBadge, formatFreqCell,
    renderPagination, QSL_STATUSES, QSO_TYPES, QSO_TYPE_LABELS
} from '../common/index.js';
import { csrfHeaders } from './auth.js';

// 状态变量
let currentPage = 1;
let currentFilters = {};
let allLogs = [];

// 加载日志
export async function loadLogs(page = 1, filters = {}) {
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
        tbody.innerHTML = '<tr><td colspan="9" class="px-4 py-4 text-center text-gray-500">暂无记录</td></tr>';
        return;
    }

    tbody.innerHTML = data.logs.map(log => {
        const escapedCall = escapeHtml(log.call);
        let callHtml = `<a href="https://www.qrz.com/db/${encodeURIComponent(log.call)}" target="_blank" class="text-blue-600 hover:text-blue-800 hover:underline">${escapedCall}</a>`;
        if (log.is_sk) {
            callHtml += ' <span class="px-1.5 py-0.5 rounded text-xs bg-gray-200 text-gray-600 ml-1">SK</span>';
        }

        const timeDisplay = log.qso_type === 'EYEBALL' ? '-' : formatTime(log.time_on);
        const rstDisplay = `${escapeHtml(log.rst_sent) || '-'}/${escapeHtml(log.rst_rcvd) || '-'}`;

        return `<tr class="border-b hover:bg-gray-50">
            <td class="px-3 py-2 font-medium whitespace-nowrap">${callHtml}</td>
            <td class="px-3 py-2 whitespace-nowrap">${escapeHtml(formatDate(log.qso_date))}</td>
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
            const filters = {
                call: document.getElementById('filterCall')?.value || '',
                band: document.getElementById('filterBand')?.value || '',
                mode: document.getElementById('filterMode')?.value || '',
                qsl_status: document.getElementById('filterStatus')?.value || '',
                qso_type: document.getElementById('filterQsoType')?.value || '',
                date_from: document.getElementById('filterDateFrom')?.value || '',
                date_to: document.getElementById('filterDateTo')?.value || '',
                is_sk: document.getElementById('filterSK')?.value || ''
            };
            loadLogs(1, filters);
        });
    }

    if (filterClear) {
        filterClear.addEventListener('click', function() {
            document.getElementById('filterCall').value = '';
            document.getElementById('filterBand').value = '';
            document.getElementById('filterMode').value = '';
            document.getElementById('filterStatus').value = '';
            document.getElementById('filterQsoType').value = '';
            document.getElementById('filterDateFrom').value = '';
            document.getElementById('filterDateTo').value = '';
            document.getElementById('filterSK').value = '';
            loadLogs(1, {});
        });
    }
}

// 注册全局函数
export function registerGlobalFunctions() {
    window.goToPage = function(page) {
        loadLogs(page, currentFilters);
    };

    window.updateLogStatus = updateLogStatus;
    window.deleteLog = deleteLog;
    window.openEditModal = function(id) {
        // 由 edit-modal.js 注册
    };
}
