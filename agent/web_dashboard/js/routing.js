// ===== Multi-Gateway Dropdown State =====
let selectedGateways = [];
let allGatewayOptions = [];

function buildGwOptions(interfaces) {
    allGatewayOptions = interfaces.map(i => i.name);
    renderGwOptions(allGatewayOptions);
}

function renderGwOptions(list) {
    const container = document.getElementById('gw-options-list');
    if (!container) return;

    if (list.length === 0) {
        container.innerHTML = '<div style="padding:16px; text-align:center; color:var(--text-muted); font-size:13px;"><i class="fa-solid fa-magnifying-glass" style="margin-left:6px;"></i>لا توجد نتائج مطابقة</div>';
        return;
    }

    container.innerHTML = list.map(name => {
        const isSelected = selectedGateways.includes(name);
        // Detect icon based on name
        let icon = 'fa-ethernet';
        if (name.startsWith('pppoe') || name.startsWith('ppp')) icon = 'fa-plug';
        else if (name.startsWith('wlan') || name.startsWith('wifi')) icon = 'fa-wifi';
        else if (name.startsWith('l2tp') || name.startsWith('sstp')) icon = 'fa-lock';
        else if (name.startsWith('vlan')) icon = 'fa-layer-group';

        return `
            <div class="gw-option ${isSelected ? 'selected' : ''}" onclick="toggleGateway('${name}')">
                <input type="checkbox" ${isSelected ? 'checked' : ''} onclick="event.stopPropagation(); toggleGateway('${name}')">
                <div class="iface-icon">
                    <i class="fa-solid ${icon}"></i>
                </div>
                <span>${name}</span>
                ${isSelected ? '<span style="margin-right:auto; color:var(--primary); font-size:11px;"><i class="fa-solid fa-check"></i> محدد</span>' : ''}
            </div>
        `;
    }).join('');
}

function toggleGateway(name) {
    const idx = selectedGateways.indexOf(name);
    if (idx === -1) {
        selectedGateways.push(name);
    } else {
        selectedGateways.splice(idx, 1);
    }
    const searchVal = document.getElementById('gw-search')?.value || '';
    const filtered = allGatewayOptions.filter(n => n.toLowerCase().includes(searchVal.toLowerCase()));
    renderGwOptions(filtered);
    updateGwTrigger();
}

function updateGwTrigger() {
    const trigger = document.getElementById('gw-trigger');
    const placeholder = document.getElementById('gw-placeholder');
    if (!trigger || !placeholder) return;

    // Remove existing tags (keep placeholder and caret)
    trigger.querySelectorAll('.gw-tag').forEach(t => t.remove());

    if (selectedGateways.length === 0) {
        placeholder.style.display = '';
        placeholder.textContent = 'اختر منفذ أو أكثر...';
    } else {
        placeholder.style.display = 'none';
        selectedGateways.forEach(name => {
            const tag = document.createElement('span');
            tag.className = 'gw-tag';
            tag.innerHTML = `<i class="fa-solid fa-right-long" style="font-size:10px;"></i>${name}<span class="gw-tag-remove" onclick="event.stopPropagation(); toggleGateway('${name}')">×</span>`;
            trigger.insertBefore(tag, trigger.querySelector('.caret'));
        });
    }
}

function toggleGwDropdown() {
    const dropdown = document.getElementById('gw-dropdown');
    const trigger = document.getElementById('gw-trigger');
    if (!dropdown || !trigger) return;
    const isOpen = dropdown.classList.toggle('open');
    trigger.classList.toggle('open', isOpen);
    if (isOpen) {
        document.getElementById('gw-search')?.focus();
    }
}

function filterGwOptions(query) {
    const filtered = allGatewayOptions.filter(n => n.toLowerCase().includes(query.toLowerCase()));
    renderGwOptions(filtered);
}

function selectAllGateways() {
    selectedGateways = [...allGatewayOptions];
    const searchVal = document.getElementById('gw-search')?.value || '';
    const filtered = allGatewayOptions.filter(n => n.toLowerCase().includes(searchVal.toLowerCase()));
    renderGwOptions(filtered);
    updateGwTrigger();
}

function clearAllGateways() {
    selectedGateways = [];
    const searchVal = document.getElementById('gw-search')?.value || '';
    const filtered = allGatewayOptions.filter(n => n.toLowerCase().includes(searchVal.toLowerCase()));
    renderGwOptions(filtered);
    updateGwTrigger();
}

// Close dropdown when clicking outside
document.addEventListener('click', function (e) {
    const wrapper = document.getElementById('gw-wrapper');
    if (wrapper && !wrapper.contains(e.target)) {
        document.getElementById('gw-dropdown')?.classList.remove('open');
        document.getElementById('gw-trigger')?.classList.remove('open');
    }
});

// ===== Routing Status =====
async function loadApps() {
    if (!isLicensed) return;
    try {
        const res = await fetch('/api/routing/list');
        const apps = await res.json();
        if (!res.ok) return;
        const list = document.getElementById('app-list');
        list.innerHTML = apps.map(app => `<option value="${app}">${app}</option>`).join('');
    } catch (e) { }
}

async function loadRoutingStatus() {
    if (!isLicensed) return;
    const tbody = document.getElementById('routing-status-tbody');
    try {
        const res = await fetch('/api/status/routing');
        const data = await res.json();
        if (data.length === 0) {
            tbody.innerHTML = '<tr><td colspan="4" style="padding:30px; text-align:center; color:var(--text-muted); font-size:0.95rem;"><i class="fa-solid fa-circle-exclamation" style="margin-left:6px;"></i>لا توجد عمليات توجيه ذكي نشطة حالياً</td></tr>';
            return;
        }
        tbody.innerHTML = data.map(route => {
            const badgeClass = route.enabled ? 'badge-success' : 'badge-danger';
            const badgeText = route.enabled ? 'مفعل' : 'معطل';
            // Support multiple gateways display
            const gwDisplay = Array.isArray(route.gateways) && route.gateways.length > 0
                ? route.gateways.map(g => `<span class="badge badge-secondary" style="font-size:11px; margin-left:4px;">${g}</span>`).join('')
                : `<span style="color:var(--primary); font-weight:bold; font-family:monospace;">${route.gateway}</span>`;

            return `
                <tr>
                    <td style="padding:16px 20px;">
                        <strong style="font-size:1.05rem; color:var(--text-main);"><i class="fa-solid fa-cube" style="color:var(--primary); margin-left:8px;"></i>${route.app}</strong><br/>
                        <small style="color:var(--text-muted); font-size:0.8rem; margin-top:4px; display:inline-block;"><i class="fa-solid fa-hashtag" style="margin-left:4px; font-size:10px;"></i>${route.comment || ''}</small>
                    </td>
                    <td style="padding:16px 20px;">
                        <i class="fa-solid fa-right-long" style="margin-left:6px; font-size:12px; color:var(--primary);"></i>${gwDisplay}
                    </td>
                    <td style="padding:16px 20px;">
                        <span class="badge ${badgeClass}">
                            ${badgeText}
                        </span>
                    </td>
                    <td style="padding:16px 20px;">
                        <div class="action-row">
                            <button class="btn" onclick="toggleRoutingManual('${route.id}', ${route.enabled})" 
                                style="width:auto; padding:8px 14px; font-size:12px; border:none; color:white; background:${route.enabled ? '#64748b' : 'var(--success)'};">
                                <i class="fa-solid ${route.enabled ? 'fa-toggle-off' : 'fa-toggle-on'}"></i> ${route.enabled ? 'تعطيل' : 'تفعيل'}
                            </button>
                            <button class="btn btn-delete btn-danger" onclick="removeRoutingManual('${route.app}')" 
                                style="width:auto; padding:8px 14px; font-size:12px;">
                                <i class="fa-solid fa-trash-can"></i> حذف
                            </button>
                        </div>
                    </td>
                </tr>
            `;
        }).join('');

    } catch (e) {
        tbody.innerHTML = '<tr><td colspan="4" style="padding:30px; text-align:center; color:var(--danger); font-weight:bold;"><i class="fa-solid fa-triangle-exclamation" style="margin-left:6px;"></i>خطأ في تحميل بيانات التوجيه</td></tr>';
    }
}

async function toggleRoutingManual(id, currentEnabled) {
    try {
        const res = await fetch('/api/routing/toggle', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id: id, disabled: !currentEnabled })
        });
        if (res.ok) {
            showToast(`تم ${currentEnabled ? 'تعطيل' : 'تفعيل'} التوجيه بنجاح`, "success");
        } else {
            showToast("فشل تعديل حالة التوجيه", "error");
        }
        loadRoutingStatus();
    } catch (e) {
        showToast("خطأ في الاتصال بالخادم", "error");
    }
}

async function removeRoutingManual(app) {
    if (!confirm(`هل أنت متأكد من حذف توجيه ${app}؟`)) return;
    try {
        const res = await fetch('/api/routing/remove', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ target: app })
        });
        const result = await res.json();
        if (res.ok) {
            showToast(`تم حذف توجيه تطبيق ${app} بنجاح!`, "success");
        } else {
            showToast(result.error, "error");
        }
        loadAll();
    } catch (e) {
        showToast("فشل الاتصال بخادم التوجيه", "error");
    }
}

async function applyRouting() {
    const appName = document.getElementById('app-list').value;

    if (selectedGateways.length === 0) {
        return showToast("يرجى اختيار خط خروج واحد على الأقل من القائمة!", "warning");
    }

    const payload = {
        target: appName,
        gateways: selectedGateways   // Send array instead of single string
    };

    try {
        const res = await fetch('/api/routing/apply', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        const result = await res.json();
        if (res.ok) {
            const gwList = selectedGateways.join('، ');
            showToast(`تم توجيه حركة ${appName} عبر: ${gwList} بنجاح! 🚀`, "success");
        } else {
            showToast(result.error, "error");
        }
        loadRoutingStatus();
    } catch (e) {
        showToast("فشل تطبيق قاعدة التوجيه بالخادم", "error");
    }
}

async function removeRouting() {
    const appName = document.getElementById('app-list').value;
    const payload = {
        target: appName
    };
    try {
        const res = await fetch('/api/routing/remove', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        const result = await res.json();
        if (res.ok) {
            showToast(`تم إزالة مسار التوجيه لـ ${appName} بنجاح`, "success");
        } else {
            showToast(result.error, "error");
        }
        loadRoutingStatus();
    } catch (e) {
        showToast("فشل إزالة مسار التوجيه من الخادم", "error");
    }
}

async function purgeRouting() {
    if (!confirm('هل أنت متأكد من مسح جميع قواعد التوجيه الذكي؟')) return;
    try {
        const res = await fetch('/api/routing/purge', { method: 'DELETE' });
        const result = await res.json();
        if (res.ok) {
            showToast("تم مسح وتصفير كافة قواعد التوجيه الذكي بنجاح! 🗑", "success");
        } else {
            showToast(result.error, "error");
        }
        loadRoutingStatus();
    } catch (e) {
        showToast("فشل الاتصال بالسيرفر لتصفير التوجيه", "error");
    }
}

async function applyBlock() {
    const payload = {
        name: document.getElementById('block-name').value,
        domains: document.getElementById('block-domains').value.split('\n').map(d => d.trim()).filter(d => d.length > 0)
    };
    try {
        const res = await fetch('/api/routing/block', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        const result = await res.json();
        if (res.ok) {
            showToast("تم تطبيق قاعدة الحظر بنجاح 🚫", "success");
        } else {
            showToast(result.error, "error");
        }
    } catch (e) {
        showToast("فشل الاتصال بالسيرفر لتطبيق الحظر", "error");
    }
}
