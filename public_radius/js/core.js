const radiusModuleState = {};
const radiusModules = {
    account: ['/radius/js/import.js?v=10'],
    profiles: ['/radius/js/profiles.js?v=10'],
    users: ['/radius/js/profiles.js?v=10', '/radius/js/transactions.js?v=10', '/radius/js/users.js?v=10'],
    vouchers: ['/radius/js/vouchers.js?v=10'],
    nas: ['/radius/js/nas.js?v=10'],
    streams: ['/radius/js/streams.js?v=10'],
    whatsapp: ['/radius/js/whatsapp.js?v=10'],
    logs: ['/radius/js/logs.js?v=10']
};

function loadRadiusScript(src) {
    if (radiusModuleState[src]) return radiusModuleState[src];
    radiusModuleState[src] = new Promise((resolve, reject) => {
        const script = document.createElement('script');
        script.src = src;
        script.defer = true;
        script.onload = resolve;
        script.onerror = () => reject(new Error(`Failed to load ${src}`));
        document.body.appendChild(script);
    });
    return radiusModuleState[src];
}

async function ensureTabModules(tabId) {
    const scripts = radiusModules[tabId] || [];
    for (const src of scripts) {
        await loadRadiusScript(src);
    }
}

async function showTab(tabId) {
    document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
    document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));

    const tab = document.getElementById(`tab-${tabId}`);
    if (tab) tab.classList.add('active');
    const btn = document.querySelector(`button[onclick="showTab('${tabId}')"]`);
    if (btn) btn.classList.add('active');

    localStorage.setItem('radius_active_tab', tabId);

    try {
        await ensureTabModules(tabId);
    } catch (e) {
        console.error(e);
        alert('تعذر تحميل هذا القسم. يرجى تحديث الصفحة والمحاولة مرة أخرى.');
        return;
    }

    // Dispatch event for other modules
    window.dispatchEvent(new CustomEvent('tabChanged', { detail: { tab: tabId } }));

    if (tabId === 'account' && typeof window.loadAdmins === 'function') window.loadAdmins();
    if (tabId === 'account' && typeof window.loadRadiusRemoteAccess === 'function') window.loadRadiusRemoteAccess();
    if (tabId === 'account' && typeof window.loadBypassStatus === 'function') window.loadBypassStatus();
    if (tabId === 'account' && typeof window.loadTelegramBackupConfig === 'function') window.loadTelegramBackupConfig();
    if (tabId === 'profiles' && typeof window.loadProfiles === 'function') window.loadProfiles();
    if (tabId === 'nas' && typeof window.loadNAS === 'function') window.loadNAS();
    if (tabId === 'vouchers' && typeof window.loadVouchers === 'function') window.loadVouchers();
    if (tabId === 'users' && typeof window.loadUsers === 'function') window.loadUsers();
    if (tabId === 'streams' && typeof window.loadStreams === 'function') window.loadStreams();
}

async function updateDashboard(usersData = null) {
    try {
        let users = usersData;
        if (!users) {
            const usersRes = await apiFetch('/radius/api/users');
            if (!usersRes.ok) return;
            users = await usersRes.json();
        }

        const stats = {
            total: users.length,
            active: users.filter(u => u.enabled).length,
            online: users.filter(u => u.session && u.session.online).length,
            expired: users.filter(u => u.session && (u.session.status === 'expired' || u.session.status === 'expired_online')).length,
            aboutToExpire: 0
        };

        const now = new Date();
        const threeDaysLater = new Date(now.getTime() + (3 * 24 * 60 * 60 * 1000));
        stats.aboutToExpire = users.filter(u => {
            if (!u.expires_at) return false;
            const exp = new Date(u.expires_at.replace(' ', 'T'));
            return exp > now && exp <= threeDaysLater;
        }).length;

        const setStat = (id, val) => {
            const el = document.getElementById(id);
            if (el) el.innerText = val;
        };

        setStat('stat-total-users', stats.total);
        setStat('stat-active-users', stats.active);
        setStat('stat-online-users', stats.online);
        setStat('stat-expired-users', stats.expired);
        setStat('stat-about-to-expire', stats.aboutToExpire);

        if (currentAdmin) {
            setStat('stat-balance', (currentAdmin.balance || 0).toLocaleString() + ' د.ع');
        }
    } catch (e) { console.error("Dashboard error", e); }
}

window.onload = async () => {
    await loadCurrentAdmin();
    await loadLicenseStatus();
    if (licenseState.valid) {
        document.getElementById('main-area').style.display = 'block';
        const lastTab = localStorage.getItem('radius_active_tab') || 'dashboard';
        await showTab(lastTab);

        if (lastTab === 'dashboard') {
            setTimeout(updateDashboard, 500);
        }
        if (typeof window.startUsersAutoRefresh === 'function') window.startUsersAutoRefresh();

        // تحميل جميع بيانات المشروع بالخلفية بدون استثناء بناءً على طلب المستخدم
        setTimeout(preloadAllData, 800);
    }
};

async function preloadAllData() {
    try {
        const tabsToLoad = Object.keys(radiusModules);
        for (const tab of tabsToLoad) {
            await ensureTabModules(tab);
        }
        
        // جلب جميع البيانات بدون استثناء بالخلفية
        if (typeof window.loadAdmins === 'function') window.loadAdmins();
        if (typeof window.loadRadiusRemoteAccess === 'function') window.loadRadiusRemoteAccess();
        if (typeof window.loadBypassStatus === 'function') window.loadBypassStatus();
        if (typeof window.loadTelegramBackupConfig === 'function') window.loadTelegramBackupConfig();
        if (typeof window.loadProfiles === 'function') window.loadProfiles();
        if (typeof window.loadNAS === 'function') window.loadNAS();
        if (typeof window.loadVouchers === 'function') window.loadVouchers();
        if (typeof window.loadUsers === 'function') window.loadUsers();
        if (typeof window.loadStreams === 'function') window.loadStreams();
        if (typeof window.loadWhatsappConfig === 'function') window.loadWhatsappConfig();
        if (typeof window.loadMessageTemplates === 'function') window.loadMessageTemplates();
        if (typeof window.fetchLogs === 'function') window.fetchLogs();
        
    } catch (e) {
        console.error("Failed to preload all data:", e);
    }
}

function openModal(id) {
    const el = document.getElementById(id);
    if (el) el.classList.add('active');
}

function closeModal(id) {
    const el = document.getElementById(id);
    if (el) el.classList.remove('active');
}

window.openModal = openModal;
window.closeModal = closeModal;

// ==========================================================================
// Deferred Module-Call Dispatcher
// ==========================================================================
// Inline onclick handlers fire before lazy-loaded scripts have finished.
// callWhenReady(scriptSrc, fnName) waits for the script to resolve, then
// invokes the named function on the window object. If the script is already
// loaded it calls immediately.
const _pendingCalls = {};

function callWhenReady(scriptSrc, fnName, ...args) {
    const state = radiusModuleState[scriptSrc];
    if (state instanceof Promise) {
        // Script is already being loaded – queue a single call for when it resolves
        if (!_pendingCalls[scriptSrc]) {
            _pendingCalls[scriptSrc] = [];
        }
        _pendingCalls[scriptSrc].push({ fnName, args });
        state.then(() => _flushPending(scriptSrc));
        return;
    }
    if (window[fnName] && typeof window[fnName] === 'function') {
        window[fnName](...args);
        return;
    }
    // Function not yet defined – load the script and queue the call
    loadRadiusScript(scriptSrc).then(() => _flushPending(scriptSrc));
}

function _flushPending(scriptSrc) {
    const pending = _pendingCalls[scriptSrc];
    if (!pending) return;
    delete _pendingCalls[scriptSrc];
    pending.forEach(({ fnName, args }) => {
        if (window[fnName] && typeof window[fnName] === 'function') {
            window[fnName](...args);
        } else {
            console.error(`[radius] ${fnName} is still undefined after ${scriptSrc} loaded.`);
            alert('جاري تحميل الوحدة المطلوبة، يرجى المحاولة مرة أخرى بعد ثوانٍ.');
        }
    });
}

// ==========================================================================
// Premium Toast Notification System
// ==========================================================================
function showToast(message, type = 'success', duration = 4000) {
    let container = document.getElementById('toast-container');
    if (!container) {
        container = document.createElement('div');
        container.id = 'toast-container';
        document.body.appendChild(container);
    }

    const toast = document.createElement('div');
    toast.className = `sas-toast ${type}`;

    let icon = '<i class="fa-solid fa-circle-check"></i>'; // Default success
    if (type === 'error') {
        icon = '<i class="fa-solid fa-circle-xmark"></i>';
    } else if (type === 'warning') {
        icon = '<i class="fa-solid fa-triangle-exclamation"></i>';
    } else if (type === 'info') {
        icon = '<i class="fa-solid fa-circle-info"></i>';
    }

    toast.innerHTML = `
        <div class="sas-toast-icon">${icon}</div>
        <div class="sas-toast-content">${message}</div>
    `;

    container.appendChild(toast);

    // Trigger animate in
    setTimeout(() => {
        toast.classList.add('show');
    }, 10);

    // Auto remove
    setTimeout(() => {
        toast.classList.remove('show');
        setTimeout(() => {
            toast.remove();
        }, 350);
    }, duration);
}

// Override native alert to use our premium toast system
window.alert = function (message) {
    if (!message) return;
    
    // Auto-detect type based on common keywords
    let type = 'info';
    const msgStr = message.toString();
    
    if (msgStr.includes('✅') || msgStr.includes('نجاح') || msgStr.includes('تم') || msgStr.includes('بنجاح') || msgStr.includes('مفعّل') || msgStr.includes('تحديث')) {
        type = 'success';
    } else if (msgStr.includes('❌') || msgStr.includes('فشل') || msgStr.includes('خطأ') || msgStr.includes('تعذر') || msgStr.includes('غير متوقع') || msgStr.includes('عطل')) {
        type = 'error';
    } else if (msgStr.includes('⚠️') || msgStr.includes('تنبيه') || msgStr.includes('تحذير') || msgStr.includes('تنبيه خطير')) {
        type = 'warning';
    }
    
    showToast(message, type);
};

window.showToast = showToast;
