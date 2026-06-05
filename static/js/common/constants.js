/**
 * 常量定义
 */

// QSO 类型显示名称
export const QSO_TYPE_LABELS = {
    'NORMAL': '一般通联',
    'SAT': '卫星通联',
    'REP': '中继通联',
    'EYEBALL': 'Eyeball QSO'
};

// QSO 类型图标
export const QSO_TYPE_ICONS = {
    'NORMAL': '',
    'SAT': '🛰',
    'REP': '📡',
    'EYEBALL': '👀'
};

// QSL 状态选项
export const QSL_STATUSES = ['无法考证', '未发送', '已发送', '无需发送', '电子确认'];

// QSO 类型选项
export const QSO_TYPES = ['NORMAL', 'SAT', 'REP', 'EYEBALL'];

// 频率范围显示映射
export const BAND_FREQ = {
    '160m': '1.800-2.000',
    '80m': '3.500-3.900',
    '60m': '5.300-5.400',
    '40m': '7.000-7.300',
    '30m': '10.100-10.150',
    '20m': '14.000-14.350',
    '17m': '18.068-18.168',
    '15m': '21.000-21.450',
    '12m': '24.890-24.990',
    '10m': '28.000-29.700',
    '6m': '50.000-54.000',
    '2m': '144.000-148.000',
    '70cm': '430.000-440.000',
    '23cm': '1240.000-1300.000'
};

// 波段列表
export const BANDS = ['160m', '80m', '60m', '40m', '30m', '20m', '17m', '15m', '12m', '10m', '6m', '2m', '70cm', '23cm'];

// 模式列表
export const MODES = ['SSB', 'CW', 'FT8', 'FT4', 'FM', 'AM', 'PSK31', 'RTTY', 'JS8', 'SSTV', 'DIGITAL'];
