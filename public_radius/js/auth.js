let currentAdmin = null;
let radiusAdminsCache = [];
let licenseState = { valid: false, router_connected: false };
let radiusRemoteBaseURL = "";

function escapeHtml(value) {
    if (!value) return '';
    return String(value)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}

async function apiFetch(url, options = {}) {
    const opts = Object.assign({ credentials: 'same-origin' }, options);
    opts.headers = Object.assign({ 'Content-Type': 'application/json' }, options.headers || {});
    const res = await fetch(url, opts);
    if (res.status === 401) {
        try { const data = await res.clone().json(); if (data && data.auth_required) window.location.href = '/radius/login.html'; } catch (e) {}
    }
    return res;
}

async function loadCurrentAdmin() {
    try {
        const res = await fetch('/radius/api/auth/me', { credentials: 'same-origin' });
        if (!res.ok) return null;
        currentAdmin = await res.json();
        
        // Update Name
        const el = document.getElementById('admin-badge');
        if (el) el.textContent = currentAdmin.name || currentAdmin.username;

        // Update Balance
        const balEl = document.getElementById('balance-badge');
        if (balEl) {
            balEl.textContent = (currentAdmin.balance || 0).toLocaleString() + ' د.ع';
            balEl.style.display = 'inline-block';
        }

        // Update Role
        const roleEl = document.getElementById('role-badge');
        if (roleEl) {
            roleEl.textContent = currentAdmin.role === 'superadmin' ? 'مدير أساسي' : 'وكيل فرعي';
            roleEl.style.display = 'inline-block';
        }

        // Enforce Tab Visibility
        document.querySelectorAll('[data-superadmin-only="true"]').forEach(tab => {
            if (currentAdmin.role !== 'superadmin') {
                tab.style.display = 'none';
            } else {
                tab.style.display = '';
            }
        });

        // Superadmin only sections
        const superAdminElements = document.querySelectorAll('.superadmin-only');
        superAdminElements.forEach(el => {
            el.style.display = currentAdmin.role === 'superadmin' ? 'block' : 'none';
        });

        loadAdmins(); // Load for all roles to see sub-agents if any

        const form = document.getElementById('profile-form');
        if (form) {
            form.querySelector('[name="username"]').value = currentAdmin.username;
            form.querySelector('[name="name"]').value = currentAdmin.name || '';
            form.querySelector('[name="email"]').value = currentAdmin.email || '';
        }
        return currentAdmin;
    } catch (e) { return null; }
}

let isFreshInstall = false;
let setupStateCache = null;

async function checkFreshInstallStatus() {
    try {
        const res = await apiFetch('/radius/api/setup/status');
        if (!res.ok) return false;
        const data = await res.json();
        setupStateCache = data;
        isFreshInstall = !!data.is_fresh_install;

        const obGate = document.getElementById('onboarding-gate');
        const licGate = document.getElementById('license-gate');
        const mainArea = document.getElementById('main-area');

        if (isFreshInstall) {
            if (obGate) obGate.style.display = 'block';
            if (licGate) licGate.style.display = 'none';
            if (mainArea) mainArea.style.display = 'none';
            return true;
        } else {
            if (obGate) obGate.style.display = 'none';
            return false;
        }
    } catch (e) {
        return false;
    }
}

async function loadLicenseStatus() {
    try {
        const isFresh = await checkFreshInstallStatus();
        const res = await apiFetch('/radius/api/license/status');
        const data = await res.json();
        licenseState = data;
        if (!isFresh) {
            renderLicenseGate(data);
        }
        return data;
    } catch (e) { return null; }
}

function renderLicenseGate(data) {
    if (isFreshInstall) return; // Don't show license gate while in onboarding
    const gate = document.getElementById('license-gate');
    const main = document.getElementById('main-area');
    const routerBox = document.getElementById('router-setup-box');
    
    let statusBadge = '';
    if (data.status === 'suspended') {
        statusBadge = '<span style="background:#78350f; color:#fef3c7; padding:4px 10px; border-radius:6px; font-weight:bold;">⏸️ الترخيص مجمّد / موقوف</span>';
    } else if (data.valid) {
        statusBadge = '<span style="background:#14532d; color:#86efac; padding:4px 10px; border-radius:6px; font-weight:bold;">🟢 الترخيص مفعّل وصالح</span>';
    } else {
        statusBadge = '<span style="background:#7f1d1d; color:#fee2e2; padding:4px 10px; border-radius:6px; font-weight:bold;">🔴 منتهي الصلاحية / غير مفعل</span>';
    }

    const expDisplay = data.expires ? data.expires : 'غير محدد';
    const daysDisplay = (data.days_remaining !== undefined && data.days_remaining !== null) ? `${data.days_remaining} يوم` : '-';

    const html = `
        <div style="margin-bottom:14px; display:flex; align-items:center; justify-content:space-between; flex-wrap:wrap; gap:10px;">
            <div style="font-size:15px;"><strong>حالة الترخيص:</strong> ${statusBadge}</div>
            ${data.valid ? `<div style="font-size:13px; color:#16a34a; font-weight:bold;"><i class="fa-solid fa-circle-check"></i> اللوحة تعمل بكامل الصلاحيات</div>` : `<div style="font-size:13px; color:#dc2626; font-weight:bold;"><i class="fa-solid fa-circle-exclamation"></i> يرجى التواصل مع الإدارة للتفعيل والتجديد</div>`}
        </div>
        <div style="display:grid; grid-template-columns:repeat(auto-fit, minmax(220px, 1fr)); gap:12px; font-size:13px; background:#f8fafc; padding:14px; border-radius:8px; border:1px solid #e2e8f0;">
            <div>
                <span style="color:#64748b; display:block; margin-bottom:2px;">📅 تاريخ انتهاء الاشتراك:</span>
                <span style="font-family:monospace; font-size:14px; font-weight:bold; color:#0284c7;">${expDisplay}</span>
            </div>
            <div>
                <span style="color:#64748b; display:block; margin-bottom:2px;">⏳ الأيام المتبقية:</span>
                <span style="font-size:14px; font-weight:bold; color:${data.days_remaining > 5 ? '#16a34a' : '#dc2626'};">${daysDisplay}</span>
            </div>
            ${data.serial ? `<div>
                <span style="color:#64748b; display:block; margin-bottom:2px;">📟 سيريال المايكروتك:</span>
                <code style="font-size:12px; background:#e2e8f0; padding:2px 6px; border-radius:4px;">${data.serial}</code>
            </div>` : ''}
        </div>
        ${data.message ? `<div style="font-size:12px; color:#64748b; margin-top:8px;"><i class="fa-solid fa-info-circle"></i> ${data.message}</div>` : ''}
    `;

    document.querySelectorAll('#license-status-box').forEach(el => { el.innerHTML = html; });
    if (routerBox) routerBox.style.display = data.router_connected ? 'none' : '';
    if (!gate || !main) return;
    if (data.valid) { gate.style.display = 'none'; main.style.display = ''; }
    else { gate.style.display = ''; main.style.display = 'none'; }
}

async function handleRouterConnect(e) {
    if (e) e.preventDefault();
    const btn = document.getElementById('router-connect-btn');
    const msg = document.getElementById('router-connect-msg');
    const payload = {
        address: document.getElementById('setup-router-address')?.value.trim() || '',
        user: document.getElementById('setup-router-user')?.value.trim() || '',
        pass: document.getElementById('setup-router-pass')?.value || ''
    };
    if (!payload.address || !payload.user) {
        if (msg) {
            msg.textContent = 'أدخل عنوان الراوتر واسم المستخدم';
            msg.style.color = 'var(--danger)';
        }
        return;
    }

    if (btn) {
        btn.disabled = true;
        btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> جاري الاتصال...';
    }
    if (msg) {
        msg.textContent = 'جاري الاتصال بالمايكروتك...';
        msg.style.color = 'var(--text-muted)';
    }

    try {
        const res = await fetch('/radius/api/router/connect', {
            method: 'POST',
            credentials: 'same-origin',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        const data = await res.json().catch(() => ({}));
        if (!res.ok) throw new Error(data.error || 'فشل الاتصال بالمايكروتك');
        if (msg) {
            msg.textContent = `تم الاتصال. السيريال: ${data.serial || ''}`;
            msg.style.color = 'var(--success)';
        }
        await loadLicenseStatus();
    } catch (err) {
        if (msg) {
            msg.textContent = err.message;
            msg.style.color = 'var(--danger)';
        }
    } finally {
        if (btn) {
            btn.disabled = false;
            btn.innerHTML = '<i class="fa-solid fa-plug"></i> اتصال وجلب السيريال';
        }
    }
}

async function handleProfileSubmit(e) {
    e.preventDefault();
    const form = e.target;
    const payload = { name: form.name.value.trim(), email: form.email.value.trim() };
    const res = await apiFetch('/radius/api/auth/profile', { method: 'PUT', body: JSON.stringify(payload) });
    const data = await res.json();
    alert(res.ok ? (data.message || 'تم الحفظ') : (data.error || 'خطأ'));
    if (res.ok) loadCurrentAdmin();
}

async function handlePasswordSubmit(e) {
    e.preventDefault();
    const form = e.target;
    const payload = { current: form.current.value, new: form.new.value };
    const res = await apiFetch('/radius/api/auth/password', { method: 'POST', body: JSON.stringify(payload) });
    const data = await res.json();
    alert(res.ok ? (data.message || 'تم') : (data.error || 'خطأ'));
    if (res.ok) form.reset();
}

function openAgentModal() {
    const modal = document.getElementById('agent-modal');
    if (modal) modal.classList.add('active');
}

function closeAgentModal() {
    const modal = document.getElementById('agent-modal');
    if (modal) {
        modal.classList.remove('active');
        const form = modal.querySelector('form');
        if (form) form.reset();
    }
}

async function handleRegisterSubmit(e) {
    e.preventDefault();
    const form = e.target;
    const payload = { 
        username: form.username.value.trim(), 
        password: form.password.value, 
        name: form.name.value.trim(), 
        email: form.email.value.trim(),
        role: form.role.value,
        can_manage_profiles: form.can_manage_profiles?.checked || false,
        can_manage_nas: form.can_manage_nas?.checked || false
    };
    const res = await apiFetch('/radius/api/auth/register', { method: 'POST', body: JSON.stringify(payload) });
    const data = await res.json();
    alert(res.ok ? (data.message || 'تم الإنشاء') : (data.error || 'خطأ'));
    if (res.ok) { 
        form.reset(); 
        closeAgentModal();
        loadAdmins(); 
    }
}

async function loadAdmins() {
    const tbody = document.getElementById('admins-tbody');
    if (!tbody) return;
    try {
        const res = await apiFetch('/radius/api/auth/admins');
        if (!res.ok) { tbody.innerHTML = '<tr><td colspan="8" style="text-align:center;">تعذر التحميل</td></tr>'; return; }
        const list = await res.json();
        radiusAdminsCache = list;

        // Populate selects
        const selects = document.querySelectorAll('.admin-owner-select');
        const optionsHtml = list.map(a => `<option value="${a.id}">${escapeHtml(a.username)} (${a.role === 'superadmin' ? 'مدير' : 'وكيل'})</option>`).join('');
        selects.forEach(s => {
            const currentVal = s.value;
            s.innerHTML = optionsHtml;
            if (currentVal && list.some(a => a.id == currentVal)) s.value = currentVal;
            else if (currentAdmin) s.value = currentAdmin.id;
        });

        if (!list.length) { tbody.innerHTML = '<tr><td colspan="8" style="text-align:center;">لا يوجد وكلاء حالياً</td></tr>'; return; }
        const me = currentAdmin ? currentAdmin.id : 0;
        tbody.innerHTML = list.map(a => {
            const created = a.created_at ? new Date(a.created_at).toLocaleString('ar') : '';
            const isSelf = a.id === me;
            const roleName = a.role === 'superadmin' ? 'مدير أساسي' : 'وكيل فرعي';
            const balance = (a.balance || 0).toLocaleString() + ' د.ع';
            const canManage = currentAdmin && (currentAdmin.role === 'superadmin' || a.parent_id === currentAdmin.id);
            
            let perms = [];
            if (a.can_manage_profiles) perms.push('باقات');
            if (a.can_manage_nas) perms.push('راوترات');
            const permsText = perms.length ? perms.join('، ') : 'بدون';

            const btn = isSelf
                ? '<span style="color:#64748b; font-size:13px;">حسابك</span>'
                : `<div style="display:flex; gap:5px;">
                     ${canManage ? `<button class="btn" style="width:auto; padding:6px 14px; background:#16a34a;" onclick="openAgentTxModal(${a.id}, '${(a.username || '').replace(/'/g, "\\'")}', 'recharge')">شحن</button>` : ''}
                     ${canManage ? `<button class="btn" style="width:auto; padding:6px 14px; background:#eab308; color:white; border:none;" onclick="openAgentTxModal(${a.id}, '${(a.username || '').replace(/'/g, "\\'")}', 'withdraw')">سحب</button>` : ''}
                     ${canManage ? `<button class="btn" style="width:auto; padding:6px 14px; background:#6366f1; color:white; border:none;" onclick="openAgentLogModal(${a.id}, '${(a.username || '').replace(/'/g, "\\'")}')">السجل</button>` : ''}
                     ${canManage ? `<button class="btn btn-danger" style="width:auto; padding:6px 14px;" onclick="deleteAdmin(${a.id}, '${(a.username || '').replace(/'/g, "\\'")}')">حذف</button>` : ''}
                   </div>`;
            return `<tr><td>${a.id}</td><td>${a.username}</td><td>${a.name || '-'}</td><td>${roleName}</td><td><span style="font-size:12px; color:#64748b;">${permsText}</span></td><td>${balance}</td><td>${created}</td><td>${btn}</td></tr>`;
        }).join('');
    } catch (e) { tbody.innerHTML = '<tr><td colspan="8" style="text-align:center;">خطأ في تحميل البيانات</td></tr>'; }
}

async function loadRadiusRemoteAccess() {
    const card = document.getElementById('radius-remote-access-card');
    const status = document.getElementById('radius-remote-access-status');
    const buttons = document.getElementById('radius-remote-access-buttons');
    if (!card || !status || !buttons) return;

    card.style.display = '';
    status.textContent = 'جاري فحص روابط الوصول عن بُعد...';
    status.style.color = 'var(--text-muted)';
    buttons.style.display = 'none';
    radiusRemoteBaseURL = "";

    try {
        const res = await apiFetch('/radius/api/ngrok/token');
        if (!res.ok) throw new Error('تعذر جلب روابط الوصول عن بُعد');
        const data = await res.json();
        const ngrokURL = data.ngrok && data.ngrok.web ? data.ngrok.web : "";
        radiusRemoteBaseURL = ngrokURL;

        if (!radiusRemoteBaseURL) {
            status.textContent = 'لم يتم تجهيز رابط وصول بعد. اضغط تحديث بعد لحظات.';
            status.style.color = 'var(--warning-hover)';
            return;
        }

        status.textContent = `الرابط جاهز عبر Ngrok: ${radiusRemoteBaseURL}`;
        status.style.color = 'var(--success)';
        buttons.style.display = 'flex';
    } catch (err) {
        status.textContent = err.message || 'فشل فحص روابط الوصول عن بُعد';
        status.style.color = 'var(--danger)';
    }
}

function copyRadiusRemotePath(path) {
    if (!radiusRemoteBaseURL) {
        alert('الرابط غير جاهز بعد. اضغط تحديث وحاول مرة أخرى.');
        return;
    }
    const fullURL = radiusRemoteBaseURL.replace(/\/$/, '') + path;
    if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(fullURL)
            .then(() => alert('تم نسخ الرابط:\n' + fullURL))
            .catch(() => fallbackRadiusCopy(fullURL));
    } else {
        fallbackRadiusCopy(fullURL);
    }
}

function fallbackRadiusCopy(text) {
    const textArea = document.createElement('textarea');
    textArea.value = text;
    textArea.style.position = 'fixed';
    textArea.style.left = '-9999px';
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();
    try {
        document.execCommand('copy');
        alert('تم نسخ الرابط:\n' + text);
    } catch (e) {
        alert('تعذر النسخ، الرابط هو:\n' + text);
    }
    document.body.removeChild(textArea);
}

function openAgentTxModal(id, username, type) {
    document.getElementById('agent-tx-admin-id').value = id;
    document.getElementById('agent-tx-type').value = type;
    
    const titleEl = document.getElementById('agent-tx-title');
    const subtitleEl = document.getElementById('agent-tx-subtitle');
    
    if (type === 'recharge') {
        titleEl.innerText = 'شحن رصيد الوكيل';
        subtitleEl.innerHTML = `شحن رصيد الوكيل: <strong>${escapeHtml(username)}</strong>`;
    } else {
        titleEl.innerText = 'سحب رصيد الوكيل';
        subtitleEl.innerHTML = `سحب رصيد الوكيل: <strong>${escapeHtml(username)}</strong>`;
    }
    
    document.getElementById('agent-tx-amount').value = '';
    document.getElementById('agent-tx-notes').value = '';
    
    document.getElementById('agent-tx-modal').classList.add('active');
}

function closeAgentTxModal() {
    document.getElementById('agent-tx-modal').classList.remove('active');
}

async function submitAgentTransaction() {
    const id = parseInt(document.getElementById('agent-tx-admin-id').value);
    const type = document.getElementById('agent-tx-type').value;
    const amountStr = document.getElementById('agent-tx-amount').value;
    const notes = document.getElementById('agent-tx-notes').value.trim();
    
    if (!amountStr || isNaN(amountStr) || parseFloat(amountStr) <= 0) {
        alert('يرجى إدخال مبلغ صحيح أكبر من صفر');
        return;
    }
    const amount = parseFloat(amountStr);
    
    const url = type === 'recharge' ? '/radius/api/auth/recharge' : '/radius/api/auth/withdraw';
    
    const res = await apiFetch(url, {
        method: 'POST',
        body: JSON.stringify({ admin_id: id, amount: amount, notes: notes })
    });
    const data = await res.json();
    alert(res.ok ? (data.message || 'تمت العملية بنجاح') : (data.error || 'حدث خطأ أثناء تنفيذ العملية'));
    if (res.ok) {
        closeAgentTxModal();
        loadCurrentAdmin();
        loadAdmins();
    }
}

async function openAgentLogModal(id, username) {
    document.getElementById('agent-log-title').innerHTML = `سجل العمليات المالية للوكيل: <strong>${escapeHtml(username)}</strong>`;
    const tbody = document.getElementById('agent-log-tbody');
    tbody.innerHTML = '<tr><td colspan="6" style="text-align:center;">جاري تحميل سجل العمليات...</td></tr>';
    
    document.getElementById('agent-log-modal').classList.add('active');
    
    try {
        const res = await apiFetch('/radius/api/auth/admins/transactions');
        if (!res.ok) {
            tbody.innerHTML = '<tr><td colspan="6" style="text-align:center; color: var(--danger);">تعذر تحميل سجل العمليات</td></tr>';
            return;
        }
        const list = await res.json();
        const filtered = list.filter(tx => tx.admin_id == id);
        
        if (!filtered.length) {
            tbody.innerHTML = '<tr><td colspan="6" style="text-align:center;">لا توجد عمليات مسجلة لهذا الوكيل بعد</td></tr>';
            return;
        }
        
        tbody.innerHTML = filtered.map(tx => {
            const date = tx.created_at ? new Date(tx.created_at).toLocaleString('ar') : '';
            const tTypeName = tx.transaction_type === 'recharge' ? 'شحن رصيد ➕' : 'سحب رصيد ➖';
            const tTypeColor = tx.transaction_type === 'recharge' ? '#16a34a' : '#dc2626';
            const amount = (tx.amount || 0).toLocaleString() + ' د.ع';
            return `<tr>
                <td>${tx.id}</td>
                <td>${escapeHtml(tx.performer_name)}</td>
                <td><span style="font-weight:bold; color:${tTypeColor};">${tTypeName}</span></td>
                <td style="font-weight:bold; color:${tTypeColor};">${amount}</td>
                <td>${escapeHtml(tx.notes) || '-'}</td>
                <td style="font-size:13px; color:#64748b;">${date}</td>
            </tr>`;
        }).join('');
    } catch (e) {
        tbody.innerHTML = '<tr><td colspan="6" style="text-align:center; color: var(--danger);">خطأ أثناء جلب البيانات</td></tr>';
    }
}

function closeAgentLogModal() {
    document.getElementById('agent-log-modal').classList.remove('active');
}

async function deleteAdmin(id, username) {
    if (!confirm(`هل تريد حذف الحساب "${username}"؟`)) return;
    const res = await apiFetch('/radius/api/auth/admins/' + id, { method: 'DELETE' });
    const data = await res.json();
    alert(res.ok ? (data.message || 'تم الحذف') : (data.error || 'خطأ'));
    if (res.ok) loadAdmins();
}

async function handleLicenseSubmit(e) {
    e.preventDefault();
    const key = document.getElementById('license-key-input').value.trim();
    if (!key) return;
    const payload = { key };
    const address = document.getElementById('setup-router-address')?.value.trim() || '';
    if (!licenseState.router_connected && address) {
        payload.address = address;
        payload.user = document.getElementById('setup-router-user')?.value.trim() || '';
        payload.pass = document.getElementById('setup-router-pass')?.value || '';
    }
    const res = await fetch('/radius/api/license/activate', {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
    });
    const data = await res.json();
    if (res.ok) { alert(data.message || 'تم التفعيل'); await loadLicenseStatus(); if (typeof loadProfiles === 'function') loadProfiles(); if (typeof loadUsers === 'function') loadUsers(); if (typeof loadNAS === 'function') loadNAS(); }
    else alert(data.error || 'خطأ');
}

async function downloadBackup() {
    try {
        const res = await fetch('/radius/api/auth/backup', { credentials: 'same-origin' });
        if (res.status === 401) { window.location.href = '/radius/login.html'; return; }
        if (!res.ok) { const d = await res.json().catch(() => ({})); alert(d.error || 'تعذر التنزيل'); return; }
        const blob = await res.blob();
        const disp = res.headers.get('Content-Disposition') || '';
        const m = disp.match(/filename="([^"]+)"/);
        const filename = m ? m[1] : ('sasman_backup_' + new Date().toISOString().slice(0, 10) + '.db');
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url; a.download = filename;
        document.body.appendChild(a); a.click(); a.remove();
        URL.revokeObjectURL(url);
    } catch (e) { alert('خطأ: ' + e.message); }
}

async function handleRestore(e) {
    e.preventDefault();
    const form = e.target;
    const file = form.file.files[0];
    if (!file) return;
    if (!confirm(`سيتم استبدال جميع البيانات الحالية بمحتوى الملف "${file.name}". هل أنت متأكد؟`)) return;
    const fd = new FormData();
    fd.append('file', file);
    try {
        const res = await fetch('/radius/api/auth/restore', { method: 'POST', credentials: 'same-origin', body: fd });
        const data = await res.json().catch(() => ({}));
        if (!res.ok) { alert(data.error || 'فشل الاستعادة'); return; }
        alert((data.message || 'تمت الاستعادة') + '\nسيتم تحديث الصفحة.');
        window.location.reload();
    } catch (err) { alert('خطأ: ' + err.message); }
}

async function loadTelegramBackupConfig() {
    const enabled = document.getElementById('tg-backup-enabled');
    const botEnabled = document.getElementById('tg-bot-enabled');
    const interval = document.getElementById('tg-backup-interval');
    const token = document.getElementById('tg-backup-token');
    const tokenHint = document.getElementById('tg-backup-token-hint');
    const chat = document.getElementById('tg-backup-chat');
    const status = document.getElementById('tg-backup-status');
    if (!enabled || !botEnabled || !interval || !token || !chat) return;

    try {
        const res = await apiFetch('/radius/api/auth/backup/telegram');
        if (!res.ok) return;
        const data = await res.json();
        enabled.value = data.enabled ? '1' : '0';
        botEnabled.value = data.bot_enabled ? '1' : '0';
        interval.value = String(data.interval_hours || 24);
        chat.value = data.chat_id || '';
        token.value = '';
        tokenHint.textContent = data.bot_token ? `التوكن محفوظ: ${data.bot_token}` : 'لم يتم حفظ توكن بعد.';
        const last = data.last_sent_at ? new Date(data.last_sent_at * 1000).toLocaleString() : 'لم يتم الإرسال بعد';
        status.textContent = `النسخ الاحتياطي: ${data.enabled ? 'مفعّل' : 'معطّل'} | البوت التفاعلي: ${data.bot_enabled ? 'مفعّل' : 'معطّل'} — آخر إرسال: ${last}`;
    } catch (e) {
        console.error('Failed to load Telegram backup config', e);
    }
}

async function saveTelegramBackupConfig() {
    const payload = {
        enabled: document.getElementById('tg-backup-enabled').value === '1',
        bot_enabled: document.getElementById('tg-bot-enabled').value === '1',
        interval_hours: parseInt(document.getElementById('tg-backup-interval').value, 10),
        bot_token: document.getElementById('tg-backup-token').value.trim(),
        chat_id: document.getElementById('tg-backup-chat').value.trim()
    };

    try {
        const res = await apiFetch('/radius/api/auth/backup/telegram', {
            method: 'POST',
            body: JSON.stringify(payload)
        });
        const data = await res.json().catch(() => ({}));
        alert(res.ok ? (data.message || 'تم الحفظ') : (data.error || 'فشل حفظ الإعدادات'));
        if (res.ok) loadTelegramBackupConfig();
    } catch (e) {
        alert('خطأ: ' + e.message);
    }
}

async function testTelegramBackup() {
    if (!confirm('سيتم إنشاء نسخة SQL كاملة وإرسالها إلى تيليگرام الآن. متابعة؟')) return;
    const status = document.getElementById('tg-backup-status');
    if (status) status.textContent = 'جارِ إنشاء النسخة وإرسالها...';

    try {
        const res = await apiFetch('/radius/api/auth/backup/telegram/test', { method: 'POST' });
        const data = await res.json().catch(() => ({}));
        alert(res.ok ? (data.message || 'تم الإرسال') : (data.error || 'فشل الإرسال'));
        loadTelegramBackupConfig();
    } catch (e) {
        alert('خطأ: ' + e.message);
    }
}

async function logout() {
    await apiFetch('/radius/api/auth/logout', { method: 'POST' });
    window.location.href = '/radius/login.html';
}

// ============================================================
// SASMAN GLOBAL BYPASS (Blind Accept - SAS 4 Style)
// ============================================================

let bypassState = { enabled: false, loading: false };

async function loadBypassStatus() {
    const badge = document.getElementById('bypass-status-badge');
    const btn = document.getElementById('bypass-toggle-btn');
    if (!badge || !btn) return;

    try {
        const res = await apiFetch('/radius/api/bypass');
        if (!res.ok) throw new Error('Failed');
        const data = await res.json();
        bypassState.enabled = data.enabled;
        updateBypassUI();
    } catch (e) {
        if (badge) badge.textContent = '⚠️ خطأ';
        if (badge) badge.style.background = '#fecaca';
        if (badge) badge.style.color = '#991b1b';
    }
}

function updateBypassUI() {
    const badge = document.getElementById('bypass-status-badge');
    const btn = document.getElementById('bypass-toggle-btn');
    if (!badge || !btn) return;

    if (bypassState.enabled) {
        badge.textContent = '🔥 مفعّل - الجميع يدخل!';
        badge.style.background = '#fecaca';
        badge.style.color = '#991b1b';
        btn.textContent = '⛔ تعطيل البايپاس';
        btn.style.background = '#374151';
    } else {
        badge.textContent = '🔒 معطّل - المصادقة طبيعية';
        badge.style.background = '#d1fae5';
        badge.style.color = '#065f46';
        btn.textContent = '🚀 تفعيل البايپاس';
        btn.style.background = '#dc2626';
    }
}

async function toggleBypass() {
    const btn = document.getElementById('bypass-toggle-btn');
    if (!btn || bypassState.loading) return;

    const newState = !bypassState.enabled;
    const confirmMsg = newState
        ? '⚠️ هل أنت متأكد من تفعيل وضع البايپاس العمياء؟\n\nسيتم قبول جميع محاولات الاتصال بدون التحقق من الباسورد.\n\nتأكد من:\n1. تفعيل PAP/CHAP في PPPoE Server بالمايكروتيك\n2. إلغاء تفعيل mschap2'
        : 'هل تريد تعطيل وضع البايپاس وإعادة المصادقة الطبيعية؟';

    if (!confirm(confirmMsg)) return;

    bypassState.loading = true;
    btn.disabled = true;
    btn.textContent = '⏳ جارِ التنفيذ...';

    try {
        const res = await apiFetch('/radius/api/bypass', {
            method: 'POST',
            body: JSON.stringify({ enabled: newState })
        });
        const data = await res.json();
        if (res.ok) {
            bypassState.enabled = newState;
            updateBypassUI();
            alert(data.message || 'تم التحديث');
        } else {
            alert(data.error || 'فشل التحديث');
            updateBypassUI();
        }
    } catch (e) {
        alert('خطأ في الاتصال');
        updateBypassUI();
    } finally {
        bypassState.loading = false;
        btn.disabled = false;
    }
}

// ─── AUDIT LOGS MODULE ───────────────────────────────────────────────────────
let currentAuditPage = 1;

async function loadAuditLogs(page = 1) {
    currentAuditPage = page;
    const tbody = document.getElementById('audit-logs-table-body');
    const info = document.getElementById('audit-logs-info');
    const pagination = document.getElementById('audit-logs-pagination');
    if (!tbody) return;

    tbody.innerHTML = '<tr><td colspan="7" style="text-align:center; padding:20px; color:var(--text-muted);">⏳ جاري تحميل سجل العمليات...</td></tr>';

    const search = encodeURIComponent(document.getElementById('audit-search')?.value.trim() || '');
    const actionType = encodeURIComponent(document.getElementById('audit-filter-type')?.value || '');
    const startDate = encodeURIComponent(document.getElementById('audit-start-date')?.value || '');
    const endDate = encodeURIComponent(document.getElementById('audit-end-date')?.value || '');

    const url = `/radius/api/audit-logs?page=${page}&limit=50&search=${search}&action_type=${actionType}&start_date=${startDate}&end_date=${endDate}`;

    try {
        const res = await apiFetch(url);
        if (!res.ok) throw new Error('فشل جلب سجل العمليات');
        const data = await res.json();

        const logs = data.logs || [];
        const total = data.total || 0;
        const totalPages = data.total_pages || 1;

        if (logs.length === 0) {
            tbody.innerHTML = '<tr><td colspan="7" style="text-align:center; padding:30px; color:var(--text-muted);">📭 لا توجد عمليات مسجلة تطابق خيارات البحث.</td></tr>';
            if (info) info.textContent = 'عرض 0 من 0 سجل';
            if (pagination) pagination.innerHTML = '';
            return;
        }

        const badgeColorMap = {
            'تجديد مشترك': 'background:#dcfce7; color:#15803d; border:1px solid #bbf7d0;',
            'إضافة مشترك': 'background:#dbeafe; color:#1d4ed8; border:1px solid #bfdbfe;',
            'تعديل مشترك': 'background:#e0f2fe; color:#0369a1; border:1px solid #bae6fd;',
            'حذف مشترك': 'background:#fee2e2; color:#b91c1c; border:1px solid #fecaca;',
            'شحن رصيد وكيل': 'background:#f0fdf4; color:#166534; border:1px solid #bbf7d0;',
            'سحب رصيد وكيل': 'background:#fff1f2; color:#be123c; border:1px solid #fecdd3;',
            'إضافة دين لمشترك': 'background:#fef3c7; color:#b45309; border:1px solid #fde68a;',
            'تسديد دين مشترك': 'background:#ecfdf5; color:#047857; border:1px solid #a7f3d0;',
            'إضافة باقة': 'background:#f3e8ff; color:#7e22ce; border:1px solid #e9d5ff;',
            'تعديل باقة': 'background:#f5f3ff; color:#6d28d9; border:1px solid #ddd6fe;',
            'حذف باقة': 'background:#fdf2f8; color:#be185d; border:1px solid #fbcfe8;',
            'إضافة جهاز NAS': 'background:#e0e7ff; color:#4338ca; border:1px solid #c7d2fe;',
            'حذف جهاز NAS': 'background:#ffe4e6; color:#e11d48; border:1px solid #fecdd3;',
            'توليد كروت': 'background:#fae8ff; color:#a21caf; border:1px solid #f5d0fe;',
            'تفعيل كارت': 'background:#dcfce7; color:#15803d; border:1px solid #bbf7d0;',
            'تسجيل دخول': 'background:#f1f5f9; color:#334155; border:1px solid #cbd5e1;',
            'تسجيل خروج': 'background:#f8fafc; color:#64748b; border:1px solid #e2e8f0;',
            'نسخ احتياطي': 'background:#e0f2fe; color:#0284c7; border:1px solid #bae6fd;',
            'استعادة نسخة احتياطية': 'background:#ffedd5; color:#c2410c; border:1px solid #fed7aa;',
            'تصفير النظام': 'background:#fee2e2; color:#991b1b; border:1px solid #fecaca;'
        };

        tbody.innerHTML = logs.map(l => {
            const style = badgeColorMap[l.action_type] || 'background:#f1f5f9; color:#475569; border:1px solid #cbd5e1;';
            const formattedDate = l.created_at ? new Date(l.created_at).toLocaleString('ar-EG', {
                year: 'numeric', month: '2-digit', day: '2-digit',
                hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: true
            }) : '---';

            return `
                <tr>
                    <td style="font-weight:bold; color:var(--text-muted);">${l.id}</td>
                    <td>
                        <span style="font-weight:bold; color:var(--text-main);"><i class="fa-solid fa-user-gear" style="margin-left:4px; color:var(--primary);"></i>${escapeHtml(l.admin_username || 'النظام')}</span>
                    </td>
                    <td>
                        <span style="display:inline-block; padding:3px 8px; border-radius:6px; font-size:12px; font-weight:bold; ${style}">
                            ${escapeHtml(l.action_type)}
                        </span>
                    </td>
                    <td><strong style="color:var(--primary);">${escapeHtml(l.target || '---')}</strong></td>
                    <td style="font-size:13px; color:var(--text-main); line-height:1.4;">${escapeHtml(l.details || '---')}</td>
                    <td><code style="font-size:11px; background:var(--bg-body, #f1f5f9); padding:2px 6px; border-radius:4px;">${escapeHtml(l.ip_address || '127.0.0.1')}</code></td>
                    <td style="font-size:12px; color:var(--text-muted); dir:ltr; text-align:right;">${formattedDate}</td>
                </tr>
            `;
        }).join('');

        if (info) {
            const startItem = (page - 1) * 50 + 1;
            const endItem = Math.min(page * 50, total);
            info.textContent = `عرض ${startItem}-${endItem} من أصل ${total} سجل`;
        }

        // Render Pagination buttons
        if (pagination) {
            let btnsHTML = '';
            if (page > 1) {
                btnsHTML += `<button class="btn" style="padding:4px 10px; font-size:12px;" onclick="loadAuditLogs(${page - 1})">السابق ◀</button>`;
            }
            btnsHTML += `<span style="align-self:center; font-size:13px; font-weight:bold; padding:0 8px;">صفحة ${page} من ${totalPages}</span>`;
            if (page < totalPages) {
                btnsHTML += `<button class="btn" style="padding:4px 10px; font-size:12px;" onclick="loadAuditLogs(${page + 1})">التالي ▶</button>`;
            }
            pagination.innerHTML = btnsHTML;
        }

    } catch (err) {
        tbody.innerHTML = `<tr><td colspan="7" style="text-align:center; color:var(--danger); padding:20px;">❌ ${err.message}</td></tr>`;
    }
}

function exportAuditLogsCSV() {
    const search = encodeURIComponent(document.getElementById('audit-search')?.value.trim() || '');
    const actionType = encodeURIComponent(document.getElementById('audit-filter-type')?.value || '');
    window.open(`/radius/api/audit-logs/export?search=${search}&action_type=${actionType}`, '_blank');
}

async function clearAuditLogsModal() {
    const days = prompt("⚠️ لتأكيد تصفير السجل، حدد الخيار:\n- اكتب 'all' لمسح كافة العمليات بالسجل بالكامل.\n- أو أدخل عدد الأيام لحذف العمليات الأقدم منها (مثال: 30 لحذف الأقدم من شهر):");
    if (!days) return;

    let url = '/radius/api/audit-logs';
    if (days.trim().toLowerCase() !== 'all' && !isNaN(parseInt(days))) {
        url += `?days=${parseInt(days)}`;
    }

    try {
        const res = await apiFetch(url, { method: 'DELETE' });
        const data = await res.json();
        if (res.ok) {
            alert(data.message || 'تم تصفير السجل بنجاح');
            loadAuditLogs(1);
        } else {
            alert(data.error || 'فشل تصفير السجل');
        }
    } catch (e) {
        alert('حدث خطأ في الاتصال أثناء تصفير السجل');
    }
}

// ─── First-Time Onboarding & Cloud Tunnel Management ─────────────────────────

let liveCheckTimer = null;
async function handleSubdomainLiveCheck(subdomain) {
    clearTimeout(liveCheckTimer);
    const statusEl = document.getElementById('ob-subdomain-status');
    if (!statusEl) return;

    subdomain = (subdomain || '').trim().toLowerCase();
    if (!subdomain) {
        statusEl.innerHTML = '';
        return;
    }
    if (subdomain.length < 3) {
        statusEl.innerHTML = '<span style="color:#f59e0b;">⚠️ يجب أن يكون طول النطاق 3 أحرف على الأقل</span>';
        return;
    }

    statusEl.innerHTML = '<span style="color:#64748b;"><i class="fa-solid fa-spinner fa-spin"></i> جاري التحقق من توفر النطاق...</span>';

    liveCheckTimer = setTimeout(async () => {
        try {
            const res = await apiFetch('/radius/api/setup/check-subdomain', {
                method: 'POST',
                body: JSON.stringify({ subdomain })
            });
            const data = await res.json();
            if (data.available) {
                statusEl.innerHTML = `<span style="color:#16a34a; font-weight:bold;"><i class="fa-solid fa-circle-check"></i> النطاق <code>${data.full_domain}</code> متاح وجاهز للاستخدام!</span>`;
            } else {
                statusEl.innerHTML = `<span style="color:#dc2626; font-weight:bold;"><i class="fa-solid fa-circle-xmark"></i> ${data.error || 'هذا النطاق مستخدم بالفعل، يرجى اختيار اسم آخر'}</span>`;
            }
        } catch (e) {
            statusEl.innerHTML = '<span style="color:#f59e0b;">تعذر التحقق الآن، سيتم التحقق عند الإرسال</span>';
        }
    }, 350);
}

async function submitOnboarding(e) {
    if (e) e.preventDefault();
    const name = document.getElementById('ob-name')?.value.trim();
    const phone = document.getElementById('ob-phone')?.value.trim();
    const subdomain = document.getElementById('ob-subdomain')?.value.trim().toLowerCase();
    const routerAddress = document.getElementById('ob-router-address')?.value.trim();
    const routerUser = document.getElementById('ob-router-user')?.value.trim();
    const routerPass = document.getElementById('ob-router-pass')?.value || '';
    const btn = document.getElementById('btn-ob-submit');
    const errBox = document.getElementById('ob-error-msg');

    if (errBox) errBox.style.display = 'none';

    if (!name || !phone || !subdomain) {
        if (errBox) {
            errBox.textContent = 'يرجى إدخال الاسم الكامل، رقم الهاتف، واسم النطاق المطلوب.';
            errBox.style.display = 'block';
        }
        return;
    }

    if (btn) {
        btn.disabled = true;
        btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> جاري حجز النطاق والربط بالسيرفر...';
    }

    try {
        const res = await apiFetch('/radius/api/setup/self-register', {
            method: 'POST',
            body: JSON.stringify({
                name,
                phone,
                subdomain,
                router_address: routerAddress,
                router_user: routerUser,
                router_pass: routerPass
            })
        });
        const data = await res.json();
        if (!res.ok) {
            throw new Error(data.error || 'فشل إكمال الإعداد');
        }

        alert(`✅ تم إعداد النطاق بنجاح!\nنطاقك الخاص هو: ${data.full_domain}\nسيتم الآن نقلك إلى خطوة الترخيص.`);
        
        isFreshInstall = false;
        const obGate = document.getElementById('onboarding-gate');
        if (obGate) obGate.style.display = 'none';

        await loadLicenseStatus();
        loadTunnelCardInfo();
    } catch (err) {
        if (errBox) {
            errBox.textContent = err.message || 'حدث خطأ أثناء الإعداد';
            errBox.style.display = 'block';
        }
    } finally {
        if (btn) {
            btn.disabled = false;
            btn.innerHTML = '<span>🚀 إكمال الإعداد وتفعيل النطاق السحابي</span>';
        }
    }
}

let currentTunnelFullDomain = "";
let currentTunnelWinboxAddr = "";

async function loadTunnelCardInfo() {
    try {
        const res = await apiFetch('/radius/api/setup/status');
        if (!res.ok) return;
        const data = await res.json();

        const domainEl = document.getElementById('tunnel-card-domain');
        const linkEl = document.getElementById('tunnel-card-link');
        const winboxEl = document.getElementById('tunnel-card-winbox');
        const statusEl = document.getElementById('tunnel-card-status');
        const ownerEl = document.getElementById('tunnel-card-owner');

        const advSubdomain = document.getElementById('adv-tunnel-subdomain');
        const advToken = document.getElementById('adv-tunnel-token');

        if (data.full_domain) {
            currentTunnelFullDomain = `http://${data.full_domain}`;
            if (domainEl) domainEl.textContent = data.full_domain;
            if (linkEl) {
                linkEl.href = `http://${data.full_domain}`;
                linkEl.style.display = 'inline-block';
            }
        } else {
            if (domainEl) domainEl.textContent = 'لم يتم تعيين نطاق بعد';
            if (linkEl) linkEl.style.display = 'none';
        }

        if (data.winbox_address) {
            currentTunnelWinboxAddr = data.winbox_address;
            if (winboxEl) winboxEl.textContent = data.winbox_address;
        } else if (data.winbox_port > 0 && data.subdomain) {
            currentTunnelWinboxAddr = `${data.full_domain}:${data.winbox_port}`;
            if (winboxEl) winboxEl.textContent = currentTunnelWinboxAddr;
        } else {
            if (winboxEl) winboxEl.textContent = 'غير متاح حالياً';
        }

        if (statusEl) {
            if (data.tunnel_connected) {
                statusEl.innerHTML = '<span style="color:#16a34a;"><i class="fa-solid fa-circle-check"></i> متصل بالسحابة (Online)</span>';
            } else {
                statusEl.innerHTML = '<span style="color:#dc2626;"><i class="fa-solid fa-circle-xmark"></i> غير متصل بالسحابة (Offline)</span>';
            }
        }

        if (ownerEl) {
            let ownerText = '';
            if (data.owner_name) ownerText += `👤 المالك: ${escapeHtml(data.owner_name)}`;
            if (data.owner_phone) ownerText += ` | 📱 الهاتف: ${escapeHtml(data.owner_phone)}`;
            ownerEl.textContent = ownerText || 'وكيل مسجل';
        }

        if (advSubdomain && data.subdomain) advSubdomain.value = data.subdomain;
        if (advToken && data.token) advToken.value = data.token;
    } catch (e) {
        console.error('loadTunnelCardInfo error:', e);
    }
}

function copyTunnelDomain() {
    if (!currentTunnelFullDomain) {
        alert('لا يوجد نطاق متاح للنسخ');
        return;
    }
    navigator.clipboard.writeText(currentTunnelFullDomain)
        .then(() => alert('تم نسخ رابط اللوحة:\n' + currentTunnelFullDomain))
        .catch(() => alert('الرابط هو: ' + currentTunnelFullDomain));
}

function copyTunnelWinbox() {
    if (!currentTunnelWinboxAddr) {
        alert('لا يوجد عنوان Winbox متاح للنسخ');
        return;
    }
    navigator.clipboard.writeText(currentTunnelWinboxAddr)
        .then(() => alert('تم نسخ عنوان Winbox المباشر:\n' + currentTunnelWinboxAddr))
        .catch(() => alert('عنوان Winbox هو: ' + currentTunnelWinboxAddr));
}

async function handleTunnelSave(e) {
    e.preventDefault();
    const subdomain = document.getElementById('adv-tunnel-subdomain')?.value.trim();
    const token = document.getElementById('adv-tunnel-token')?.value.trim();
    const gatewayUrl = document.getElementById('adv-tunnel-gateway')?.value.trim();

    try {
        const res = await apiFetch('/radius/api/auth/tunnel/config', {
            method: 'POST',
            body: JSON.stringify({
                mode: 'agent',
                subdomain: subdomain,
                token: token,
                gateway_url: gatewayUrl
            })
        });
        const data = await res.json();
        alert(res.ok ? (data.message || 'تم الحفظ وإعادة تشغيل النفق') : (data.error || 'فشل الحفظ'));
        if (res.ok) loadTunnelCardInfo();
    } catch (err) {
        alert('خطأ في حفظ إعدادات النفق: ' + err.message);
    }
}

// Make functions accessible globally
window.loadAuditLogs = loadAuditLogs;
window.exportAuditLogsCSV = exportAuditLogsCSV;
window.clearAuditLogsModal = clearAuditLogsModal;
window.handleSubdomainLiveCheck = handleSubdomainLiveCheck;
window.submitOnboarding = submitOnboarding;
window.loadTunnelCardInfo = loadTunnelCardInfo;
window.copyTunnelDomain = copyTunnelDomain;
window.copyTunnelWinbox = copyTunnelWinbox;
window.handleTunnelSave = handleTunnelSave;
