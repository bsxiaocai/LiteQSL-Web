/**
 * 双时钟组件（北京时间 + UTC）
 */

export function updateClocks() {
    const clocksEl = document.getElementById('clocks');
    if (!clocksEl) return;

    const now = new Date();
    const fmt = (tz) => {
        const d = new Date(now.toLocaleString('en-US', { timeZone: tz }));
        return [d.getHours(), d.getMinutes(), d.getSeconds()]
            .map(n => String(n).padStart(2, '0'))
            .join(':');
    };

    clocksEl.textContent = `北京时间 ${fmt('Asia/Shanghai')}  |  UTC ${fmt('UTC')}`;
}

// 启动时钟（每秒更新）
export function startClock() {
    updateClocks();
    setInterval(updateClocks, 1000);
}
