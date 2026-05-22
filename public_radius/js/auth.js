let currentAdmin = null;
let radiusAdminsCache = [];
let licenseState = { valid: false, router_connected: false };

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
        const res = await apiFetch('/radius/api/auth/me');
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

async function loadLicenseStatus() {
    try {
        const res = await apiFetch('/radius/api/license/status');
        const data = await res.json();
        licenseState = data;
        renderLicenseGate(data);
        return data;
    } catch (e) { return null; }
}

function renderLicenseGate(data) {
    const gate = document.getElementById('license-gate');
    const main = document.getElementById('main-area');
    const html = `
        <div><strong>حالة الترخيص:</strong> ${data.valid ? '<span style="color:#166534;">مفعل ✅</span>' : '<span style="color:#991b1b;">غير مفعل</span>'}</div>
        <div style="font-size:13px; color:#475569; margin-top:4px;">${data.message || ''}</div>
        ${data.serial ? `<div style="font-size:13px; color:#475569;"><strong>السيريال:</strong> <code>${data.serial}</code></div>` : ''}
        ${data.expires && data.valid ? `<div style="font-size:13px; color:#475569;"><strong>ينتهي:</strong> ${data.expires}</div>` : ''}
    `;
    document.querySelectorAll('#license-status-box').forEach(el => { el.innerHTML = html; });
    if (!gate || !main) return;
    if (data.valid) { gate.style.display = 'none'; main.style.display = ''; }
    else { gate.style.display = ''; main.style.display = 'none'; }
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
    const res = await apiFetch('/radius/api/license/activate', { method: 'POST', body: JSON.stringify({ key }) });
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
