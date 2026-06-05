/**
 * 通用分页渲染器
 */

// 渲染分页控件
export function renderPagination(containerId, infoId, data, goFn) {
    const container = document.getElementById(containerId);
    const info = document.getElementById(infoId);

    if (!container || !info) return;

    const totalPages = Math.ceil(data.total / data.page_size);

    if (data.total === 0) {
        container.classList.add('hidden');
        return;
    }

    container.classList.remove('hidden');
    info.textContent = `共 ${data.total} 条，第 ${data.page}/${totalPages} 页`;

    const btnContainer = container.querySelector('.flex.gap-2');
    if (!btnContainer) return;

    let html = '';

    // 上一页按钮
    if (data.page > 1) {
        html += `<button onclick="${goFn}(${data.page - 1})" class="px-3 py-1 border rounded hover:bg-gray-50">上一页</button>`;
    }

    // 页码按钮（显示最多 7 页）
    let startPage = Math.max(1, data.page - 3);
    let endPage = Math.min(totalPages, startPage + 6);
    if (endPage - startPage < 6) {
        startPage = Math.max(1, endPage - 6);
    }

    for (let i = startPage; i <= endPage; i++) {
        const isActive = i === data.page;
        html += `<button onclick="${goFn}(${i})" class="px-3 py-1 border rounded ${isActive ? 'bg-blue-600 text-white' : 'hover:bg-gray-50'}">${i}</button>`;
    }

    // 下一页按钮
    if (data.page < totalPages) {
        html += `<button onclick="${goFn}(${data.page + 1})" class="px-3 py-1 border rounded hover:bg-gray-50">下一页</button>`;
    }

    btnContainer.innerHTML = html;
}
