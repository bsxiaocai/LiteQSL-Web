/**
 * 登录/登出/CSRF 管理
 */

import { showToast, setLoading } from '../common/index.js';

let csrfToken = null;

// 获取 CSRF Token
export async function fetchCsrfToken() {
    try {
        const resp = await fetch('/api/admin/csrf-token');
        const data = await resp.json();
        csrfToken = data.csrf_token;
        return csrfToken;
    } catch (err) {
        console.error('Failed to fetch CSRF token:', err);
        return null;
    }
}

// 构造带 CSRF Token 的请求头
export function csrfHeaders(extra = {}) {
    const headers = { ...extra };
    if (csrfToken) {
        headers['X-CSRF-Token'] = csrfToken;
    }
    return headers;
}

// 检查登录状态
export async function checkLogin() {
    try {
        const resp = await fetch('/api/admin/check');
        const data = await resp.json();
        return data;
    } catch (err) {
        console.error('Failed to check login:', err);
        return { logged_in: false };
    }
}

// 登录
export async function login(username, password) {
    try {
        const resp = await fetch('/api/admin/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password })
        });
        const data = await resp.json();

        if (resp.ok) {
            await fetchCsrfToken();
            return { ok: true, first_login: data.first_login };
        } else {
            return { ok: false, detail: data.detail || '登录失败' };
        }
    } catch (err) {
        return { ok: false, detail: '请求失败' };
    }
}

// 登出
export async function logout() {
    try {
        await fetch('/api/admin/logout', { method: 'POST' });
        csrfToken = null;
        return true;
    } catch (err) {
        console.error('Failed to logout:', err);
        return false;
    }
}

// 修改密码
export async function changePassword(oldPassword, newPassword) {
    try {
        const resp = await fetch('/api/admin/change-password', {
            method: 'POST',
            headers: csrfHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify({ old_password: oldPassword, new_password: newPassword })
        });
        const data = await resp.json();

        if (resp.ok) {
            return { ok: true };
        } else {
            return { ok: false, detail: data.detail || '修改失败' };
        }
    } catch (err) {
        return { ok: false, detail: '请求失败' };
    }
}

// 初始化登录表单
export function initLoginForm(onSuccess) {
    const loginForm = document.getElementById('loginForm');
    if (!loginForm) return;

    loginForm.addEventListener('submit', async function(e) {
        e.preventDefault();

        const username = document.getElementById('loginUsername').value;
        const password = document.getElementById('loginPassword').value;
        const errEl = document.getElementById('loginError');
        const btn = this.querySelector('button[type="submit"]');

        errEl.classList.add('hidden');
        setLoading(btn, true);

        const result = await login(username, password);

        setLoading(btn, false);

        if (result.ok) {
            onSuccess(result.first_login);
        } else {
            errEl.textContent = result.detail;
            errEl.classList.remove('hidden');
        }
    });
}

// 初始化登出按钮
export function initLogoutButton(onLogout) {
    const logoutBtn = document.getElementById('logoutBtn');
    if (!logoutBtn) return;

    logoutBtn.addEventListener('click', async function() {
        if (await logout()) {
            onLogout();
        }
    });
}

// 初始化修改密码模态框
export function initChangePasswordModal() {
    const changePwdBtn = document.getElementById('changePwdBtn');
    const pwdModal = document.getElementById('pwdModal');
    const pwdModalClose = document.getElementById('pwdModalClose');
    const changePwdForm = document.getElementById('changePwdForm');

    if (!changePwdBtn || !pwdModal) return;

    changePwdBtn.addEventListener('click', function() {
        pwdModal.classList.remove('hidden');
    });

    pwdModalClose.addEventListener('click', function() {
        pwdModal.classList.add('hidden');
    });

    changePwdForm.addEventListener('submit', async function(e) {
        e.preventDefault();

        const oldPassword = document.getElementById('oldPassword').value;
        const newPassword = document.getElementById('newPassword').value;
        const confirmPassword = document.getElementById('confirmPassword').value;
        const errEl = document.getElementById('pwdError');

        errEl.classList.add('hidden');

        if (newPassword !== confirmPassword) {
            errEl.textContent = '两次输入的新密码不一致';
            errEl.classList.remove('hidden');
            return;
        }

        if (newPassword.length < 8) {
            errEl.textContent = '密码长度至少为 8 位';
            errEl.classList.remove('hidden');
            return;
        }

        let cats = 0;
        if (/[A-Z]/.test(newPassword)) cats++;
        if (/[a-z]/.test(newPassword)) cats++;
        if (/[0-9]/.test(newPassword)) cats++;
        if (/[^A-Za-z0-9]/.test(newPassword)) cats++;

        if (cats < 3) {
            errEl.textContent = '密码需包含大写字母、小写字母、数字、符号中的至少三类';
            errEl.classList.remove('hidden');
            return;
        }

        const btn = this.querySelector('button[type="submit"]');
        setLoading(btn, true);

        const result = await changePassword(oldPassword, newPassword);

        setLoading(btn, false);

        if (result.ok) {
            showToast('密码修改成功，即将重新登录', 'success');
            setTimeout(() => location.reload(), 1500);
        } else {
            errEl.textContent = result.detail;
            errEl.classList.remove('hidden');
        }
    });
}
