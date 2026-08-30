async function loadNAS() {
    refreshNASLiveStatus();
    try {
        const res = await apiFetch('/radius/api/nas');
        if (!res.ok) return;
        const nasList = await res.json();

        const tbody = document.getElementById('nas-tbody');

        if (nasList.length === 0) {
            tbody.innerHTML = '<tr><td colspan="7" style="text-align:center;">لا توجد أجهزة NAS متصلة...</td></tr>';
        } else {
            const hasCloudFixed = nasList.some(n => n.is_cloud_fixed);
            const addNasBtn = document.querySelector("button[onclick='openNASModal()']");
            if (addNasBtn) {
                addNasBtn.style.display = hasCloudFixed ? 'none' : '';
            }

            tbody.innerHTML = nasList.map(n => {
                let actionsHtml = '';
                if (n.is_cloud_fixed) {
                    actionsHtml = `
                        <div style="display:flex; gap:6px; flex-wrap:wrap; align-items:center;">
                            <button class="btn" style="padding:6px 12px; font-size:12px; background:linear-gradient(135deg, #10b981, #059669); color:white; border:none; font-weight:bold; box-shadow:0 2px 6px rgba(16,185,129,0.3);" onclick="openAutoRadSecModal()"><i class="fa-solid fa-bolt"></i> كود التثبيت السريع</button>
                            <button class="btn btn-primary" style="padding:6px 12px; font-size:12px;" onclick="window.open('/pki/cert/${n.subdomain || '1'}/agent.crt', '_blank')"><i class="fa-solid fa-download"></i> الشهادة</button>
                            <span class="badge" style="background:rgba(99, 102, 241, 0.15); color:#6366f1; border:1px solid rgba(99, 102, 241, 0.3); padding:5px 8px; border-radius:6px; font-size:11px;"><i class="fa-solid fa-lock"></i> مثبت سحابياً</span>
                        </div>
                    `;
                } else if (currentAdmin && currentAdmin.role === 'superadmin') {
                    actionsHtml = `
                        <div style="display:flex; gap:6px; flex-wrap:wrap;">
                            <button class="btn" style="padding:4px 8px; font-size:12px; background:var(--warning); border-color:var(--warning);" onclick="prepareEditNAS('${n.id}', '${escapeHtml(n.ip)}', '${escapeHtml(n.name)}', '${escapeHtml(n.secret)}', '${escapeHtml(n.profile_nas_ip)}', ${n.admin_id}, '${escapeHtml(n.admin_name)}')">تعديل</button>
                            <button class="btn btn-danger" style="padding:4px 8px; font-size:12px;" onclick="deleteNAS('${escapeHtml(n.ip)}')">حذف</button>
                            <button class="btn" style="padding:4px 8px; font-size:12px; background:var(--primary); color:#fff;" onclick="generateNASCert('${n.id}', '${escapeHtml(n.name || n.ip)}')"><i class="fa-solid fa-key"></i> شهادة</button>
                            ${n.common_name ? `
                                <button class="btn btn-success" style="padding:4px 8px; font-size:12px;" onclick="downloadNASCertBundle('${n.id}')"><i class="fa-solid fa-download"></i> الحزمة</button>
                                <button class="btn" style="padding:4px 8px; font-size:12px; background:#e53e3e; color:#fff;" onclick="revokeNASCert('${n.id}', '${escapeHtml(n.name || n.ip)}')"><i class="fa-solid fa-ban"></i> إبطال</button>
                            ` : ''}
                        </div>
                    `;
                } else {
                    actionsHtml = '<span style="color:var(--text-muted); font-size:12px;">غير مصرح</span>';
                }

                return `
                <tr>
                    <td><strong>${escapeHtml(n.ip)}</strong></td>
                    <td>${n.profile_nas_ip ? escapeHtml(n.profile_nas_ip) : '<span style="color:var(--text-muted);">تلقائي</span>'}</td>
                    <td>${escapeHtml(n.name || '-')}</td>
                    <td><span style="font-family:monospace; background:var(--bg-app); color:var(--text-main); padding:2px 6px; border-radius:4px;">${escapeHtml(n.secret)}</span></td>
                    <td>${getRadSecBadge(n)}</td>
                    ${(currentAdmin && currentAdmin.role === 'superadmin') ? `<td><span class="badge badge-secondary">${escapeHtml(n.admin_name || 'System')}</span></td>` : ''}
                    <td>${actionsHtml}</td>
                </tr>
            `}).join('');
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

async function openAutoRadSecModal() {
    const modal = document.getElementById('nas-auto-modal');
    if (!modal) return;
    modal.classList.add('active');

    try {
        const res = await apiFetch('/radius/api/nas/provision-code');
        if (res.ok) {
            const data = await res.json();
            document.getElementById('auto-provision-cmd').value = data.command;
            const badge = document.getElementById('auto-subdomain-badge');
            if (badge) badge.innerText = data.subdomain;
        } else {
            const hostParts = window.location.hostname.split('.');
            let inferredSub = (hostParts.length > 2 && hostParts[0] !== 'www') ? hostParts[0] : '';
            const sub = (currentAdmin && currentAdmin.subdomain) ? currentAdmin.subdomain : (inferredSub || 'default');
            const cmd = `/tool fetch url="https://sas-man.net/pki/install/${sub}.rsc" dst-path="sasman_cloud.rsc" mode=https; :delay 2s; /import sasman_cloud.rsc;`;
            document.getElementById('auto-provision-cmd').value = cmd;
            const badge = document.getElementById('auto-subdomain-badge');
            if (badge) badge.innerText = sub;
        }
    } catch (e) {
        console.error(e);
    }
}

function closeAutoRadSecModal() {
    const modal = document.getElementById('nas-auto-modal');
    if (modal) modal.classList.remove('active');
}

function copyAutoProvisionCode() {
    const cmdArea = document.getElementById('auto-provision-cmd');
    if (!cmdArea) return;
    cmdArea.select();
    navigator.clipboard.writeText(cmdArea.value).then(() => {
        alert('✅ تم نسخ كود التثبيت التلقائي إلى الحافظة بنجاح!\nالصقه الآن في Terminal المايكروتك.');
    }).catch(() => {
        document.execCommand('copy');
        alert('✅ تم نسخ كود التثبيت التلقائي بنجاح!');
    });
}

// Attach all NAS functions directly to window object for instant availability
window.openAutoRadSecModal = openAutoRadSecModal;
window.closeAutoRadSecModal = closeAutoRadSecModal;
window.copyAutoProvisionCode = copyAutoProvisionCode;
window.openNASGuideModal = openNASGuideModal;
window.closeNASGuideModal = closeNASGuideModal;
window.switchGuideTab = switchGuideTab;
window.copyCodeText = copyCodeText;
window.loadNAS = loadNAS;
window.openNASModal = openNASModal;
window.closeNASModal = closeNASModal;
window.quickSetupNAS = quickSetupNAS;
window.generateNASCert = generateNASCert;
window.downloadNASCertBundle = downloadNASCertBundle;
window.revokeNASCert = revokeNASCert;

async function refreshNASLiveStatus() {
    const pingEl = document.getElementById('nas-ping-val');
    const titleEl = document.getElementById('nas-status-title');
    const badgeEl = document.getElementById('nas-status-badge');
    const descEl = document.getElementById('nas-status-desc');
    const modeBadge = document.getElementById('nas-mode-badge');
    const indicator = document.getElementById('nas-status-indicator');

    if (pingEl) pingEl.textContent = '...';

    try {
        const res = await apiFetch('/radius/api/nas/status');
        if (!res.ok) throw new Error('فشل جلب الحالة');
        const data = await res.json();

        if (pingEl) {
            pingEl.textContent = `${data.latency_ms} ms`;
            if (data.latency_ms <= 5) {
                pingEl.style.color = '#10b981'; // Green (< 5ms local)
            } else if (data.latency_ms <= 40) {
                pingEl.style.color = '#0284c7'; // Blue (< 40ms cloud)
            } else {
                pingEl.style.color = '#f59e0b'; // Amber
            }
        }

        if (titleEl) titleEl.textContent = data.status_text || (data.connected ? '🟢 راوتر المايكروتك متصل الآن' : '🔴 الراوتر غير متصل');
        if (descEl) descEl.textContent = `البروتوكول: ${data.protocol} | المعرّف: ${data.common_name || data.router_ip || '-'}`;

        if (data.winbox_address) {
            let winboxBox = document.getElementById('nas-winbox-info');
            if (!winboxBox) {
                const headerCard = descEl ? descEl.closest('.card') || descEl.parentNode : null;
                if (headerCard) {
                    winboxBox = document.createElement('div');
                    winboxBox.id = 'nas-winbox-info';
                    winboxBox.style = 'margin-top:12px; padding:10px 14px; background:rgba(30,41,59,0.7); border:1px solid rgba(255,255,255,0.1); border-radius:8px; display:flex; align-items:center; justify-content:space-between; flex-wrap:wrap; gap:10px;';
                    headerCard.appendChild(winboxBox);
                }
            }
            if (winboxBox) {
                winboxBox.innerHTML = `
                    <div style="display:flex; align-items:center; gap:8px;">
                        <i class="fa-solid fa-desktop" style="color:#38bdf8; font-size:16px;"></i>
                        <span style="font-size:13px; color:#cbd5e1;">منفذ Winbox السحابي المباشر:</span>
                        <strong style="font-family:monospace; color:#38bdf8; font-size:14px; background:#0f172a; padding:2px 8px; border-radius:4px; border:1px solid #334155; direction:ltr;">${data.winbox_address}</strong>
                    </div>
                    <button class="btn btn-sm" onclick="navigator.clipboard.writeText('${data.winbox_address}'); alert('تم نسخ عنوان Winbox: ${data.winbox_address}');" style="background:#0284c7; color:#fff; padding:4px 12px; font-size:12px; border-radius:6px; border:none; font-weight:600; cursor:pointer;">
                        <i class="fa-solid fa-copy"></i> نسخ للـ Winbox
                    </button>
                `;
            }
        }

        if (modeBadge) {
            modeBadge.textContent = data.mode === 'cloud' ? 'RadSec TLS السحابي' : 'محلي (Loopback)';
            modeBadge.style.background = data.mode === 'cloud' ? 'rgba(14,165,233,0.15)' : 'rgba(16,185,129,0.15)';
            modeBadge.style.color = data.mode === 'cloud' ? '#0284c7' : '#10b981';
        }

        if (badgeEl) {
            badgeEl.style.background = data.connected ? '#10b981' : '#ef4444';
            badgeEl.style.boxShadow = data.connected ? '0 0 10px #10b981' : '0 0 10px #ef4444';
        }

        if (indicator) {
            indicator.style.color = data.connected ? '#10b981' : '#ef4444';
            indicator.style.background = data.connected ? 'rgba(16,185,129,0.15)' : 'rgba(239,68,68,0.15)';
        }
    } catch (err) {
        if (pingEl) pingEl.textContent = '-';
        if (titleEl) titleEl.textContent = 'تعذر فحص حالة الراوتر';
    }
}
window.refreshNASLiveStatus = refreshNASLiveStatus;
