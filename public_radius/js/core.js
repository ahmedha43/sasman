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
    resetRadiusContentScroll();

    try {
        await ensureTabModules(tabId);
    } catch (e) {
        console.error(e);
        alert('تعذر تحميل هذا القسم. يرجى تحديث الصفحة والمحاولة مرة أخرى.');
        return;
    }

    // Dispatch event for other modules
    window.dispatchEvent(new CustomEvent('tabChanged', { detail: { tab: tabId } }));

    if (tabId === 'dashboard' && typeof window.updateDashboard === 'function') window.updateDashboard();
    if (tabId === 'account' && typeof window.loadAdmins === 'function') window.loadAdmins();
    if (tabId === 'account' && typeof window.loadTunnelCardInfo === 'function') window.loadTunnelCardInfo();
    if (tabId === 'account' && typeof window.loadRadiusRemoteAccess === 'function') window.loadRadiusRemoteAccess();
    if (tabId === 'account' && typeof window.loadBypassStatus === 'function') window.loadBypassStatus();
    if (tabId === 'account' && typeof window.loadTelegramBackupConfig === 'function') window.loadTelegramBackupConfig();
    if (tabId === 'account' && typeof window.loadShutdownConfig === 'function') window.loadShutdownConfig();
    if (tabId === 'profiles' && typeof window.loadProfiles === 'function') window.loadProfiles();
    if (tabId === 'nas' && typeof window.loadNAS === 'function') window.loadNAS();
    if (tabId === 'vouchers' && typeof window.loadVouchers === 'function') window.loadVouchers();
    if (tabId === 'users' && typeof window.loadUsers === 'function') window.loadUsers();
    if (tabId === 'audit-logs' && typeof window.loadAuditLogs === 'function') window.loadAuditLogs(1);
    if (tabId === 'streams' && typeof window.loadStreams === 'function') window.loadStreams();
}

function resetRadiusContentScroll() {
    requestAnimationFrame(() => {
        const content = document.querySelector('.content-body');
        if (content) content.scrollTop = 0;
        window.scrollTo({ top: 0, behavior: 'auto' });
    });
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
        if (typeof window.loadTunnelCardInfo === 'function') window.loadTunnelCardInfo();
        if (typeof window.loadRadiusRemoteAccess === 'function') window.loadRadiusRemoteAccess();
        if (typeof window.loadBypassStatus === 'function') window.loadBypassStatus();
        if (typeof window.loadTelegramBackupConfig === 'function') window.loadTelegramBackupConfig();
        if (typeof window.loadShutdownConfig === 'function') window.loadShutdownConfig();
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

// ==========================================================================
// SASMAN Smart Broadcast & Notification Client Engine (Agent Panel)
// ==========================================================================
(function() {
    // Inject animation styles
    if (!document.getElementById('sas-bc-styles')) {
        const style = document.createElement('style');
        style.id = 'sas-bc-styles';
        style.textContent = `
            @keyframes sasSlideDown {
                from { transform: translate(-50%, -100%); opacity: 0; }
                to { transform: translate(-50%, 0); opacity: 1; }
            }
            @keyframes sasFadeIn {
                from { opacity: 0; transform: scale(0.95); }
                to { opacity: 1; transform: scale(1); }
            }
        `;
        document.head.appendChild(style);
    }

    async function checkAgentBroadcasts() {
        try {
            const res = await fetch('/radius/api/broadcasts/active');
            if (!res.ok) return;
            const broadcasts = await res.json();
            if (!Array.isArray(broadcasts) || broadcasts.length === 0) return;

            broadcasts.forEach(b => {
                if (!b || !b.id) return;
                
                // Only process agent-targeted broadcasts here (skip PPPoE-only broadcasts)
                const isForAgent = (!b.target_type || b.target_type === 'agents' || b.target_type === 'all' || b.target_type === 'both');
                if (!isForAgent) return;

                const seenKey = `sasman_bc_seen_${b.id}`;
                const lastSeenStr = localStorage.getItem(seenKey);

                if (b.frequency !== 'always' && lastSeenStr) {
                    const lastSeenTime = parseInt(lastSeenStr, 10);
                    if (b.frequency === 'once') return;
                    if (b.frequency === 'daily') {
                        const diffHours = (Date.now() - lastSeenTime) / (1000 * 60 * 60);
                        if (diffHours < 24) return;
                    }
                }

                if (b.display_type === 'modal') {
                    renderBroadcastModal(b);
                } else if (b.display_type === 'banner' || !b.display_type) {
                    renderBroadcastBanner(b);
                }
            });
        } catch (e) {
            console.debug('Broadcast check:', e);
        }
    }

    function renderBroadcastBanner(b) {
        if (document.getElementById(`sas-bc-banner-${b.id}`)) return;

        let container = document.getElementById('sas-broadcast-banner-container');
        if (!container) {
            container = document.createElement('div');
            container.id = 'sas-broadcast-banner-container';
            container.style.cssText = 'position:fixed; top:16px; left:50%; transform:translateX(-50%); z-index:999999; width:92%; max-width:960px; display:flex; flex-direction:column; gap:10px; pointer-events:none;';
            document.body.appendChild(container);
        }

        const banner = document.createElement('div');
        banner.id = `sas-bc-banner-${b.id}`;
        banner.style.cssText = 'pointer-events:auto; background:linear-gradient(135deg, rgba(15, 23, 42, 0.98), rgba(30, 41, 59, 0.98)); border:1.5px solid #0ea5e9; border-radius:14px; padding:14px 20px; box-shadow:0 12px 35px rgba(0,0,0,0.6); backdrop-filter:blur(14px); display:flex; justify-content:space-between; align-items:center; gap:14px; animation:sasSlideDown 0.35s ease; color:#e2e8f0; font-family:inherit; direction:rtl;';

        const actionBtn = (b.action_url && b.action_text) 
            ? `<a href="${b.action_url}" target="_blank" onclick="logBroadcastClick('${b.id}')" style="display:inline-flex; align-items:center; gap:6px; background:linear-gradient(135deg, #0ea5e9, #0284c7); color:#fff; padding:8px 18px; border-radius:8px; font-size:13px; font-weight:bold; text-decoration:none; white-space:nowrap; box-shadow:0 2px 10px rgba(14,165,233,0.3);">${escapeBcHtml(b.action_text)}</a>` 
            : '';

        banner.innerHTML = `
            <div style="display:flex; align-items:center; gap:12px; flex:1;">
                <span style="font-size:22px; filter:drop-shadow(0 0 8px rgba(14,165,233,0.5));">📢</span>
                <div>
                    <strong style="color:#38bdf8; font-size:14px; display:block; margin-bottom:2px;">${escapeBcHtml(b.title)}</strong>
                    <span style="font-size:13px; color:#cbd5e1; line-height:1.4;">${escapeBcHtml(b.message)}</span>
                </div>
            </div>
            <div style="display:flex; align-items:center; gap:10px;">
                ${actionBtn}
                <button onclick="dismissAgentBroadcast('${b.id}', 'banner')" style="background:rgba(255,255,255,0.08); border:none; color:#94a3b8; font-size:16px; cursor:pointer; padding:6px 10px; border-radius:8px; transition:all 0.2s;" onmouseover="this.style.color='#fff'; this.style.background='rgba(239,68,68,0.2)'" onmouseout="this.style.color='#94a3b8'; this.style.background='rgba(255,255,255,0.08)'" title="إغلاق">✕</button>
            </div>
        `;

        container.appendChild(banner);
        logBroadcastView(b.id);
    }

    function renderBroadcastModal(b) {
        if (document.getElementById(`sas-bc-modal-${b.id}`)) return;

        const overlay = document.createElement('div');
        overlay.id = `sas-bc-modal-${b.id}`;
        overlay.style.cssText = 'position:fixed; top:0; left:0; width:100%; height:100%; background:rgba(0,0,0,0.85); z-index:9999999; display:flex; justify-content:center; align-items:center; padding:20px; backdrop-filter:blur(8px); direction:rtl;';

        const imgHtml = b.image_url 
            ? `<div style="margin-bottom:14px; text-align:center;"><img src="${b.image_url}" style="max-width:100%; max-height:200px; border-radius:12px; border:1px solid #334155;"></div>` 
            : '';

        const actionBtn = (b.action_url && b.action_text) 
            ? `<a href="${b.action_url}" target="_blank" onclick="logBroadcastClick('${b.id}')" style="display:inline-flex; align-items:center; justify-content:center; background:linear-gradient(135deg, #0ea5e9, #0284c7); color:#fff; padding:12px 24px; border-radius:10px; font-size:14px; font-weight:bold; text-decoration:none; box-shadow:0 4px 15px rgba(14,165,233,0.4);">${escapeBcHtml(b.action_text)}</a>` 
            : '';

        overlay.innerHTML = `
            <div style="background:#0f172a; border:2px solid #0ea5e9; border-radius:20px; padding:28px; width:100%; max-width:540px; box-shadow:0 25px 60px rgba(0,0,0,0.8); text-align:center; color:#e2e8f0; font-family:inherit; animation:sasFadeIn 0.35s ease;">
                <div style="font-size:38px; margin-bottom:8px; filter:drop-shadow(0 0 10px rgba(14,165,233,0.5));">📢</div>
                <h3 style="color:#38bdf8; font-size:19px; font-weight:bold; margin-bottom:12px;">${escapeBcHtml(b.title)}</h3>
                ${imgHtml}
                <p style="font-size:14px; color:#cbd5e1; line-height:1.7; margin-bottom:24px; white-space:pre-wrap; text-align:right; background:rgba(30,41,59,0.5); padding:14px; border-radius:10px; border:1px solid rgba(255,255,255,0.05);">${escapeBcHtml(b.message)}</p>
                <div style="display:flex; justify-content:center; gap:12px; flex-wrap:wrap;">
                    ${actionBtn}
                    <button onclick="dismissAgentBroadcast('${b.id}', 'modal')" style="background:#334155; color:#fff; padding:12px 24px; border-radius:10px; font-size:14px; font-weight:bold; border:1px solid #475569; cursor:pointer; transition:all 0.2s;" onmouseover="this.style.background='#475569'" onmouseout="this.style.background='#334155'">تمت القراءة والمتابعة ✓</button>
                </div>
            </div>
        `;

        document.body.appendChild(overlay);
        logBroadcastView(b.id);
    }

    window.dismissAgentBroadcast = function(id, type) {
        localStorage.setItem(`sasman_bc_seen_${id}`, Date.now().toString());
        if (type === 'modal') {
            const el = document.getElementById(`sas-bc-modal-${id}`);
            if (el) el.remove();
        } else {
            const el = document.getElementById(`sas-bc-banner-${id}`);
            if (el) el.remove();
        }
    };

    window.logBroadcastClick = function(id) {
        fetch('/radius/api/broadcasts/log', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ broadcast_id: id, user_identifier: 'panel', clicked: 1 })
        }).catch(() => {});
    };

    function logBroadcastView(id) {
        fetch('/radius/api/broadcasts/log', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ broadcast_id: id, user_identifier: 'panel', clicked: 0 })
        }).catch(() => {});
    }

    function escapeBcHtml(str) {
        if (!str) return '';
        return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    }

    // Run on startup, on window focus, and poll every 4s
    setTimeout(checkAgentBroadcasts, 800);
    setInterval(checkAgentBroadcasts, 4000);
    window.addEventListener('focus', checkAgentBroadcasts);
})();
