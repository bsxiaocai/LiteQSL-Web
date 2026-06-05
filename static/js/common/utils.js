/**
 * 通用工具函数
 */

// XSS 防护：转义 HTML 特殊字符
export function escapeHtml(str) {
    if (!str) return '';
    return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

// Toast 通知系统
export function showToast(message, type = 'info') {
    const container = document.getElementById('toastContainer');
    if (!container) return;

    const colors = {
        success: 'bg-green-50 border-green-400 text-green-800',
        error: 'bg-red-50 border-red-400 text-red-800',
        warning: 'bg-yellow-50 border-yellow-400 text-yellow-800',
        info: 'bg-blue-50 border-blue-400 text-blue-800',
    };
    const icons = { success: '✓', error: '✗', warning: '⚠', info: 'ℹ' };

    const toast = document.createElement('div');
    toast.className = `flex items-center gap-3 px-4 py-3 rounded-lg border-l-4 shadow-lg ${colors[type]} transform translate-x-full opacity-0 transition-all duration-300`;
    toast.innerHTML = `<span class="text-lg">${icons[type]}</span><span class="flex-1 text-sm">${escapeHtml(message)}</span><button onclick="this.parentElement.remove()" class="text-current opacity-50 hover:opacity-100 text-lg leading-none">&times;</button>`;
    container.appendChild(toast);

    requestAnimationFrame(() => toast.classList.remove('translate-x-full', 'opacity-0'));
    setTimeout(() => {
        toast.classList.add('translate-x-full', 'opacity-0');
        setTimeout(() => toast.remove(), 300);
    }, 3500);
}

// 按钮加载状态
export function setLoading(buttonEl, loading) {
    if (!buttonEl) return;
    if (loading) {
        buttonEl.disabled = true;
        buttonEl.dataset.originalText = buttonEl.textContent;
        buttonEl.textContent = '处理中...';
        buttonEl.classList.add('opacity-70', 'cursor-not-allowed');
    } else {
        buttonEl.disabled = false;
        buttonEl.textContent = buttonEl.dataset.originalText || buttonEl.textContent;
        buttonEl.classList.remove('opacity-70', 'cursor-not-allowed');
    }
}

// QSL 状态颜色映射
export function statusColor(s) {
    const map = {
        '已发送': 'bg-green-100 text-green-800',
        '未发送': 'bg-yellow-100 text-yellow-800',
        '电子确认': 'bg-blue-100 text-blue-800',
        '无需发送': 'bg-gray-100 text-gray-600',
        '无法考证': 'bg-red-100 text-red-800'
    };
    return map[s] || 'bg-gray-100 text-gray-600';
}
