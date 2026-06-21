/**
 * 编辑弹窗管理
 */

import { showToast, setLoading } from '../common/index.js';
import { csrfHeaders } from './auth.js';
import { getAllLogs, loadLogs, getCurrentPage, getCurrentFilters } from './qso-table.js';

let currentEditId = null;

// 打开编辑弹窗
export function openEditModal(id) {
    const logs = getAllLogs();
    const log = logs.find(l => l.id === id);
    if (!log) {
        showToast('记录未找到', 'error');
        return;
    }

    currentEditId = id;

    // 填充通用字段
    document.getElementById('editCall').value = log.call || '';
    document.getElementById('editQsoDate').value = formatDateForInput(log.qso_date);
    document.getElementById('editTimeOn').value = formatTimeForInput(log.time_on);
    document.getElementById('editInputTimezone').value = 'UTC';
    document.getElementById('editQsoType').value = log.qso_type || 'NORMAL';
    document.getElementById('editQslStatus').value = log.qsl_status || '未发送';
    document.getElementById('editComment').value = log.comment || '';
    document.getElementById('editIsSkCheck').checked = log.is_sk ? true : false;

    // 根据类型填充特定字段
    toggleEditFields(log.qso_type);

    if (log.qso_type === 'SAT') {
        document.getElementById('editSatName').value = log.sat_name || '';
        document.getElementById('editTxFreq').value = log.tx_freq || '';
        document.getElementById('editRxFreq').value = log.rx_freq || '';
        document.getElementById('editSatMode').value = log.mode || '';
        document.getElementById('editSatelliteMode').value = log.sat_mode || '';
        document.getElementById('editSatRstSent').value = log.rst_sent || '';
        document.getElementById('editSatRstRcvd').value = log.rst_rcvd || '';
    } else if (log.qso_type === 'REP') {
        document.getElementById('editRepName').value = log.sat_name || '';
        document.getElementById('editRepTxFreq').value = log.tx_freq || '';
        document.getElementById('editRepRxFreq').value = log.rx_freq || '';
        document.getElementById('editRepMode').value = log.mode || '';
        document.getElementById('editRepRstSent').value = log.rst_sent || '';
        document.getElementById('editRepRstRcvd').value = log.rst_rcvd || '';
    } else if (log.qso_type === 'EYEBALL') {
        document.getElementById('editQth').value = log.qth || '';
    } else {
        document.getElementById('editFreq').value = log.freq || '';
        document.getElementById('editMode').value = log.mode || '';
        document.getElementById('editRstSent').value = log.rst_sent || '';
        document.getElementById('editRstRcvd').value = log.rst_rcvd || '';
    }

    // 显示弹窗
    document.getElementById('editModal').classList.remove('hidden');
}

// 关闭编辑弹窗
function closeEditModal() {
    document.getElementById('editModal').classList.add('hidden');
    currentEditId = null;
}

// 切换编辑弹窗中的字段显示
function toggleEditFields(qsoType) {
    document.getElementById('editNormalFields').classList.toggle('hidden', qsoType !== 'NORMAL');
    document.getElementById('editSatFields').classList.toggle('hidden', qsoType !== 'SAT');
    document.getElementById('editRepFields').classList.toggle('hidden', qsoType !== 'REP');
    document.getElementById('editEyeballFields').classList.toggle('hidden', qsoType !== 'EYEBALL');
    document.getElementById('editTimeGroup').classList.toggle('hidden', qsoType === 'EYEBALL');
    document.getElementById('editTimezoneGroup').classList.toggle('hidden', qsoType === 'EYEBALL');
}

// 格式化日期为 input[type="date"] 格式
function formatDateForInput(dateStr) {
    if (!dateStr || dateStr.length < 8) return '';
    return `${dateStr.substring(0, 4)}-${dateStr.substring(4, 6)}-${dateStr.substring(6, 8)}`;
}

// 格式化时间为 input[type="time"] 格式
function formatTimeForInput(timeStr) {
    if (!timeStr || timeStr.length < 4) return '';
    return `${timeStr.substring(0, 2)}:${timeStr.substring(2, 4)}`;
}

// 收集编辑表单数据
function collectEditData() {
    const qsoType = document.getElementById('editQsoType').value;
    const data = {};

    data.call = document.getElementById('editCall').value.trim();
    data.qso_date = document.getElementById('editQsoDate').value.replace(/-/g, '');
    data.time_on = document.getElementById('editTimeOn').value.replace(/:/g, '');
    data.input_timezone = document.getElementById('editInputTimezone').value;
    data.qso_type = qsoType;
    data.qsl_status = document.getElementById('editQslStatus').value;
    data.comment = document.getElementById('editComment').value.trim();
    data.is_sk = document.getElementById('editIsSkCheck').checked ? 1 : 0;

    if (qsoType === 'SAT') {
        data.sat_name = document.getElementById('editSatName').value.trim();
        data.sat_mode = document.getElementById('editSatelliteMode').value.trim();
        data.tx_freq = document.getElementById('editTxFreq').value.trim();
        data.rx_freq = document.getElementById('editRxFreq').value.trim();
        data.mode = document.getElementById('editSatMode').value;
        data.rst_sent = document.getElementById('editSatRstSent').value.trim();
        data.rst_rcvd = document.getElementById('editSatRstRcvd').value.trim();
    } else if (qsoType === 'REP') {
        data.sat_name = document.getElementById('editRepName').value.trim();
        data.tx_freq = document.getElementById('editRepTxFreq').value.trim();
        data.rx_freq = document.getElementById('editRepRxFreq').value.trim();
        data.mode = document.getElementById('editRepMode').value;
        data.rst_sent = document.getElementById('editRepRstSent').value.trim();
        data.rst_rcvd = document.getElementById('editRepRstRcvd').value.trim();
    } else if (qsoType === 'EYEBALL') {
        data.qth = document.getElementById('editQth').value.trim();
        data.mode = '';
        data.rst_sent = '';
        data.rst_rcvd = '';
    } else {
        data.freq = document.getElementById('editFreq').value.trim();
        data.mode = document.getElementById('editMode').value;
        data.rst_sent = document.getElementById('editRstSent').value.trim();
        data.rst_rcvd = document.getElementById('editRstRcvd').value.trim();
    }

    return data;
}

// 提交编辑
async function submitEdit() {
    if (!currentEditId) return;

    const data = collectEditData();

    if (!data.call) {
        showToast('请输入呼号', 'error');
        return;
    }

    const btn = document.querySelector('#editForm button[type="submit"]');
    setLoading(btn, true);

    try {
        const resp = await fetch(`/api/admin/logs/${currentEditId}`, {
            method: 'PUT',
            headers: csrfHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify(data)
        });

        const result = await resp.json();

        setLoading(btn, false);

        if (resp.ok) {
            showToast('记录已更新', 'success');
            closeEditModal();
            loadLogs(getCurrentPage(), getCurrentFilters());
        } else {
            showToast(result.detail || '更新失败', 'error');
        }
    } catch (err) {
        setLoading(btn, false);
        showToast('请求失败', 'error');
    }
}

// 初始化编辑弹窗
export function initEditModal() {
    const editForm = document.getElementById('editForm');
    const editModalClose = document.getElementById('editModalClose');
    const editQsoType = document.getElementById('editQsoType');

    if (editForm) {
        editForm.addEventListener('submit', function(e) {
            e.preventDefault();
            submitEdit();
        });
    }

    if (editModalClose) {
        editModalClose.addEventListener('click', closeEditModal);
    }

    if (editQsoType) {
        editQsoType.addEventListener('change', function() {
            toggleEditFields(this.value);
        });
    }

    // 注册全局函数
    window.openEditModal = openEditModal;
}

// 设置编辑弹窗的日期时间按钮
export function setupEditDateTimeButtons() {
    const editTodayBtn = document.getElementById('editTodayBtn');
    const editNowBtn = document.getElementById('editNowBtn');

    if (editTodayBtn) {
        editTodayBtn.addEventListener('click', function() {
            setEditNow(true, false);
        });
    }

    if (editNowBtn) {
        editNowBtn.addEventListener('click', function() {
            setEditNow(false, true);
        });
    }
}

function setEditNow(setDate, setTime) {
    const timeZone = document.getElementById('editInputTimezone')?.value || 'UTC';
    const parts = new Intl.DateTimeFormat('en-CA', {
        timeZone,
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        hourCycle: 'h23'
    }).formatToParts(new Date());
    const values = Object.fromEntries(parts.map(part => [part.type, part.value]));
    if (setDate) {
        document.getElementById('editQsoDate').value =
            `${values.year}-${values.month}-${values.day}`;
    }
    if (setTime) {
        document.getElementById('editTimeOn').value = `${values.hour}:${values.minute}`;
    }
}
