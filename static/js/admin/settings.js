/**
 * 系统设置管理
 */

import { showToast, setLoading } from '../common/index.js';
import { csrfHeaders } from './auth.js';

// 加载设置
async function loadSettings() {
    try {
        const resp = await fetch('/api/admin/settings');
        const data = await resp.json();

        if (data.settings) {
            const callsignInput = document.getElementById('settingCallsign');
            const stationNameInput = document.getElementById('settingStationName');
            const visitorTimezoneInput = document.getElementById('settingVisitorTimezone');

            if (callsignInput) {
                callsignInput.value = data.settings.callsign || '';
            }
            if (stationNameInput) {
                stationNameInput.value = data.settings.station_name || '';
            }
            if (visitorTimezoneInput) {
                visitorTimezoneInput.value = data.settings.visitor_timezone || 'Asia/Shanghai';
            }
        }
    } catch (err) {
        console.error('Failed to load settings:', err);
    }
}

// 保存设置
async function saveSettings(callsign, stationName, visitorTimezone) {
    try {
        const resp = await fetch('/api/admin/settings', {
            method: 'PUT',
            headers: csrfHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify({
                callsign: callsign,
                station_name: stationName || 'QSL & Log Management',
                visitor_timezone: visitorTimezone
            })
        });

        const data = await resp.json();

        if (data.ok) {
            showToast('设置已保存', 'success');
            // 更新页面标题
            document.title = `管理后台 - ${callsign} ${stationName || 'QSL & Log Management'}`;
            return true;
        } else {
            showToast(data.detail || '保存失败', 'error');
            return false;
        }
    } catch (err) {
        showToast('请求失败', 'error');
        return false;
    }
}

// 初始化设置功能
export function initSettings() {
    const settingsForm = document.getElementById('settingsForm');

    if (settingsForm) {
        settingsForm.addEventListener('submit', async function(e) {
            e.preventDefault();

            const callsign = document.getElementById('settingCallsign').value.trim();
            const stationName = document.getElementById('settingStationName').value.trim();
            const visitorTimezone = document.getElementById('settingVisitorTimezone').value;

            if (!callsign) {
                showToast('请输入操作员呼号', 'error');
                return;
            }

            const btn = this.querySelector('button[type="submit"]');
            setLoading(btn, true);

            await saveSettings(callsign, stationName, visitorTimezone);

            setLoading(btn, false);
        });
    }

    // 加载设置
    loadSettings();
}
