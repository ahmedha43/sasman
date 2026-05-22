async function loadWhatsappConfig() {
    try {
        const res = await apiFetch('/radius/api/whatsapp/config');
        if (!res.ok) {
            const err = await res.json().catch(() => ({ error: 'Unknown server error' }));
            console.error('WhatsApp config error:', err.error);
            return;
        }
        const data = await res.json();
        
        setFieldValue('wa-enabled', data.enabled ? '1' : '0');
        setFieldValue('wa-phone', data.phone_number || '');
        setFieldChecked('wa-reminder-enabled', data.reminder_enabled === 1);
        setFieldValue('wa-reminder-hours', data.reminder_hours || 24);
        
        updateWAStatus(data.status);
    } catch (e) {
        console.error('Failed to load whatsapp config:', e);
    }
}

function setFieldValue(id, value) {
    const el = document.getElementById(id);
    if (el) el.value = value;
}

function setFieldChecked(id, checked) {
    const el = document.getElementById(id);
    if (el) el.checked = checked;
}

function updateWAStatus(status) {
    const badge = document.getElementById('wa-status-badge');
    const qrContainer = document.getElementById('wa-qr-container');
    const connectedBox = document.getElementById('wa-connected-box');
    const qrBtn = document.getElementById('wa-qr-btn');

    if (!badge || !qrContainer || !connectedBox || !qrBtn) return;

    if (status === 'connected') {
        badge.innerText = '✅ متصل';
        badge.className = 'badge badge-success';
        badge.style.color = '';
        qrContainer.style.display = 'none';
        connectedBox.style.display = 'block';
        qrBtn.style.display = 'none';
    } else if (status === 'waiting') {
        badge.innerText = '⏳ بانتظار اكتمال الربط';
        badge.className = 'badge badge-warning';
        badge.style.color = '';
        connectedBox.style.display = 'none';
        qrBtn.style.display = 'inline-block';
    } else {
        badge.innerText = '❌ غير متصل';
        badge.className = 'badge badge-danger';
        badge.style.color = '';
        connectedBox.style.display = 'none';
        qrBtn.style.display = 'inline-block';
    }
}

async function getWhatsappQR() {
    const btn = document.getElementById('wa-qr-btn');
    const badge = document.getElementById('wa-status-badge');
    const qrContainer = document.getElementById('wa-qr-container');
    const qrImg = document.getElementById('wa-qr-img');

    btn.disabled = true;
    btn.innerText = '⏳ جاري جلب الكود...';
    badge.innerText = '🔄 جاري الاتصال بالسيرفر...';

    try {
        const res = await apiFetch('/radius/api/whatsapp/qr');
        if (!res.ok) {
            const errData = await res.json().catch(() => ({ error: 'فشل الاتصال بالسيرفر' }));
            alert(errData.error);
            btn.disabled = false;
            btn.innerText = 'جلب كود الربط (QR)';
            return;
        }
        
        const data = await res.json();
        if (data.status === 'connected') {
            updateWAStatus('connected');
        } else if (data.status === 'qr' && data.qr) {
            qrImg.src = data.qr;
            qrContainer.style.display = 'block';
            btn.innerText = '🔄 تحديث الكود';
            btn.disabled = false;
            badge.innerText = '📸 امسح الكود الآن';
            startWAPolling();
        } else if (data.status === 'waiting') {
            updateWAStatus('waiting');
            startWAPolling();
            btn.disabled = false;
            btn.innerText = '🔄 تحديث الكود';
        } else {
            alert(data.error || 'فشل جلب كود QR');
            btn.disabled = false;
            btn.innerText = 'جلب كود الربط (QR)';
        }
    } catch (e) {
        alert('حدث خطأ غير متوقع');
        btn.disabled = false;
        btn.innerText = 'جلب كود الربط (QR)';
    }
}

let waPollInterval = null;
function startWAPolling() {
    if (waPollInterval) clearInterval(waPollInterval);
    waPollInterval = setInterval(async () => {
        try {
            const res = await apiFetch('/radius/api/whatsapp/config');
            if (res.ok) {
                const data = await res.json();
                if (data.status === 'connected') {
                    updateWAStatus('connected');
                    clearInterval(waPollInterval);
                }
            }
        } catch (e) {}
    }, 5000);
}

async function saveWhatsappConfig() {
    const payload = {
        enabled: parseInt(document.getElementById('wa-enabled').value),
        phone_number: document.getElementById('wa-phone').value.trim(),
        reminder_enabled: document.getElementById('wa-reminder-enabled').checked ? 1 : 0,
        reminder_hours: parseInt(document.getElementById('wa-reminder-hours').value) || 24
    };

    try {
        const res = await apiFetch('/radius/api/whatsapp/config', {
            method: 'POST',
            body: JSON.stringify(payload)
        });
        const result = await res.json().catch(() => ({ error: 'فشل حفظ الإعدادات' }));
        if (result.error) {
            alert(result.error);
        } else {
            alert(result.message || 'تم الحفظ بنجاح');
            loadWhatsappConfig();
        }
    } catch (e) {
        alert('خطأ في الاتصال');
    }
}

async function loadMessageTemplates() {
    try {
        const res = await apiFetch('/radius/api/whatsapp/templates');
        if (!res.ok) return;
        const templates = await res.json();
        renderTemplates(templates);
    } catch (e) {
        console.error('Failed to load templates', e);
    }
}

function renderTemplates(templates) {
    const container = document.getElementById('templates-list');
    if (!container) return;

    const labels = {
        'renew_paid': '🔄 تجديد — مدفوع',
        'renew_debt': '🔄 تجديد — ديون',
        'add_debt':   '➕ إضافة ديون',
        'payment':    '➖ تسديد ديون',
        'expiry_reminder': '⚠️ تنبيه انتهاء الاشتراك'
    };

    if (!templates || templates.length === 0) {
        container.innerHTML = '<p style="color:var(--text-muted);">لا توجد قوالب...</p>';
        return;
    }

    container.innerHTML = templates.map(t => {
        const label = labels[t.template_key] || t.template_key;
        return `
            <div style="margin-bottom:16px; padding:12px; background:var(--bg-app); border-radius:8px; border:1px solid var(--border);">
                <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:8px;">
                    <strong style="color:var(--text-main);">${label}</strong>
                    <span class="badge badge-secondary" style="font-size:12px; padding:2px 8px; border-radius:4px;">${t.template_key}</span>
                </div>
                <textarea id="template-${t.template_key}" rows="4" style="width:100%; font-family:inherit; font-size:14px; resize:vertical;">${escapeHtml(t.template_text)}</textarea>
                <button class="btn btn-primary" style="margin-top:8px; width:auto; padding:6px 16px;" onclick="saveTemplate('${t.template_key}')">💾 حفظ القالب</button>
            </div>
        `;
    }).join('');
}

async function saveTemplate(key) {
    const text = document.getElementById(`template-${key}`).value;
    const res = await apiFetch('/radius/api/whatsapp/templates', {
        method: 'POST',
        body: JSON.stringify({ template_key: key, template_text: text })
    });
    const result = await res.json();
    alert(result.message || result.error);
}

async function testWhatsappNotification() {
    const phone = prompt('أدخل رقم الهاتف للاختبار (صيغة دولية بدون +):', document.getElementById('wa-phone').value.trim());
    if (!phone) return;

    const res = await apiFetch('/radius/api/whatsapp/test', {
        method: 'POST',
        body: JSON.stringify({ phone, message: '🔔 هذا اختبار من نظام SASMAN RADIUS' })
    });

    const result = await res.json();
    alert(result.message || result.error);
}

async function logoutWhatsapp() {
    if (!confirm('هل أنت متأكد من تسجيل الخروج وفصل الواتساب؟')) return;
    
    try {
        const res = await apiFetch('/radius/api/whatsapp/logout', { method: 'POST' });
        const result = await res.json();
        alert(result.message || result.error);
        loadWhatsappConfig();
    } catch (e) {
        alert('فشل تسجيل الخروج');
    }
}

// Load config when whatsapp tab is opened
window.addEventListener('tabChanged', (e) => {
    if (e.detail.tab === 'whatsapp') {
        loadWhatsappConfig();
        loadMessageTemplates();
    } else {
        if (waPollInterval) clearInterval(waPollInterval);
    }
});
