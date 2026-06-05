/**
 * QSO 录入表单
 */

import { showToast, setLoading, freqToBand, QSO_TYPE_LABELS } from '../common/index.js';
import { csrfHeaders } from './auth.js';

// QSO 类型切换逻辑
export function setupFormTypeToggle(formPrefix = 'add') {
    const typeSelect = document.getElementById(`${formPrefix}QsoTypeSelect`) ||
                       document.querySelector(`#${formPrefix}Form [name="qso_type"]`);

    if (!typeSelect) return;

    const normalFields = document.getElementById(`${formPrefix}NormalFields`);
    const satFields = document.getElementById(`${formPrefix}SatFields`);
    const repFields = document.getElementById(`${formPrefix}RepFields`);
    const eyeballFields = document.getElementById(`${formPrefix}EyeballFields`);
    const timeGroup = document.getElementById(`${formPrefix}TimeGroup`);

    function toggleFields() {
        const type = typeSelect.value;

        // 隐藏所有特殊字段
        if (normalFields) normalFields.classList.toggle('hidden', type !== 'NORMAL');
        if (satFields) satFields.classList.toggle('hidden', type !== 'SAT');
        if (repFields) repFields.classList.toggle('hidden', type !== 'REP');
        if (eyeballFields) eyeballFields.classList.toggle('hidden', type !== 'EYEBALL');
        if (timeGroup) timeGroup.classList.toggle('hidden', type === 'EYEBALL');
    }

    typeSelect.addEventListener('change', toggleFields);
    toggleFields();
}

// 频率输入自动提示波段
export function setupFreqHint() {
    const freqInput = document.getElementById('addFreqInput') ||
                      document.querySelector('#addForm [name="freq"]');
    const freqBandHint = document.getElementById('freqBandHint');

    if (!freqInput || !freqBandHint) return;

    freqInput.addEventListener('input', function() {
        const band = freqToBand(this.value);
        freqBandHint.textContent = band ? `波段: ${band}` : '';
    });
}

// 呼号自动转大写
export function setupCallUppercase() {
    const callInputs = document.querySelectorAll('input[name="call"]');
    callInputs.forEach(input => {
        input.addEventListener('input', function() {
            this.value = this.value.toUpperCase();
        });
    });
}

// 设置今天的日期
export function setTodayDate(inputId) {
    const input = document.getElementById(inputId);
    if (!input) return;

    const now = new Date();
    const dateStr = now.getFullYear() + '-' +
        String(now.getMonth() + 1).padStart(2, '0') + '-' +
        String(now.getDate()).padStart(2, '0');
    input.value = dateStr;
}

// 设置当前北京时间
export function setNowBeijing(inputId) {
    const input = document.getElementById(inputId);
    if (!input) return;

    const now = new Date();
    const utcMs = now.getTime() + now.getTimezoneOffset() * 60000;
    const beijing = new Date(utcMs + 8 * 3600000);
    const timeStr = String(beijing.getHours()).padStart(2, '0') + ':' +
        String(beijing.getMinutes()).padStart(2, '0');
    input.value = timeStr;
}

// 自动填充日期时间
export function setupAutoDateTime() {
    // 添加表单的日期时间
    setTodayDate('addQsoDate');
    setNowBeijing('addTimeOn');

    // 今天按钮
    const todayBtn = document.getElementById('addTodayBtn');
    if (todayBtn) {
        todayBtn.addEventListener('click', () => setTodayDate('addQsoDate'));
    }

    // 现在按钮
    const nowBtn = document.getElementById('addNowBtn');
    if (nowBtn) {
        nowBtn.addEventListener('click', () => setNowBeijing('addTimeOn'));
    }

    // EYEBALL 类型切换时自动填充日期
    const typeSelect = document.getElementById('addQsoTypeSelect');
    if (typeSelect) {
        typeSelect.addEventListener('change', function() {
            if (this.value === 'EYEBALL') {
                setTodayDate('addQsoDate');
                const timeInput = document.getElementById('addTimeOn');
                if (timeInput) timeInput.value = '';
            }
        });
    }
}

// 收集表单数据
export function collectFormData(form, qsoType) {
    const data = {};

    // 通用字段
    data.call = form.querySelector('[name="call"]')?.value?.trim() || '';
    data.qso_date = form.querySelector('[name="qso_date"]')?.value?.replace(/-/g, '') || '';
    data.time_on = form.querySelector('[name="time_on"]')?.value?.replace(/:/g, '') || '';
    data.qso_type = qsoType || 'NORMAL';
    data.qsl_status = form.querySelector('[name="qsl_status"]')?.value || '未发送';
    data.comment = form.querySelector('[name="comment"]')?.value?.trim() || '';
    data.is_sk = form.querySelector('[name="is_sk"]')?.checked ? 1 : 0;

    // 根据类型收集特定字段
    if (qsoType === 'SAT') {
        data.sat_name = form.querySelector('[name="sat_name"]')?.value?.trim() || '';
        data.tx_freq = form.querySelector('[name="tx_freq"]')?.value?.trim() || '';
        data.rx_freq = form.querySelector('[name="rx_freq"]')?.value?.trim() || '';
        data.mode = form.querySelector('#addSatFields [name="mode"]')?.value || '';
        data.rst_sent = form.querySelector('#addSatFields [name="rst_sent"]')?.value?.trim() || '';
        data.rst_rcvd = form.querySelector('#addSatFields [name="rst_rcvd"]')?.value?.trim() || '';
    } else if (qsoType === 'REP') {
        data.sat_name = form.querySelector('#addRepFields [name="sat_name"]')?.value?.trim() || '';
        data.tx_freq = form.querySelector('#addRepFields [name="tx_freq"]')?.value?.trim() || '';
        data.rx_freq = form.querySelector('#addRepFields [name="rx_freq"]')?.value?.trim() || '';
        data.mode = form.querySelector('#addRepFields [name="mode"]')?.value || '';
        data.rst_sent = form.querySelector('#addRepFields [name="rst_sent"]')?.value?.trim() || '';
        data.rst_rcvd = form.querySelector('#addRepFields [name="rst_rcvd"]')?.value?.trim() || '';
    } else if (qsoType === 'EYEBALL') {
        data.qth = form.querySelector('[name="qth"]')?.value?.trim() || '';
        data.mode = '';
        data.rst_sent = '';
        data.rst_rcvd = '';
    } else {
        data.freq = form.querySelector('[name="freq"]')?.value?.trim() || '';
        data.mode = form.querySelector('#addNormalFields [name="mode"]')?.value || '';
        data.rst_sent = form.querySelector('#addNormalFields [name="rst_sent"]')?.value?.trim() || '';
        data.rst_rcvd = form.querySelector('#addNormalFields [name="rst_rcvd"]')?.value?.trim() || '';
    }

    return data;
}

// 验证表单数据
export function validateFormData(data) {
    if (!data.call) {
        return '请输入呼号';
    }

    if (data.qso_type !== 'EYEBALL') {
        if (!data.qso_date) {
            return '请输入日期';
        }
    }

    if (data.qso_type === 'NORMAL' && !data.freq) {
        return '请输入频率';
    }

    return null;
}

// 提交 QSO 记录
export async function submitQSO(data, isEdit = false, logId = null) {
    const url = isEdit ? `/api/admin/logs/${logId}` : '/api/admin/logs';
    const method = isEdit ? 'PUT' : 'POST';

    try {
        const resp = await fetch(url, {
            method,
            headers: csrfHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify(data)
        });

        const result = await resp.json();

        if (resp.status === 409) {
            // 重复记录
            return { ok: false, duplicate: true, detail: result.detail };
        }

        if (resp.ok) {
            return { ok: true };
        } else {
            return { ok: false, detail: result.detail || '操作失败' };
        }
    } catch (err) {
        return { ok: false, detail: '请求失败' };
    }
}

// 初始化添加表单
export function initAddForm(onSuccess) {
    const addForm = document.getElementById('addForm');
    if (!addForm) return;

    // 设置类型切换
    setupFormTypeToggle('add');

    // 设置频率提示
    setupFreqHint();

    // 设置呼号自动转大写
    setupCallUppercase();

    // 设置自动日期时间
    setupAutoDateTime();

    addForm.addEventListener('submit', async function(e) {
        e.preventDefault();

        const qsoType = document.getElementById('addQsoTypeSelect')?.value || 'NORMAL';
        const data = collectFormData(this, qsoType);
        const error = validateFormData(data);

        if (error) {
            showToast(error, 'error');
            return;
        }

        const btn = this.querySelector('button[type="submit"]');
        setLoading(btn, true);

        const result = await submitQSO(data);

        setLoading(btn, false);

        if (result.ok) {
            showToast('QSO 记录已添加', 'success');
            this.reset();
            setupFormTypeToggle('add');
            setTodayDate('addQsoDate');
            setNowBeijing('addTimeOn');
            onSuccess();
        } else if (result.duplicate) {
            if (confirm('检测到重复记录，是否仍然添加？')) {
                data.force = true;
                setLoading(btn, true);
                const retryResult = await submitQSO(data);
                setLoading(btn, false);

                if (retryResult.ok) {
                    showToast('QSO 记录已添加', 'success');
                    this.reset();
                    setupFormTypeToggle('add');
                    setTodayDate('addQsoDate');
                    setNowBeijing('addTimeOn');
                    onSuccess();
                } else {
                    showToast(retryResult.detail || '添加失败', 'error');
                }
            }
        } else {
            showToast(result.detail || '添加失败', 'error');
        }
    });
}
