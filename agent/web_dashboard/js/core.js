let isLicensed = false;
let remoteBaseURL = "";

// Premium Toast Notification System
function getOrCreateToastContainer() {
    let container = document.getElementById('toast-container');
    if (!container) {
        container = document.createElement('div');
        container.id = 'toast-container';
        document.body.appendChild(container);
    }
    return container;
}

function showToast(message, type = 'success') {
    const container = getOrCreateToastContainer();
    const toast = document.createElement('div');
    toast.className = `sas-toast ${type}`;
    
    let icon = '<i class="fa-solid fa-circle-check"></i>';
    if (type === 'error') icon = '<i class="fa-solid fa-circle-xmark"></i>';
    if (type === 'warning') icon = '<i class="fa-solid fa-circle-exclamation"></i>';
    if (type === 'info') icon = '<i class="fa-solid fa-circle-info"></i>';
    
    toast.innerHTML = `
        <div class="sas-toast-icon">${icon}</div>
        <div class="sas-toast-message">${message}</div>
    `;
    container.appendChild(toast);
    
    // Slide-in transition trigger
    setTimeout(() => {
        toast.classList.add('show');
    }, 50);
    
    // Slide-out and remove
    setTimeout(() => {
        toast.classList.remove('show');
        setTimeout(() => {
            toast.remove();
        }, 350);
    }, 3500);
}

// Remote Access URL functions (Ngrok only)
async function loadRemoteAccess() {
    try {
        const response = await fetch('/api/ngrok/token');
        if (!response.ok) return;
        const data = await response.json();

        const section = document.getElementById('cloudflared-section');
        const loading = document.getElementById('cloudflared-loading');
        const hint = document.getElementById('cloudflared-hint');
        const buttons = document.getElementById('cloudflared-buttons');
        const winboxSection = document.getElementById('winbox-section');
        const winboxURL = document.getElementById('winbox-url');

        if (!section) return;

        // Check Ngrok status via a separate status endpoint if available
        const statusRes = await fetch('/api/cloudflared/url').catch(() => null);
        if (!statusRes || !statusRes.ok) return;
        const statusData = await statusRes.json();

        if (statusData.enabled && statusData.ngrok && statusData.ngrok.web) {
            section.style.display = 'block';
            remoteBaseURL = statusData.ngrok.web;

            if (winboxSection && statusData.ngrok.tcp) {
                winboxSection.style.display = 'flex';
                if (winboxURL) winboxURL.innerText = statusData.ngrok.tcp;
            }

            const configSection = document.getElementById('ngrok-config-section');
            if (configSection) configSection.style.display = 'none';

            showTunnels(loading, hint, buttons, true);
        } else if (statusData.ngrok) {
            // Ngrok token exists but not connected yet — show config
            section.style.display = 'block';
            const configSection = document.getElementById('ngrok-config-section');
            if (configSection) configSection.style.display = 'flex';
            showTunnels(loading, hint, buttons, false);
        } else {
            section.style.display = 'none';
        }
    } catch (error) {
        console.log('Remote access lookup failed:', error);
    }
}

function showTunnels(loading, hint, buttons, ready) {
    if (loading) loading.style.display = ready ? 'none' : 'block';
    if (hint) hint.style.display = ready ? 'block' : 'none';
    if (buttons) buttons.style.display = ready ? 'flex' : 'none';
}

function copyPathLink(path) {
    if (!remoteBaseURL) {
        return showToast('يرجى الانتظار حتى يتم تجهيز الرابط المباشر...', 'warning');
    }
    const cleanBase = remoteBaseURL.replace(/\/$/, "");
    const fullURL = cleanBase + path;
    copyToClipboard(fullURL);
}

function copyToClipboard(text) {
    if (!text || text === '---') return;

    if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(() => {
            showToast('تم نسخ الرابط الحافظة بنجاح 📋', 'success');
        }).catch(err => {
            console.error('Clipboard API failed, using fallback:', err);
            fallbackCopyText(text);
        });
    } else {
        fallbackCopyText(text);
    }
}

function fallbackCopyText(text) {
    const textArea = document.createElement("textarea");
    textArea.value = text;
    textArea.style.position = "fixed";
    textArea.style.left = "-9999px";
    textArea.style.top = "0";
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();
    try {
        const successful = document.execCommand('copy');
        if (successful) {
            showToast('تم نسخ الرابط بنجاح 📋', 'success');
        } else {
            showToast('فشل نسخ الرابط تلقائياً، يرجى النسخ يدوياً.', 'error');
        }
    } catch (err) {
        showToast('فشل نسخ الرابط تلقائياً، يرجى النسخ يدوياً.', 'error');
    }
    document.body.removeChild(textArea);
}

async function checkAuth() {
    try {
        const res = await fetch('/api/auth/status');
        if (res.status === 401) {
            lockUI();
            return;
        }
        const status = await res.json();

        // Populate fields with current session data
        if (status.address) document.getElementById('login-ip').value = status.address;
        if (status.user) document.getElementById('login-user').value = status.user;

        if (status.authenticated) {
            unlockUI();
            await checkLicense();
            if (isLicensed) {
                loadAll();
                const lastTab = localStorage.getItem('sasman_active_tab') || 'wan';
                showTab(lastTab);
            } else {
                showTab('license');
            }
        } else {
            lockUI();
        }
    } catch (e) {
        lockUI();
    }
}

async function checkLicense() {
    try {
        const res = await fetch('/api/license/status');
        const data = await res.json();

        const badge = document.getElementById('license-status-badge');
        const serialEl = document.getElementById('license-serial');
        const expiryEl = document.getElementById('license-expiry-text');
        const main = document.getElementById('main-container');

        serialEl.innerText = data.serial || 'غير متوفر';

        if (data.valid) {
            isLicensed = true;
            badge.innerHTML = '<i class="fa-solid fa-circle-check" style="margin-left:4px;"></i> نظام مفعل ✅';
            badge.className = "license-badge license-valid";
            expiryEl.innerText = "تاريخ الانتهاء: " + data.expires;
            main.classList.remove('locked');
        } else {
            isLicensed = false;
            badge.innerHTML = '<i class="fa-solid fa-circle-xmark" style="margin-left:4px;"></i> نظام غير مفعل ❌';
            badge.className = "license-badge license-invalid";
            expiryEl.innerText = "يرجى إدخال كود تنشيط صالح للسيريال أعلاه.";
            main.classList.add('locked');
        }
    } catch (e) {
        console.error("License check failed", e);
    }
}

async function activateLicense() {
    const key = document.getElementById('license-key-input').value.trim();
    if (!key) return showToast("يرجى إدخال كود التنشيط أولاً", "warning");

    const msgEl = document.getElementById('activation-msg');
    msgEl.innerHTML = `<i class="fa-solid fa-spinner fa-spin" style="margin-left:6px;"></i> جاري التحقق...`;
    msgEl.style.color = "#475569";

    try {
        const res = await fetch('/api/license/activate', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ key: key })
        });
        const result = await res.json();

        if (res.ok) {
            msgEl.innerHTML = `<i class="fa-solid fa-circle-check" style="margin-left:6px;"></i> ${result.message}`;
            msgEl.style.color = "#10b981";
            showToast("تم ترخيص وتفعيل النظام بنجاح! 🔑", "success");
            setTimeout(() => {
                window.location.reload();
            }, 1500);
        } else {
            msgEl.innerHTML = `<i class="fa-solid fa-circle-xmark" style="margin-left:6px;"></i> ${result.error}`;
            msgEl.style.color = "#ef4444";
            showToast(result.error, "error");
        }
    } catch (e) {
        msgEl.innerHTML = `<i class="fa-solid fa-triangle-exclamation" style="margin-left:6px;"></i> فشل الاتصال بالسيرفر`;
        msgEl.style.color = "#ef4444";
        showToast("فشل الاتصال بالسيرفر", "error");
    }
}

async function login() {
    const payload = {
        address: document.getElementById('login-ip').value,
        user: document.getElementById('login-user').value,
        pass: document.getElementById('login-pass').value
    };
    const btn = document.querySelector('#login-overlay .btn');
    btn.innerHTML = `<i class="fa-solid fa-spinner fa-spin" style="margin-left:8px;"></i> جاري الاتصال بالراوتر...`;
    btn.disabled = true;

    try {
        const res = await fetch('/api/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        const result = await res.json();
        if (res.ok) {
            unlockUI();
            showToast("تم الاتصال بالراوتر بنجاح! 🔌", "success");
            await checkLicense();
            if (isLicensed) {
                loadAll();
            } else {
                showTab('license');
            }
        } else {
            document.getElementById('login-error').innerText = result.error;
            showToast(result.error, "error");
            btn.innerHTML = `<i class="fa-solid fa-plug"></i> اتصال بالراوتر ودخول`;
            btn.disabled = false;
        }
    } catch (e) {
        document.getElementById('login-error').innerText = "فشل الاتصال بخادم الإدارة";
        showToast("فشل الاتصال بخادم الإدارة", "error");
        btn.innerHTML = `<i class="fa-solid fa-plug"></i> اتصال بالراوتر ودخول`;
        btn.disabled = false;
    }
}

function unlockUI() {
    document.getElementById('login-overlay').style.display = 'none';
    document.getElementById('main-container').classList.remove('locked');
}

function lockUI() {
    document.getElementById('login-overlay').style.display = 'flex';
    document.getElementById('main-container').classList.add('locked');
}

async function logout() {
    await fetch('/api/logout', { method: 'POST' });
    showToast("تم تسجيل الخروج بنجاح", "info");
    location.reload();
}

async function loadAll() {
    await loadApps();
    await loadInterfaces();
    await loadWanStatus();
    await loadRoutingStatus();
    await loadNgrokToken();
    await loadTunnelSettings();
    await loadRemoteAccess();
}

function showTab(tabId) {
    document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
    document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
    
    const content = document.getElementById(`tab-${tabId}`);
    if (content) content.classList.add('active');
    
    const btn = document.querySelector(`button[onclick="showTab('${tabId}')"]`);
    if (btn) btn.classList.add('active');
    
    localStorage.setItem('sasman_active_tab', tabId);

    // Lazy load MikroTik WebFig when tab is selected
    if (tabId === 'mikrotik') {
        const iframe = document.getElementById('mikrotik-iframe');
        if (iframe && (iframe.src === 'about:blank' || !iframe.src.includes('/mikrotik'))) {
            iframe.src = '/mikrotik';
        }
    }
}

async function purgeSystem() {
    if (!confirm('هل أنت متأكد من مسح وتصفير جميع إعدادات موازنة ودمج النظام؟')) return;
    try {
        const res = await fetch('/api/purge', { method: 'DELETE' });
        const result = await res.json();
        if (res.ok) {
            showToast("تم تنظيف وتصفير المنظومة بنجاح! 🗑", "success");
        } else {
            showToast(result.error, "error");
        }
        loadInterfaces();
    } catch (e) {
        showToast("فشل الاتصال بالسيرفر", "error");
    }
}

window.onload = checkAuth;

// Load Remote Access status on page load
document.addEventListener('DOMContentLoaded', loadRemoteAccess);

// Interval for monitoring (every 5 seconds)
setInterval(() => {
    if (isLicensed) {
        loadWanStatus();
        loadRoutingStatus();
    }
    loadRemoteAccess();
}, 5000);

async function loadNgrokToken() {
    try {
        const res = await fetch('/api/ngrok/token');
        const data = await res.json();
        if (data.token) {
            document.getElementById('ngrok-token-input').value = data.token;
        }
    } catch (e) { }
}

async function loadTunnelSettings() {
    try {
        const res = await fetch('/api/tunnel/settings');
        const data = await res.json();
        if (res.ok) {
            document.getElementById('tunnel-mode-input').value = data.mode || '';
            document.getElementById('tunnel-subdomain-input').value = data.subdomain || '';
            document.getElementById('tunnel-token-input').value = data.token || '';
            document.getElementById('tunnel-gateway-input').value = data.gateway_url || '';
        }
    } catch (e) { }
}

async function saveTunnelSettings() {
    const payload = {
        mode: document.getElementById('tunnel-mode-input').value.trim(),
        subdomain: document.getElementById('tunnel-subdomain-input').value.trim(),
        token: document.getElementById('tunnel-token-input').value.trim(),
        gateway_url: document.getElementById('tunnel-gateway-input').value.trim()
    };

    const msgEl = document.getElementById('tunnel-settings-msg');
    msgEl.innerHTML = '<i class="fa-solid fa-spinner fa-spin" style="margin-left:6px;"></i> جاري الحفظ...';

    try {
        const res = await fetch('/api/tunnel/settings', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        const data = await res.json();
        if (res.ok) {
            msgEl.innerHTML = '<i class="fa-solid fa-circle-check" style="margin-left:6px;"></i> ' + (data.message || 'تم الحفظ');
            msgEl.style.color = '#10b981';
            showToast('تم حفظ إعدادات Tunnel بنجاح', 'success');
        } else {
            msgEl.innerHTML = '<i class="fa-solid fa-circle-xmark" style="margin-left:6px;"></i> ' + (data.error || 'فشل الحفظ');
            msgEl.style.color = '#ef4444';
            showToast(data.error || 'فشل الحفظ', 'error');
        }
    } catch (e) {
        msgEl.innerHTML = '<i class="fa-solid fa-triangle-exclamation" style="margin-left:6px;"></i> فشل الاتصال بالسيرفر';
        msgEl.style.color = '#ef4444';
        showToast('فشل الاتصال بالسيرفر', 'error');
    }
}

async function saveNgrokToken() {
    const token = document.getElementById('ngrok-token-input').value.trim();
    if (!token) return showToast('يرجى إدخال التوكن أولاً', 'warning');

    try {
        const res = await fetch('/api/ngrok/token', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ token: token })
        });
        const data = await res.json();
        if (res.ok) {
            showToast('تم حفظ وتشغيل توكن Ngrok بنجاح! 🚀', 'success');
        } else {
            showToast(data.error, 'error');
        }
        loadRemoteAccess(); // Re-check status
    } catch (e) {
        showToast('فشل اتصال حفظ التوكن بالخادم', 'error');
    }
}
