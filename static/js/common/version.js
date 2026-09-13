/**
 * 应用版本号显示
 *
 * 从后端 /health 读取版本号并注入页脚元素，避免在 HTML 中硬编码版本号导致过期。
 */

export async function loadAppVersion(elementId = 'appVersion') {
    const el = document.getElementById(elementId);
    if (!el) return;

    try {
        const resp = await fetch('/health');
        if (!resp.ok) return;
        const data = await resp.json();
        if (data && data.version) {
            el.textContent = `v${data.version}`;
        }
    } catch (err) {
        // 版本号非关键信息，读取失败时静默忽略
        console.debug('Failed to load app version:', err);
    }
}
