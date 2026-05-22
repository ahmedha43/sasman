async function loadNAS() {
    try {
        const res = await apiFetch('/radius/api/nas');
        if (!res.ok) return;
        const nasList = await res.json();

        const tbody = document.getElementById('nas-tbody');

        if (nasList.length === 0) {
            tbody.innerHTML = '<tr><td colspan="5" style="text-align:center;">لا توجد أجهزة NAS متصلة...</td></tr>';
        } else {
            tbody.innerHTML = nasList.map(n => `
                <tr>
                    <td><strong>${n.ip}</strong></td>
                    <td>${n.profile_nas_ip || '<span style="color:var(--text-muted);">غير مضبوط</span>'}</td>
                    <td>${n.name || '-'}</td>
                    <td><span style="font-family:monospace; background:var(--bg-app); color:var(--text-main); padding:2px 6px; border-radius:4px;">${n.secret}</span></td>
                    ${(currentAdmin && currentAdmin.role === 'superadmin') ? `<td><span class="badge badge-secondary">${escapeHtml(n.admin_name || 'System')}</span></td>` : ''}
                    <td>
                        ${(currentAdmin && currentAdmin.role === 'superadmin') ? `
                            <button class="btn" style="padding:6px 12px; width:auto; background:var(--warning); border-color:var(--warning);" onclick="prepareEditNAS('${n.id}', '${n.ip}', '${n.name}', '${n.secret}', '${n.profile_nas_ip}', ${n.admin_id}, '${n.admin_name}')">تعديل</button>
                            <button class="btn btn-danger" style="padding:6px 12px; width:auto;" onclick="deleteNAS('${n.ip}')">حذف</button>
                        ` : '<span style="color:var(--text-muted); font-size:12px;">غير مصرح</span>'}
                    </td>
                </tr>
            `).join('');
        }

        const nasSelect = document.getElementById('prof-nas');
        if (nasSelect) {
            const profileTargets = nasList.filter(n => n.profile_nas_ip);
            nasSelect.innerHTML = '<option value="ALL">جميع الراوترات المتصلة</option>' +
                (profileTargets.length ? profileTargets.map(n => `<option value="${n.profile_nas_ip}">${n.name || n.ip} (${n.profile_nas_ip})</option>`).join('') : '');
        }

    } catch (e) {
        console.error(e);
    }
}

let currentEditNASId = null;

async function prepareEditNAS(id, ip, name, secret, profileIP, adminId, adminName) {
    document.getElementById('nas-ip').value = ip;
    document.getElementById('nas-name').value = name || '';
    document.getElementById('nas-secret').value = secret;
    document.getElementById('nas-profile-ip').value = profileIP || '';
    
    if (document.getElementById('nas-owner')) {
        document.getElementById('nas-owner').value = adminId;
    }
    
    if (document.getElementById('nas-is-global')) {
        document.getElementById('nas-is-global').checked = (adminId === 0 || adminName === 'Global');
    }
    
    currentEditNASId = id;
    const btn = document.getElementById('nas-submit-btn');
    if (btn) {
        btn.innerText = 'تحديث الراوتر';
        btn.onclick = updateNAS;
        btn.style.background = 'var(--warning)';
        btn.style.borderColor = 'var(--warning)';
    }
    
    document.getElementById('nas-modal-title').innerText = 'تعديل راوتر (NAS): ' + ip;
    document.getElementById('nas-modal').classList.add('active');
}

function cancelEditNAS() {
    currentEditNASId = null;
    document.getElementById('nas-ip').value = '';
    document.getElementById('nas-name').value = '';
    document.getElementById('nas-secret').value = '';
    document.getElementById('nas-profile-ip').value = '';
    if (document.getElementById('nas-is-global')) document.getElementById('nas-is-global').checked = false;
    
    const btn = document.getElementById('nas-submit-btn');
    if (btn) {
        btn.innerText = 'إضافة الراوتر';
        btn.onclick = createNAS;
        btn.style.background = '';
        btn.style.borderColor = '';
    }
    
    closeNASModal();
}

async function updateNAS() {
    if (!currentEditNASId) return;
    
    const payload = {
        ip: document.getElementById('nas-ip').value,
        name: document.getElementById('nas-name').value,
        secret: document.getElementById('nas-secret').value,
        profile_nas_ip: document.getElementById('nas-profile-ip').value,
        admin_id: parseInt(document.getElementById('nas-owner')?.value || "0"),
        is_global: document.getElementById('nas-is-global')?.checked || false
    };

    if (!payload.ip || !payload.secret) return alert("يرجى تعبئة عنوان العميل والسر المشترك");

    const res = await apiFetch(`/radius/api/nas/${currentEditNASId}`, {
        method: 'PUT',
        body: JSON.stringify(payload)
    });

    const result = await res.json();
    alert(result.message || result.error);

    cancelEditNAS();
    loadNAS();
}

async function createNAS() {
    const payload = {
        ip: document.getElementById('nas-ip').value,
        name: document.getElementById('nas-name').value,
        secret: document.getElementById('nas-secret').value,
        profile_nas_ip: document.getElementById('nas-profile-ip').value,
        admin_id: parseInt(document.getElementById('nas-owner')?.value || "0"),
        is_global: document.getElementById('nas-is-global')?.checked || false
    };

    if (!payload.ip || !payload.secret) return alert("يرجى تعبئة عنوان العميل والسر المشترك");

    const res = await apiFetch('/radius/api/nas', {
        method: 'POST',
        body: JSON.stringify(payload)
    });

    const result = await res.json();
    alert(result.message || result.error);

    if (res.ok) {
        document.getElementById('nas-ip').value = '';
        document.getElementById('nas-name').value = '';
        document.getElementById('nas-secret').value = '';
        document.getElementById('nas-profile-ip').value = '';
        closeNASModal();
        loadNAS();
    }
}

async function deleteNAS(ip) {
    if (!confirm(`حذف الراوتر ${ip}؟`)) return;
    const res = await apiFetch(`/radius/api/nas/${encodeURIComponent(ip)}`, { method: 'DELETE' });
    const result = await res.json();
    alert(result.message || result.error);
    loadNAS();
}

function openNASModal() {
    cancelEditNAS();
    document.getElementById('nas-modal-title').innerText = 'ربط راوتر مايكروتك جديد (NAS)';
    document.getElementById('nas-modal').classList.add('active');
}

function closeNASModal() {
    document.getElementById('nas-modal').classList.remove('active');
}
