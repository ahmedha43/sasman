let radiusUsersCache = [];
let renewTargetUser = '';
let editingUser = null;
let usersAutoRefreshTimer = null;
const USERS_AUTO_REFRESH_MS = 30000;

function startUsersAutoRefresh() {
    stopUsersAutoRefresh();
    usersAutoRefreshTimer = setInterval(() => {
        const tab = document.getElementById('tab-users');
        if (tab && tab.classList.contains('active') && !document.hidden) {
            loadUsers();
        }
    }, USERS_AUTO_REFRESH_MS);
}

function stopUsersAutoRefresh() {
    if (usersAutoRefreshTimer) { clearInterval(usersAutoRefreshTimer); usersAutoRefreshTimer = null; }
}

function renderSessionStatus(_user, session) {
    const status = session && session.status ? session.status : 'offline';
    const map = {
        online: { text: 'متصل', className: 'badge-success' },
        stale: { text: 'تأخر التحديث', className: 'badge-warning' },
        offline: { text: 'غير متصل', className: 'badge-secondary' },
        expired: { text: 'منتهي', className: 'badge-danger' },
        expired_online: { text: 'منتهي (متصل)', className: 'badge-warning' }
    };
    const meta = map[status] || map.offline;
    return `<span class="badge ${meta.className}">${meta.text}</span>`;
}

function getInitials(name, username) {
    if (name) {
        const parts = name.trim().split(/\s+/);
        if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase();
        return name.slice(0, 2).toUpperCase();
    }
    return username ? username.slice(0, 2).toUpperCase() : '??';
}

function escapeHtml(value) {
    return String(value ?? '')
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}

function formatProfileValidity(profile) {
    if (!profile) return 'الصلاحية: غير معروفة';
    if (!profile.validity_days) return 'الصلاحية: مفتوحة';
    return `الصلاحية: ${profile.validity_days} يوم`;
}

function resetUserForm() {
    document.getElementById('usr-name').value = '';
    document.getElementById('usr-pass').value = '';
    document.getElementById('usr-full-name').value = '';
    document.getElementById('usr-phone').value = '';
    const expiryEl = document.getElementById('usr-expiry');
    expiryEl.value = '';
    const expiryWrapper = document.getElementById('usr-expiry-wrapper');
    expiryWrapper.style.display = ''; // Always show for new user creation mode

    document.getElementById('usr-submit-btn').innerText = 'حفظ المشترك';
    document.getElementById('usr-name').readOnly = false;
    editingUser = null;
}

function fillRenewProfileOptions(selectedProfileName = '') {
    const select = document.getElementById('renew-profile-select');

    if (!Array.isArray(radiusProfilesCache) || radiusProfilesCache.length === 0) {
        select.innerHTML = '<option value="">لا توجد باقات متوفرة</option>';
        document.getElementById('renew-profile-validity').innerText = 'أضف باقة أولاً حتى تتمكن من التجديد.';
        return false;
    }

    select.innerHTML = radiusProfilesCache
        .map(profile => `<option value="${escapeHtml(profile.name)}">${escapeHtml(profile.name)}</option>`)
        .join('');

    if (selectedProfileName && radiusProfilesCache.some(profile => profile.name === selectedProfileName)) {
        select.value = selectedProfileName;
    }

    updateRenewValidityHint();
    return true;
}

function updateRenewValidityHint() {
    const selectedName = document.getElementById('renew-profile-select').value;
    const selectedProfile = radiusProfilesCache.find(profile => profile.name === selectedName);
    document.getElementById('renew-profile-validity').innerText = formatProfileValidity(selectedProfile);
    updateRenewPriceDisplay();
}

function updateRenewPriceDisplay() {
    const selectedName = document.getElementById('renew-profile-select').value;
    const selectedProfile = radiusProfilesCache.find(profile => profile.name === selectedName);
    const priceDisplay = document.getElementById('renew-price-display');
    const isPaid = document.getElementById('renew-paid').checked;

    if (selectedProfile && selectedProfile.price > 0) {
        const statusText = isPaid ? '✅ مدفوع (لا تُضاف ديون)' : '⏳ غير مدفوع (سيتم إضافة ديون)';
        priceDisplay.innerHTML = `💰 سعر الباقة: ${selectedProfile.price.toLocaleString()} د.ع<br><span style="font-size:13px;">${statusText}</span>`;
        priceDisplay.style.display = 'block';
    } else {
        priceDisplay.style.display = 'none';
    }
}

function openRenewModal(encodedUser) {
    const username = decodeURIComponent(encodedUser);
    const user = radiusUsersCache.find(item => item.user === username);
    if (!user) return;

    document.getElementById('renew-modal-user').innerText = `المشترك: ${username}`;
    document.getElementById('renew-current-expiry').innerText = user.expires_at
        ? `تاريخ الانتهاء الحالي: ${user.expires_at}`
        : 'تاريخ الانتهاء الحالي: غير متوفر';

    if (!fillRenewProfileOptions(user.profile || '')) {
        renewTargetUser = '';
        alert('لا توجد باقات متوفرة للتجديد');
        return;
    }

    renewTargetUser = username;
    document.getElementById('renew-modal').classList.add('active');
}

function closeRenewModal() {
    renewTargetUser = '';
    document.getElementById('renew-paid').checked = false;
    document.getElementById('renew-modal').classList.remove('active');
}

function buildUserRow(u) {
    const s = u.session || {};
    const balanceClass = (u.balance || 0) > 0 ? 'badge-danger' : 'badge-success';
    const initials = getInitials(u.full_name, u.user);
    
    // Status Badge
    let stoppedBadge = '';
    if (!u.enabled) {
        stoppedBadge = `<span class="badge badge-danger" style="font-size:11px; margin-right:5px;">موقوف</span>`;
    }

    // Initials Avatar
    const avatar = `<div class="user-avatar-circle" style="width: 32px; height: 32px; border-radius: 50%; background: linear-gradient(135deg, var(--primary) 0%, var(--info) 100%); color: white; display: inline-flex; align-items: center; justify-content: center; font-size: 11px; font-weight: bold; margin-left: 8px;">${initials}</div>`;

    // Subscriber Details Info
    const subscriber = `
        <div style="display: flex; align-items: center;">
            ${avatar}
            <div style="text-align: right;">
                <strong style="display: block; font-size: 0.9rem; color: var(--text-main);">${escapeHtml(u.full_name || '—')}</strong>
                <span style="font-size: 0.75rem; color: var(--text-muted); font-family: monospace;">${escapeHtml(u.user)}</span>
            </div>
        </div>
    `;

    // Disconnect Button
    const disconnectBtn = (s.online || s.stale)
        ? `<button class="btn btn-danger" style="padding: 6px 10px; font-size: 11px; width:auto; border-radius: 6px;" title="فصل الجلسة" onclick="disconnectUser('${encodeURIComponent(u.user)}')">📡 فصل</button>`
        : '';

    // Actions Block
    const actions = `
        <div class="action-row" style="display: flex; gap: 6px; justify-content: flex-start; flex-wrap: nowrap;">
            <button class="btn btn-primary" style="padding: 6px 10px; font-size: 11px; width:auto; border-radius: 6px;" title="تجديد الاشتراك" onclick="openRenewModal('${encodeURIComponent(u.user)}')">🔄 تجديد</button>
            <button class="btn btn-edit" style="padding: 6px 10px; font-size: 11px; width:auto; border-radius: 6px;" title="تعديل" onclick="editUser('${encodeURIComponent(u.user)}')">✏️</button>
            <button class="btn" style="padding: 6px 10px; font-size: 11px; width:auto; border-radius: 6px; background: ${u.enabled ? 'var(--warning-light)' : 'var(--success-light)'}; color: ${u.enabled ? 'var(--warning)' : 'var(--success)'}; border: 1px solid ${u.enabled ? 'var(--warning)' : 'var(--success)'};" title="${u.enabled ? 'إيقاف المشترك' : 'تشغيل المشترك'}" onclick="toggleUserStatus('${encodeURIComponent(u.user)}')">
                ${u.enabled ? '🚫 إيقاف' : '✅ تشغيل'}
            </button>
            <button class="btn" style="padding: 6px 10px; font-size: 11px; width:auto; border-radius: 6px; background: var(--primary-light); color: var(--primary); border: 1px solid var(--border);" title="تفاصيل وتاريخ المشترك" onclick="openUserDetails('${encodeURIComponent(u.user)}')">📋</button>
            ${disconnectBtn}
            <button class="btn btn-danger" style="padding: 6px 10px; font-size: 11px; width:auto; border-radius: 6px;" title="حذف" onclick="deleteUser('${encodeURIComponent(u.user)}')">🗑️</button>
        </div>
    `;

    // Traffic Display
    const traffic = (s.download || s.upload)
        ? `<span style="font-size:11px; font-family:monospace; color:var(--text-muted); white-space: nowrap;">⬇️ ${escapeHtml(s.download || '0 B')} <br> ⬆️ ${escapeHtml(s.upload || '0 B')}</span>`
        : '<span style="color:var(--text-muted);">—</span>';

    // IP Display
    const ipDisplay = s.ip
        ? `<span class="device-link" onclick="openDeviceModal('${escapeHtml(s.ip)}')" title="فتح واجهة الجهاز" style="font-family:monospace; font-size:12px; color:var(--primary); cursor:pointer; text-decoration:underline;">
            ${escapeHtml(s.ip)} 🌐
           </span>`
        : '<span style="color:var(--text-muted);">—</span>';

    // Expiry Date styling
    const expiry = u.expires_at 
        ? `<span style="font-weight: 500; font-size:0.85rem;">${escapeHtml(u.expires_at)}</span>`
        : '<span style="color:var(--text-muted); font-size:0.85rem;">غير محدد</span>';

    return `
    <tr class="${u.enabled ? '' : 'disabled-row'}" data-user="${escapeHtml(u.user)}" style="${u.enabled ? '' : 'opacity: 0.7; background-color: #f8fafc;'}">
        <td>${subscriber}</td>
        <td><span style="font-family:monospace; background:var(--bg-app); padding:3px 8px; border-radius:6px; font-size:0.85rem; font-weight: 600;">${escapeHtml(u.pass)}</span></td>
        <td><span class="badge" style="background:rgba(14, 165, 233, 0.1); color:#0284c7; padding:4px 8px; border-radius:6px; font-weight:600; font-size:0.8rem;">${escapeHtml(u.profile || 'بدون باقة')}</span></td>
        <td>${expiry}</td>
        <td><span class="badge ${balanceClass}" style="padding:4px 8px; border-radius:6px; font-weight:bold; font-size:0.8rem;">${(u.balance || 0).toLocaleString()} د.ع</span></td>
        <td>${renderSessionStatus(u, s)} ${stoppedBadge}</td>
        <td>${traffic}</td>
        <td>${ipDisplay}</td>
        <td>${actions}</td>
    </tr>`;
}

async function loadUsers() {
    try {
        const res = await apiFetch('/radius/api/users');
        if (!res.ok) return;
        const users = await res.json();
        radiusUsersCache = users;

        const container = document.getElementById('users-tbody');
        if (!container) return;

        if (users.length === 0) {
            container.innerHTML = '<tr><td colspan="9" style="text-align:center; padding:30px; color:var(--text-muted);">لا يوجد مستخدمين مسجلين بالمنظومة...</td></tr>';
        } else {
            container.innerHTML = users.map(u => buildUserRow(u)).join('');
        }
        updateSearchBadge(users.length);
        updateDashboard(users);
    } catch (e) {
        console.error(e);
    }
}

function updateSearchBadge(count) {
    const badge = document.getElementById('users-search-badge');
    if (badge) badge.textContent = count || 0;
}

function filterUsers() {
    const input = document.getElementById('users-search-input');
    if (!input) return;
    const query = input.value.trim().toLowerCase();
    const tbody = document.getElementById('users-tbody');
    if (!tbody) return;

    let visibleCount = 0;

    if (!query) {
        tbody.querySelectorAll('tr').forEach(row => {
            row.style.display = '';
            visibleCount++;
        });
        updateSearchBadge(visibleCount);
        return;
    }

    tbody.querySelectorAll('tr').forEach(row => {
        const user = row.getAttribute('data-user') || '';
        const fullNameEl = row.querySelector('strong');
        const fullName = fullNameEl ? fullNameEl.textContent : '';
        const cachedUser = radiusUsersCache.find(u => u.user === user);
        const phone = cachedUser ? (cachedUser.phone || '') : '';

        const match = user.toLowerCase().includes(query)
            || fullName.toLowerCase().includes(query)
            || phone.toLowerCase().includes(query);

        if (match) {
            row.style.display = '';
            visibleCount++;
        } else {
            row.style.display = 'none';
        }
    });

    updateSearchBadge(visibleCount);
}

async function createUser() {
    const userName = document.getElementById('usr-name').value;
    const payload = {
        user: userName,
        old_user: editingUser || "", // Send old username if editing
        pass: document.getElementById('usr-pass').value,
        full_name: document.getElementById('usr-full-name').value,
        phone: document.getElementById('usr-phone').value,
        profile: document.getElementById('usr-profile').value,
        expires_at: document.getElementById('usr-expiry').value,
        admin_id: parseInt(document.getElementById('usr-owner')?.value || "0")
    };

    if (!payload.user || !payload.pass || !payload.profile) return alert("يرجى تعبئة كافة الحقول المطلوبة");

    try {
        const res = await apiFetch('/radius/api/users', {
            method: 'POST',
            body: JSON.stringify(payload)
        });

        const result = await res.json().catch(() => ({ error: "خطأ غير متوقع من الخادم" }));
        
        if (result.message && result.message.includes('نقل البيانات المالية')) {
            alert('✅ ' + result.message + '\n\nتم نقل الرصيد والديون والسجل المالي للمشترك بنجاح');
        } else {
            alert(result.message || result.error || "حدث خطأ غير معروف");
        }

        if (res.ok) {
            resetUserForm();
            loadUsers();
            closeUserModal();
        }
    } catch (e) {
        alert("فشل الاتصال بالخادم: " + e.message);
    }
}

async function submitRenewal() {
    if (!renewTargetUser) return;

    const profile = document.getElementById('renew-profile-select').value;
    if (!profile) {
        alert('يرجى اختيار باقة التجديد');
        return;
    }

    const paid = document.getElementById('renew-paid').checked;
    try {
        const res = await apiFetch(`/radius/api/users/${encodeURIComponent(renewTargetUser)}/renew`, {
            method: 'POST',
            body: JSON.stringify({ profile, paid })
        });

        const result = await res.json().catch(() => ({ error: "خطأ غير متوقع من الخادم" }));
        alert(result.message || result.error || "حدث خطأ غير معروف");

        if (res.ok) {
            closeRenewModal();
            loadUsers();
        }
    } catch (e) {
        alert("فشل الاتصال بالخادم: " + e.message);
    }
}

async function editUser(encodedUser) {
    const username = decodeURIComponent(encodedUser);
    const user = radiusUsersCache.find(item => item.user === username);
    if (!user) return;

    const nameInput = document.getElementById('usr-name');
    nameInput.value = user.user;
    nameInput.readOnly = false;
    nameInput.disabled = false;
    nameInput.placeholder = 'يمكن تعديل اسم المستخدم - البيانات المالية ستُنقل تلقائياً';
    document.getElementById('usr-pass').value = user.pass;
    document.getElementById('usr-full-name').value = user.full_name || '';
    document.getElementById('usr-phone').value = user.phone || '';
    document.getElementById('usr-profile').value = user.profile || '';
    const expiryEl = document.getElementById('usr-expiry');
    const expiryWrapper = document.getElementById('usr-expiry-wrapper');
    expiryEl.value = user.expires_at ? user.expires_at.replace(' ', 'T') : '';
    
    // Restriction: If editing existing user, agents cannot change expiration date
    if (currentAdmin && currentAdmin.role !== 'superadmin') {
        expiryWrapper.style.display = 'none';
    } else {
        expiryWrapper.style.display = '';
    }

    if (document.getElementById('usr-owner')) document.getElementById('usr-owner').value = user.admin_id || "0";
    document.getElementById('usr-submit-btn').innerText = 'تحديث المشترك';
    editingUser = username;

    document.getElementById('user-modal-title').innerText = 'تحديث المشترك: ' + username;
    document.getElementById('user-modal').classList.add('active');
}

async function disconnectUser(encodedUser) {
    const username = decodeURIComponent(encodedUser);
    if (!confirm(`هل تريد فصل جلسة المشترك "${username}" الآن؟`)) return;
    const res = await apiFetch(`/radius/api/users/${encodeURIComponent(username)}/disconnect`, { method: 'POST' });
    const result = await res.json();
    alert(result.message || result.error);
    if (res.ok) setTimeout(loadUsers, 1500);
}

async function deleteUser(encodedUser) {
    const username = decodeURIComponent(encodedUser);
    if (!confirm(`هل أنت متأكد من حذف المستخدم "${username}"؟`)) {
        return;
    }

    const res = await apiFetch(`/radius/api/users/${encodeURIComponent(username)}`, {
        method: 'DELETE'
    });

    const result = await res.json();
    alert(result.message || result.error);

    if (res.ok) {
        loadUsers();
    }
}
async function toggleUserStatus(encodedUser) {
    const username = decodeURIComponent(encodedUser);
    const user = radiusUsersCache.find(u => u.user === username);
    const action = user && user.enabled ? 'إيقاف' : 'تشغيل';

    if (!confirm(`هل أنت متأكد من ${action} المستخدم "${username}"؟`)) return;

    const res = await apiFetch(`/radius/api/users/${encodeURIComponent(username)}/toggle-status`, {
        method: 'POST'
    });

    const result = await res.json();
    if (res.ok) {
        loadUsers();
    } else {
        alert(result.error || 'حدث خطأ أثناء تغيير الحالة');
    }
}

/* ==========================================================
   Swipe / Panel interactions for user cards
   ========================================================== */
function toggleSwipePanel(safeId) {
    const panel = document.getElementById('swipe-panel-' + safeId);
    if (!panel) return;
    const isOpen = panel.classList.contains('open');
    closeAllSwipePanels();
    if (!isOpen) panel.classList.add('open');
}

function closeAllSwipePanels() {
    document.querySelectorAll('.user-card-swipe-panel').forEach(p => p.classList.remove('open'));
}

function initSwipeOnCards() {
    document.querySelectorAll('.user-card').forEach(card => {
        let startX = 0;
        let currentX = 0;
        let isDragging = false;
        const panel = card.querySelector('.user-card-swipe-panel');
        if (!panel) return;

        card.addEventListener('pointerdown', (e) => {
            if (e.target.closest('button, .icon-btn, .swipe-action, .device-link')) return;
            startX = e.clientX;
            isDragging = true;
            card.setPointerCapture(e.pointerId);
        });

        card.addEventListener('pointermove', (e) => {
            if (!isDragging) return;
            currentX = e.clientX;
            const diff = startX - currentX; // RTL: swipe left means diff > 0
            if (diff > 40) {
                closeAllSwipePanels();
                panel.classList.add('open');
            } else if (diff < -30) {
                panel.classList.remove('open');
            }
        });

        card.addEventListener('pointerup', () => {
            isDragging = false;
        });

        card.addEventListener('pointercancel', () => {
            isDragging = false;
        });
    });
}

// Close panels when clicking outside
document.addEventListener('click', (e) => {
    if (!e.target.closest('.user-card') && !e.target.closest('.user-card-desktop-reveal')) {
        closeAllSwipePanels();
    }
});

// Device Proxy Modal Control
function openDeviceModal(ip) {
    const modal = document.getElementById('device-proxy-modal');
    const iframe = document.getElementById('device-proxy-iframe');
    const loading = document.getElementById('device-proxy-loading');
    const title = document.getElementById('device-proxy-title');

    title.innerText = `📡 واجهة الجهاز: ${ip}`;
    iframe.style.display = 'none';
    loading.style.display = 'block';
    
    // Set source
    iframe.src = `/proxy/${ip}/`;
    
    modal.classList.add('active');

    iframe.onload = () => {
        loading.style.display = 'none';
        iframe.style.display = 'block';
    };
}

function closeDeviceModal() {
    const modal = document.getElementById('device-proxy-modal');
    const iframe = document.getElementById('device-proxy-iframe');
    
    modal.classList.remove('active');
    iframe.src = 'about:blank';
}

function openUserModal() {
    resetUserForm();
    document.getElementById('user-modal-title').innerText = 'إضافة مشترك جديد';
    document.getElementById('user-modal').classList.add('active');
}

function closeUserModal() {
    document.getElementById('user-modal').classList.remove('active');
}
