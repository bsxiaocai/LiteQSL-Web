/**
 * ADIF/CSV 导入导出管理
 */

import { showToast, setLoading } from '../common/index.js';
import { csrfHeaders } from './auth.js';
import { loadLogs, getCurrentPage, getCurrentFilters } from './qso-table.js';

// 导入 ADIF
async function importADIF(file, force = false) {
    const formData = new FormData();
    formData.append('file', file);
    if (force) {
        formData.append('force', 'true');
    }

    try {
        const resp = await fetch('/api/admin/import-adif', {
            method: 'POST',
            headers: csrfHeaders(),
            body: formData
        });

        const data = await resp.json();

        if (data.ok) {
            showToast(`成功导入 ${data.count} 条记录`, 'success');
            loadLogs(getCurrentPage(), getCurrentFilters());
            return { ok: true, count: data.count };
        } else if (data.duplicates) {
            return { ok: false, duplicates: true, detail: data.detail };
        } else {
            showToast(data.detail || '导入失败', 'error');
            return { ok: false, detail: data.detail };
        }
    } catch (err) {
        showToast('导入请求失败', 'error');
        return { ok: false, detail: '请求失败' };
    }
}

// 导出 ADIF
function exportADIF() {
    window.location.href = '/api/admin/export-adif';
}

// 导出 CSV
function exportCSV() {
    window.location.href = '/api/admin/export-csv';
}

// 初始化导入导出功能
export function initImportExport() {
    const importForm = document.getElementById('importForm');
    const exportBtn = document.getElementById('exportBtn');
    const exportCsvBtn = document.getElementById('exportCsvBtn');

    if (importForm) {
        importForm.addEventListener('submit', async function(e) {
            e.preventDefault();

            const fileInput = document.getElementById('adifFile');
            if (!fileInput.files.length) {
                showToast('请选择 ADIF 文件', 'error');
                return;
            }

            const btn = this.querySelector('button[type="submit"]');
            setLoading(btn, true);

            const result = await importADIF(fileInput.files[0]);

            setLoading(btn, false);

            if (result.duplicates) {
                if (confirm('检测到重复记录，是否强制导入？')) {
                    setLoading(btn, true);
                    const retryResult = await importADIF(fileInput.files[0], true);
                    setLoading(btn, false);

                    if (retryResult.ok) {
                        fileInput.value = '';
                    }
                }
            } else if (result.ok) {
                fileInput.value = '';
            }
        });
    }

    if (exportBtn) {
        exportBtn.addEventListener('click', exportADIF);
    }

    if (exportCsvBtn) {
        exportCsvBtn.addEventListener('click', exportCSV);
    }
}
