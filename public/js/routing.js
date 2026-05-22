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
            tbody.innerHTML = '<tr><td colspan="4" style="padding:20px; text-align:center; color:#64748b;">لا توجد عمليات توجيه نشطة</td></tr>';
            return;
        }
        tbody.innerHTML = data.map(route => `
            <tr style="border-bottom:1px solid #e2e8f0;">
                <td style="padding:12px;">
                    <strong>${route.app}</strong><br/>
                    <small style="color:#64748b;">${route.comment || ''}</small>
                </td>
                <td style="padding:12px;"><span style="color:#4f46e5; font-weight:bold;">${route.gateway}</span></td>
                <td style="padding:12px;">
                    <span style="background:${route.enabled ? '#dcfce7' : '#fee2e2'}; color:${route.enabled ? '#166534' : '#991b1b'}; padding:4px 8px; border-radius:6px; font-size:12px;">
                        ${route.enabled ? 'مفعل' : 'معطل'}
                    </span>
                </td>
                <td style="padding:12px; display:flex; gap:5px;">
                    <button class="btn" onclick="toggleRoutingManual('${route.id}', ${route.enabled})" 
                        style="width:auto; padding:5px 10px; font-size:12px; background:${route.enabled ? '#64748b' : '#10b981'};">
                        ${route.enabled ? 'تعطيل' : 'تفعيل'}
                    </button>
                    <button class="btn btn-danger" onclick="removeRoutingManual('${route.app}')" 
                        style="width:auto; padding:5px 10px; font-size:12px;">حذف</button>
                </td>
            </tr>
        `).join('');

    } catch (e) {
        tbody.innerHTML = '<tr><td colspan="4" style="padding:20px; text-align:center; color:#ef4444;">خطأ في تحميل البيانات</td></tr>';
    }
}

async function toggleRoutingManual(id, currentEnabled) {
    const res = await fetch('/api/routing/toggle', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: id, disabled: !currentEnabled })
    });
    loadRoutingStatus();
}

async function removeRoutingManual(app) {
    if (!confirm(`هل أنت متأكد من حذف توجيه ${app}؟`)) return;
    const res = await fetch('/api/routing/remove', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target: app })
    });
    const result = await res.json();
    alert(result.message || result.error);
    loadAll();
}

async function applyRouting() {
    const payload = {
        target: document.getElementById('app-list').value,
        gateway: document.getElementById('gateway-list-routing').value
    };
    const res = await fetch('/api/routing/apply', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
    });
    const result = await res.json();
    alert(result.message || result.error);
}

async function removeRouting() {
    const payload = {
        target: document.getElementById('app-list').value
    };
    const res = await fetch('/api/routing/remove', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
    });
    const result = await res.json();
    alert(result.message || result.error);
}

async function purgeRouting() {
    if (!confirm('هل أنت متأكد من مسح جميع قواعد التوجيه الذكي؟')) return;
    const res = await fetch('/api/routing/purge', { method: 'DELETE' });
    const result = await res.json();
    alert(result.message || result.error);
}

async function applyBlock() {
    const payload = {
        name: document.getElementById('block-name').value,
        domains: document.getElementById('block-domains').value.split('\n').map(d => d.trim()).filter(d => d.length > 0)
    };
    const res = await fetch('/api/routing/block', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
    });
    const result = await res.json();
    alert(result.message || result.error);
}
