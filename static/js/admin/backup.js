/**
 * 数据库备份管理
 */

import { showToast, setLoading, escapeHtml } from '../common/index.js';
import { csrfHeaders } from './auth.js';
import { loadLogs } from './qso-table.js';

// 格式化文件大小
function formatSize(bytes) {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
}

// 加载备份列表
async function loadBackups() {
    try {
        const resp = await fetch('/api/admin/backups');
        const data = await resp.json();

        const container = document.getElementById('backupListContainer');
        if (!container) return;

        if (!data.backups || data.backups.length === 0) {
            container.innerHTML = '<p class="text-gray-500 text-sm">暂无备份记录</p>';
            return;
        }

        let html = '<div class="overflow-x-auto"><table class="w-full text-sm text-left">';
        html += '<thead class="bg-gray-50 text-gray-600"><tr>';
        html += '<th class="px-3 py-2">文件名</th>';
        html += '<th class="px-3 py-2">大小</th>';
        html += '<th class="px-3 py-2">创建时间</th>';
        html += '<th class="px-3 py-2">操作</th>';
        html += '</tr></thead><tbody>';

        data.backups.forEach(b => {
            const safeFilename = escapeHtml(b.filename);
            html += `<tr class="border-b hover:bg-gray-50">
                <td class="px-3 py-2 font-mono text-xs">${safeFilename}</td>
                <td class="px-3 py-2">${formatSize(b.size)}</td>
                <td class="px-3 py-2">${escapeHtml(b.created_at)}</td>
                <td class="px-3 py-2 whitespace-nowrap">
                    <button onclick="window.downloadBackup('${safeFilename}')" class="text-blue-600 hover:text-blue-800 text-xs mr-2">下载</button>
                    <button onclick="window.restoreFromBackup('${safeFilename}')" class="text-orange-600 hover:text-orange-800 text-xs mr-2">恢复</button>
                    <button onclick="window.deleteBackup('${safeFilename}')" class="text-red-600 hover:text-red-800 text-xs">删除</button>
                </td>
            </tr>`;
        });

        html += '</tbody></table></div>';
        container.innerHTML = html;
    } catch (err) {
        console.error('Failed to load backups:', err);
        const container = document.getElementById('backupListContainer');
        if (container) {
            container.innerHTML = '<p class="text-red-500 text-sm">加载备份列表失败</p>';
        }
    }
}

// 创建备份
async function createBackup() {
    try {
        const resp = await fetch('/api/admin/backup', {
            method: 'POST',
            headers: csrfHeaders()
        });

        const data = await resp.json();

        if (data.ok) {
            showToast('备份成功', 'success');
            loadBackups();
        } else {
            showToast(data.detail || '备份失败', 'error');
        }
    } catch (err) {
        showToast('备份请求失败', 'error');
    }
}

// 下载备份
function downloadBackup(filename) {
    window.location.href = '/api/admin/backups/' + encodeURIComponent(filename);
}

// 删除备份
async function deleteBackup(filename) {
    if (!confirm('确定删除备份 ' + filename + '？')) return;

    try {
        const resp = await fetch('/api/admin/backups/' + encodeURIComponent(filename), {
            method: 'DELETE',
            headers: csrfHeaders()
        });

        const data = await resp.json();

        if (data.ok) {
            showToast('备份已删除', 'success');
            loadBackups();
        } else {
            showToast(data.detail || '删除失败', 'error');
        }
    } catch (err) {
        showToast('删除请求失败', 'error');
    }
}

// 恢复备份
async function restoreFromBackup(filename) {
    if (!confirm('确定要恢复到备份 ' + filename + '？\n当前数据库将自动备份。')) return;

    try {
        const resp = await fetch('/api/admin/restore', {
            method: 'POST',
            headers: csrfHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify({ filename: filename })
        });

        const data = await resp.json();

        if (data.ok) {
            showToast('数据库已恢复（安全备份: ' + data.safety_backup + '）', 'success');
            loadLogs(1, {});
            loadBackups();
        } else {
            showToast(data.detail || '恢复失败', 'error');
        }
    } catch (err) {
        showToast('恢复请求失败', 'error');
    }
}

// 初始化备份管理
export function initBackup() {
    const backupBtn = document.getElementById('backupBtn');

    if (backupBtn) {
        backupBtn.addEventListener('click', function() {
            setLoading(this, true);
            createBackup().finally(() => setLoading(this, false));
        });
    }

    // 注册全局函数
    window.downloadBackup = downloadBackup;
    window.deleteBackup = deleteBackup;
    window.restoreFromBackup = restoreFromBackup;

    // 加载备份列表
    loadBackups();
}
