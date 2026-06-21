/**
 * 格式化函数
 */

import { escapeHtml } from './utils.js';
import { QSO_TYPE_LABELS, QSO_TYPE_ICONS } from './constants.js';

// 频率转波段（前端版本，用于显示）
export function freqToBand(freq) {
    const f = parseFloat(freq);
    if (isNaN(f)) return '';
    if (f >= 1.8 && f < 2.0) return '160m';
    if (f >= 3.5 && f < 4.0) return '80m';
    if (f >= 5.3 && f < 5.4) return '60m';
    if (f >= 7.0 && f < 7.3) return '40m';
    if (f >= 10.1 && f < 10.15) return '30m';
    if (f >= 14.0 && f < 14.35) return '20m';
    if (f >= 18.068 && f < 18.168) return '17m';
    if (f >= 21.0 && f < 21.45) return '15m';
    if (f >= 24.89 && f < 25.0) return '12m';
    if (f >= 28.0 && f < 29.7) return '10m';
    if (f >= 50.0 && f < 54.0) return '6m';
    if (f >= 144.0 && f < 148.0) return '2m';
    if (f >= 430.0 && f < 440.0) return '70cm';
    if (f >= 1240.0 && f < 1300.0) return '23cm';
    return '';
}

// 格式化日期：YYYYMMDD -> YYYY-MM-DD
export function formatDate(dateStr) {
    if (!dateStr || dateStr.length < 8) return dateStr || '-';
    return `${dateStr.substring(0, 4)}-${dateStr.substring(4, 6)}-${dateStr.substring(6, 8)}`;
}

// 格式化时间：HHMM -> HH:MM
export function formatTime(timeStr) {
    if (!timeStr || timeStr.length < 4) return timeStr || '-';
    return `${timeStr.substring(0, 2)}:${timeStr.substring(2, 4)}`;
}

// 将数据库中的 UTC 日期时间转换为指定表格显示时区
export function formatQsoDateTime(qsoDate, timeOn, timezone = 'UTC') {
    if (!qsoDate || qsoDate.length < 8 || !timeOn || timeOn.length < 4) {
        return {
            date: formatDate(qsoDate),
            time: formatTime(timeOn)
        };
    }

    const year = Number(qsoDate.substring(0, 4));
    const month = Number(qsoDate.substring(4, 6)) - 1;
    const day = Number(qsoDate.substring(6, 8));
    const hour = Number(timeOn.substring(0, 2));
    const minute = Number(timeOn.substring(2, 4));
    const offsetMs = timezone === 'Asia/Shanghai' ? 8 * 60 * 60 * 1000 : 0;
    const value = new Date(Date.UTC(year, month, day, hour, minute) + offsetMs);

    return {
        date: [
            value.getUTCFullYear(),
            String(value.getUTCMonth() + 1).padStart(2, '0'),
            String(value.getUTCDate()).padStart(2, '0')
        ].join('-'),
        time: [
            String(value.getUTCHours()).padStart(2, '0'),
            String(value.getUTCMinutes()).padStart(2, '0')
        ].join(':')
    };
}

// 格式化类型标签
export function formatTypeBadge(qsoType, location) {
    const type = qsoType || 'NORMAL';
    const label = QSO_TYPE_LABELS[type] || '一般通联';
    const icon = QSO_TYPE_ICONS[type] || '';

    if (type === 'SAT') {
        return `<span class="px-2 py-1 rounded-full text-xs bg-purple-100 text-purple-700">${icon} ${escapeHtml(label)}</span>`;
    } else if (type === 'REP') {
        return `<span class="px-2 py-1 rounded-full text-xs bg-orange-100 text-orange-700">${icon} ${escapeHtml(label)}</span>`;
    } else if (type === 'EYEBALL') {
        const titleAttr = location ? ` title="${escapeHtml(location)}"` : '';
        return `<span class="px-2 py-1 rounded-full text-xs bg-teal-100 text-teal-700"${titleAttr}>${icon} ${escapeHtml(label)}</span>`;
    }
    return `<span class="px-2 py-1 rounded-full text-xs bg-gray-100 text-gray-600">${escapeHtml(label)}</span>`;
}

// 格式化频率单元格（考虑 QSO 类型）
export function formatFreqCell(log) {
    const qsoType = log.qso_type || 'NORMAL';

    if (qsoType === 'SAT') {
        let html = '';
        if (log.sat_name) {
            html += `<div class="text-purple-600 text-xs font-medium">🛰 ${escapeHtml(log.sat_name)}</div>`;
        }
        if (log.rx_freq) {
            const rxBand = freqToBand(log.rx_freq);
            const rxBandStr = rxBand ? ` (${escapeHtml(rxBand)})` : '';
            html += `<div class="text-gray-600 text-xs">${escapeHtml(log.rx_freq)} MHz${rxBandStr} ↓</div>`;
        }
        if (log.tx_freq) {
            const txBand = freqToBand(log.tx_freq);
            const txBandStr = txBand ? ` (${escapeHtml(txBand)})` : '';
            html += `<div class="text-gray-600 text-xs">${escapeHtml(log.tx_freq)} MHz${txBandStr} ↑</div>`;
        }
        if (!log.sat_name && !log.rx_freq && !log.tx_freq) {
            if (log.freq) {
                const band = log.band || freqToBand(log.freq);
                const bandStr = band ? ` (${escapeHtml(band)})` : '';
                html = `<div class="text-gray-600 text-xs">${escapeHtml(log.freq)} MHz${bandStr}</div>`;
            } else if (log.band) {
                html = `<div class="text-gray-600 text-xs">${escapeHtml(log.band)}</div>`;
            } else {
                html = '-';
            }
        }
        return html;
    } else if (qsoType === 'REP') {
        let html = '';
        if (log.sat_name) {
            html += `<div class="text-orange-600 text-xs font-medium">📡 ${escapeHtml(log.sat_name)}</div>`;
        }
        if (log.rx_freq) {
            const rxBand = freqToBand(log.rx_freq);
            const rxBandStr = rxBand ? ` (${escapeHtml(rxBand)})` : '';
            html += `<div class="text-gray-600 text-xs">${escapeHtml(log.rx_freq)} MHz${rxBandStr} ↓</div>`;
        }
        if (log.tx_freq) {
            const txBand = freqToBand(log.tx_freq);
            const txBandStr = txBand ? ` (${escapeHtml(txBand)})` : '';
            html += `<div class="text-gray-600 text-xs">${escapeHtml(log.tx_freq)} MHz${txBandStr} ↑</div>`;
        }
        if (!log.sat_name && !log.rx_freq && !log.tx_freq) {
            if (log.freq) {
                const band = log.band || freqToBand(log.freq);
                const bandStr = band ? ` (${escapeHtml(band)})` : '';
                html = `<div class="text-gray-600 text-xs">${escapeHtml(log.freq)} MHz${bandStr}</div>`;
            } else if (log.band) {
                html = `<div class="text-gray-600 text-xs">${escapeHtml(log.band)}</div>`;
            } else {
                html = '-';
            }
        }
        return html;
    } else if (qsoType === 'EYEBALL') {
        if (log.qth) {
            return `<span class="text-gray-500 text-xs">${escapeHtml(log.qth)}</span>`;
        }
        return '<span class="text-gray-400 text-xs">-</span>';
    }

    // 普通通联
    const freq = log.freq || '';
    const band = log.band || (freq ? freqToBand(freq) : '');
    let html = '';
    if (freq) {
        html += `<span class="text-gray-700">${escapeHtml(freq)} MHz</span>`;
    }
    if (band) {
        html += `<span class="text-gray-500 text-xs ml-1">(${escapeHtml(band)})</span>`;
    }
    if (!freq && !band) {
        html = '-';
    }
    return html;
}
