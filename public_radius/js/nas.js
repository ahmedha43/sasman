async function loadNAS() {
    try {
        const res = await apiFetch('/radius/api/nas');
        if (!res.ok) return;
        const nasList = await res.json();

        const tbody = document.getElementById('nas-tbody');

        if (nasList.length === 0) {
            tbody.innerHTML = '<tr><td colspan="7" style="text-align:center;">لا توجد أجهزة NAS متصلة...</td></tr>';
        } else {
            tbody.innerHTML = nasList.map(n => `
                <tr>
                    <td><strong>${escapeHtml(n.ip)}</strong></td>
                    <td>${n.profile_nas_ip ? escapeHtml(n.profile_nas_ip) : '<span style="color:var(--text-muted);">غير مضبوط</span>'}</td>
                    <td>${escapeHtml(n.name || '-')}</td>
                    <td><span style="font-family:monospace; background:var(--bg-app); color:var(--text-main); padding:2px 6px; border-radius:4px;">${escapeHtml(n.secret)}</span></td>
                    <td>${getRadSecBadge(n)}</td>
                    ${(currentAdmin && currentAdmin.role === 'superadmin') ? `<td><span class="badge badge-secondary">${escapeHtml(n.admin_name || 'System')}</span></td>` : ''}
                    <td>
                        ${(currentAdmin && currentAdmin.role === 'superadmin') ? `
                            <div style="display:flex; gap:6px; flex-wrap:wrap;">
                                <button class="btn" style="padding:4px 8px; font-size:12px; background:var(--warning); border-color:var(--warning);" onclick="prepareEditNAS('${n.id}', '${escapeHtml(n.ip)}', '${escapeHtml(n.name)}', '${escapeHtml(n.secret)}', '${escapeHtml(n.profile_nas_ip)}', ${n.admin_id}, '${escapeHtml(n.admin_name)}')">تعديل</button>
                                <button class="btn btn-danger" style="padding:4px 8px; font-size:12px;" onclick="deleteNAS('${escapeHtml(n.ip)}')">حذف</button>
                                <button class="btn" style="padding:4px 8px; font-size:12px; background:var(--primary); color:#fff;" onclick="generateNASCert('${n.id}', '${escapeHtml(n.name || n.ip)}')"><i class="fa-solid fa-key"></i> شهادة</button>
                                ${n.common_name ? `
                                    <button class="btn btn-success" style="padding:4px 8px; font-size:12px;" onclick="downloadNASCertBundle('${n.id}')"><i class="fa-solid fa-download"></i> الحزمة</button>
                                    <button class="btn" style="padding:4px 8px; font-size:12px; background:#e53e3e; color:#fff;" onclick="revokeNASCert('${n.id}', '${escapeHtml(n.name || n.ip)}')"><i class="fa-solid fa-ban"></i> إبطال</button>
                                ` : ''}
                            </div>
                        ` : '<span style="color:var(--text-muted); font-size:12px;">غير مصرح</span>'}
                    </td>
                </tr>
            `).join('');
        }

        const nasSelect = document.getElementById('prof-nas');
        if (nasSelect) {
            const profileTargets = nasList.filter(n => n.profile_nas_ip);
            nasSelect.innerHTML = '<option value="ALL">جميع الراوترات المتصلة</option>' +
                (profileTargets.length ? profileTargets.map(n => `<option value="${n.profile_nas_ip}">${escapeHtml(n.name || n.ip)} (${n.profile_nas_ip})</option>`).join('') : '');
        }

    } catch (e) {
        console.error(e);
    }
}

function getRadSecBadge(n) {
    if (n.radsec_status === 'online') {
        return '<span class="badge" style="background:#2f855a; color:#fff; padding:4px 8px; border-radius:6px;"><i class="fa-solid fa-shield-halved"></i> متصل RadSec 🟢</span>';
    } else if (n.radsec_status === 'configured') {
        return '<span class="badge" style="background:#4a5568; color:#e2e8f0; padding:4px 8px; border-radius:6px;"><i class="fa-solid fa-key"></i> شهادة صادرة ⚪</span>';
    }
    return '<span class="badge" style="background:rgba(66, 153, 225, 0.2); color:#63b3ed; padding:4px 8px; border-radius:6px;">UDP مباشر 🔵</span>';
}

async function generateNASCert(id, name) {
    if (!confirm(`هل تريد توليد شهادة RadSec mTLS مشفرة للراوتر "${name}"؟`)) return;
    try {
        const res = await apiFetch(`/radius/api/nas/${id}/generate-cert`, { method: 'POST' });
        const data = await res.json();
        if (res.ok) {
            alert(data.message || 'تم توليد الشهادة بنجاح');
            loadNAS();
        } else {
            alert(data.error || 'فشل توليد الشهادة');
        }
    } catch (e) {
        alert('خطأ في الاتصال: ' + e);
    }
}

function downloadNASCertBundle(id) {
    window.open(`/radius/api/nas/${id}/cert-bundle`, '_blank');
}

async function revokeNASCert(id, name) {
    if (!confirm(`تحذير أمني: هل أنت متأكد من إبطال شهادة RadSec للراوتر "${name}"؟ سيتم فصل اتصال الوكيل فوراً ومنعه من الاتصال.`)) return;
    try {
        const res = await apiFetch(`/radius/api/nas/${id}/revoke-cert`, { method: 'POST' });
        const data = await res.json();
        if (res.ok) {
            alert(data.message || 'تم إبطال الشهادة بنجاح');
            loadNAS();
        } else {
            alert(data.error || 'فشل إبطال الشهادة');
        }
    } catch (e) {
        alert('خطأ في الاتصال: ' + e);
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
    document.getElementById('nas-ip').value = '';
    document.getElementById('nas-name').value = '';
    document.getElementById('nas-secret').value = '';
    document.getElementById('nas-profile-ip').value = '';
    currentEditNASId = null;
    const btn = document.getElementById('nas-submit-btn');
    if (btn) {
        btn.innerText = 'إضافة الراوتر';
        btn.onclick = createNAS;
        btn.style.background = '';
        btn.style.borderColor = '';
    }
    document.getElementById('nas-modal-title').innerText = 'ربط راوتر مايكروتك جديد (NAS)';
    document.getElementById('nas-modal').classList.add('active');
}

function closeNASModal() {
    document.getElementById('nas-modal').classList.remove('active');
}

async function quickSetupNAS() {
    if (!confirm("هل أنت متأكد من تنفيذ الإعداد السريع التلقائي لراديوس المايكروتك (172.17.0.1)؟")) return;
    
    try {
        const res = await apiFetch('/radius/api/nas/quick-setup', { method: 'POST' });
        const data = await res.json();
        if (data.error) {
            alert("خطأ: " + data.error);
        } else {
            alert(data.message || "تم الإعداد بنجاح!");
            loadNAS();
        }
    } catch(e) {
        alert("حدث خطأ أثناء الاتصال بالخادم: " + e);
    }
}

function openNASGuideModal() {
    const modal = document.getElementById('nas-guide-modal');
    if (modal) {
        modal.classList.add('active');
        switchGuideTab('overview');
    }
}

function closeNASGuideModal() {
    const modal = document.getElementById('nas-guide-modal');
    if (modal) {
        modal.classList.remove('active');
    }
}

function switchGuideTab(tabId) {
    document.querySelectorAll('.guide-tab-btn').forEach(btn => {
        btn.style.background = 'transparent';
        btn.style.color = 'var(--text-muted)';
        btn.style.borderBottom = '2px solid transparent';
    });
    document.querySelectorAll('.guide-tab-pane').forEach(pane => pane.style.display = 'none');
    
    const activeBtn = document.getElementById(`btn-guide-${tabId}`);
    if (activeBtn) {
        activeBtn.style.background = 'rgba(66, 153, 225, 0.1)';
        activeBtn.style.color = 'var(--primary)';
        activeBtn.style.borderBottom = '2px solid var(--primary)';
    }
    
    const activePane = document.getElementById(`pane-guide-${tabId}`);
    if (activePane) activePane.style.display = 'block';
}

function copyCodeText(elementId) {
    const el = document.getElementById(elementId);
    if (!el) return;
    const text = el.innerText || el.textContent;
    navigator.clipboard.writeText(text).then(() => {
        alert('تم نسخ الأمر إلى الحافظة بنجاح 📋');
    }).catch(err => {
        alert('تعذر النسخ التلقائي: ' + err);
    });
}
