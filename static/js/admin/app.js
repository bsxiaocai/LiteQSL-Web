/**
 * 管理后台主入口
 */

import { showToast, loadAppVersion } from '../common/index.js';
import { checkLogin, initLoginForm, initLogoutButton, initChangePasswordModal, fetchCsrfToken } from './auth.js';
import { loadLogs, initFilters, registerGlobalFunctions } from './qso-table.js';
import { initAddForm } from './qso-form.js';
import { initEditModal, setupEditDateTimeButtons } from './edit-modal.js';
import { initImportExport } from './import-export.js';
import { initBackup } from './backup.js';
import { initSettings } from './settings.js';
import { startClock } from '../common/clock.js';
import { initTabs } from './stats.js';

// 显示管理后台
async function showAdmin() {
    document.getElementById('loginPage').classList.add('hidden');
    document.getElementById('adminPage').classList.remove('hidden');

    // 首次登录未完成时：强制先修改用户名和密码。
    // 此时不加载任何管理数据、也不初始化其他模块，确保修改完成前无法进行任何操作。
    if (await needFirstLoginChange()) {
        return;
    }

    // 初始化各个模块
    loadLogs(1, {});
    initFilters();
    initAddForm(() => loadLogs(1, {}));
    initEditModal();
    setupEditDateTimeButtons();
    initImportExport();
    initBackup();
    initSettings();
    initTabs();
}

// 查询是否需要强制修改首次登录凭据。
// 需要时立即弹出不可跳过的弹窗并返回 true；查询失败时返回 false（后端仍会拦截操作）。
async function needFirstLoginChange() {
    try {
        const resp = await fetch('/api/admin/first-login-status');
        if (resp.ok) {
            const data = await resp.json();
            if (data.first_login) {
                openFirstLoginModal();
                return true;
            }
        }
    } catch (err) {
        console.error('Failed to check first login:', err);
    }
    return false;
}

// 打开首次登录弹窗
function openFirstLoginModal() {
    const modal = document.getElementById('firstLoginModal');
    if (modal) {
        modal.classList.remove('hidden');
        document.body.style.overflow = 'hidden';
        document.body.classList.add('modal-open');
    }
}

// 关闭首次登录弹窗
function closeFirstLoginModal() {
    const modal = document.getElementById('firstLoginModal');
    if (modal) {
        modal.classList.add('hidden');
        document.body.style.overflow = '';
        document.body.classList.remove('modal-open');
    }
}

// 初始化首次登录表单
function initFirstLoginForm() {
    const form = document.getElementById('firstLoginForm');
    if (!form) return;

    form.addEventListener('submit', async function(e) {
        e.preventDefault();

        const oldPassword = document.getElementById('flOldPassword').value;
        const newUsername = document.getElementById('flNewUsername').value;
        const confirmUsername = document.getElementById('flConfirmUsername').value;
        const newPassword = document.getElementById('flNewPassword').value;
        const confirmPassword = document.getElementById('flConfirmPassword').value;
        const errEl = document.getElementById('flError');

        errEl.classList.add('hidden');

        if (newUsername !== confirmUsername) {
            errEl.textContent = '两次输入的新用户名不一致';
            errEl.classList.remove('hidden');
            return;
        }

        if (newPassword !== confirmPassword) {
            errEl.textContent = '两次输入的新密码不一致';
            errEl.classList.remove('hidden');
            return;
        }

        if (newUsername.length < 5) {
            errEl.textContent = '用户名长度至少为 5 位';
            errEl.classList.remove('hidden');
            return;
        }

        if (newPassword.length < 8) {
            errEl.textContent = '密码长度至少为 8 位';
            errEl.classList.remove('hidden');
            return;
        }

        const btn = this.querySelector('button[type="submit"]');
        btn.disabled = true;
        btn.textContent = '处理中...';

        try {
            const resp = await fetch('/api/admin/complete-first-login', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-CSRF-Token': await fetchCsrfToken()
                },
                body: JSON.stringify({
                    old_password: oldPassword,
                    new_username: newUsername,
                    new_password: newPassword,
                    confirm_username: confirmUsername,
                    confirm_password: confirmPassword
                })
            });

            const data = await resp.json();

            if (resp.ok) {
                showToast('凭据已更新，请重新登录', 'success');
                closeFirstLoginModal();
                setTimeout(() => location.reload(), 1500);
            } else {
                errEl.textContent = data.detail || '操作失败';
                errEl.classList.remove('hidden');
            }
        } catch (err) {
            errEl.textContent = '请求失败';
            errEl.classList.remove('hidden');
        }

        btn.disabled = false;
        btn.textContent = '确认修改';
    });
}

// 密码强度指示器
function initPasswordStrength() {
    const passwordInput = document.getElementById('flNewPassword');
    const strengthBar = document.getElementById('pwStrength');

    if (!passwordInput || !strengthBar) return;

    passwordInput.addEventListener('input', function() {
        const password = this.value;
        let strength = 0;

        if (password.length >= 8) strength++;
        if (/[A-Z]/.test(password)) strength++;
        if (/[a-z]/.test(password)) strength++;
        if (/[0-9]/.test(password)) strength++;
        if (/[^A-Za-z0-9]/.test(password)) strength++;

        const percentage = (strength / 5) * 100;
        const colors = ['bg-red-500', 'bg-orange-500', 'bg-yellow-500', 'bg-blue-500', 'bg-green-500'];

        strengthBar.style.width = percentage + '%';
        strengthBar.className = `h-2 rounded-full transition-all ${colors[strength - 1] || 'bg-gray-200'}`;
    });
}

// 初始化
export async function init() {
    // 注册全局函数
    registerGlobalFunctions();

    // 启动时钟
    startClock();

    // 加载页脚版本号
    loadAppVersion();

    // 初始化首次登录表单
    initFirstLoginForm();

    // 供其他模块在遇到 403（未完成首次登录）时重新弹出强制修改弹窗
    window.openFirstLoginModal = openFirstLoginModal;

    // 初始化密码强度指示器
    initPasswordStrength();

    // 检查登录状态
    const loginStatus = await checkLogin();

    if (loginStatus.logged_in) {
        // 已登录，获取 CSRF token 后显示管理后台
        await fetchCsrfToken();
        showAdmin();
    } else {
        // 未登录，显示登录表单
        document.getElementById('loginPage').classList.remove('hidden');
        document.getElementById('adminPage').classList.add('hidden');
    }

    // 初始化登录表单
    initLoginForm(async (firstLogin) => {
        // 登录成功后，获取 CSRF token
        await fetchCsrfToken();
        // 显示管理后台
        showAdmin();
    });

    // 初始化登出按钮
    initLogoutButton(() => {
        document.getElementById('loginPage').classList.remove('hidden');
        document.getElementById('adminPage').classList.add('hidden');
    });

    // 初始化修改密码模态框
    initChangePasswordModal();
}
