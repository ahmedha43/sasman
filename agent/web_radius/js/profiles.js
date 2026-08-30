let radiusProfilesCache = [];

function toggleProfileQuotaType() {
    const quotaType = document.querySelector('input[name="prof_quota_type"]:checked')?.value || 'unlimited';
    const valWrapper = document.getElementById('prof-quota-val-wrapper');
    const badge = document.getElementById('prof-quota-unit-badge');
    const input = document.getElementById('prof-quota-val');

    if (quotaType === 'gb') {
        if (valWrapper) valWrapper.style.display = 'block';
        if (badge) badge.innerText = 'GB';
        if (input && !input.value) input.value = '5';
    } else if (quotaType === 'mb') {
        if (valWrapper) valWrapper.style.display = 'block';
        if (badge) badge.innerText = 'MB';
        if (input && !input.value) input.value = '500';
    } else {
        if (valWrapper) valWrapper.style.display = 'none';
        if (input) input.value = '';
    }
}

function toggleProfileLinkType() {
    const linkType = document.querySelector('input[name="prof_link_type"]:checked')?.value || 'none';
    const poolWrapper = document.getElementById('prof-mikrotik-pool-wrapper');
    const groupWrapper = document.getElementById('prof-mikrotik-group-wrapper');
    const poolInput = document.getElementById('prof-pool');
    const groupInput = document.getElementById('prof-mikrotik-group');

    if (linkType === 'pool') {
        poolWrapper.style.display = 'block';
        groupWrapper.style.display = 'none';
        groupInput.value = ''; // clear the other
    } else if (linkType === 'group') {
        poolWrapper.style.display = 'none';
        groupWrapper.style.display = 'block';
        poolInput.value = ''; // clear the other
    } else {
        poolWrapper.style.display = 'none';
        groupWrapper.style.display = 'none';
        poolInput.value = '';
        groupInput.value = '';
    }
}

function toggleProfileExpiredLinkType() {
    const linkType = document.querySelector('input[name="prof_expired_link_type"]:checked')?.value || 'none';
    const poolWrapper = document.getElementById('prof-expired-pool-wrapper');
    const groupWrapper = document.getElementById('prof-expired-profile-wrapper');
    const poolInput = document.getElementById('prof-expired-pool');
    const groupInput = document.getElementById('prof-expired-profile');

    if (linkType === 'pool') {
        if (poolWrapper) poolWrapper.style.display = 'block';
        if (groupWrapper) groupWrapper.style.display = 'none';
        if (groupInput) groupInput.value = ''; // clear the other
    } else if (linkType === 'group') {
        if (poolWrapper) poolWrapper.style.display = 'none';
        if (groupWrapper) groupWrapper.style.display = 'block';
        if (poolInput) poolInput.value = ''; // clear the other
    } else {
        if (poolWrapper) poolWrapper.style.display = 'none';
        if (groupWrapper) groupWrapper.style.display = 'none';
        if (poolInput) poolInput.value = '';
        if (groupInput) groupInput.value = '';
    }
}

async function loadProfiles() {
    try {
        const res = await apiFetch('/radius/api/profiles');
        if (!res.ok) return;
        const profiles = await res.json();
        radiusProfilesCache = Array.isArray(profiles) ? profiles : [];

        const tbody = document.getElementById('profiles-tbody');
        const userProfileSelect = document.getElementById('usr-profile');
        const selectedUserProfile = userProfileSelect ? userProfileSelect.value : null;

        if (profiles.length === 0) {
            if (tbody) tbody.innerHTML = '<tr><td colspan="9" style="text-align:center;">لا توجد باقات...</td></tr>';
            if (userProfileSelect) userProfileSelect.innerHTML = '<option value="">لا توجد باقات</option>';
        } else {
            if (tbody) {
                tbody.innerHTML = profiles.map(p => {
                    let poolDisplay = '<span style="color:var(--text-muted);">بدون ارتباط</span>';
                    if (p.pool) {
                        poolDisplay = `<span class="badge" style="background:rgba(14, 165, 233, 0.1); color:#0284c7;">🔵 Pool: ${p.pool}</span>`;
                    } else if (p.mikrotik_group) {
                        poolDisplay = `<span class="badge" style="background:rgba(219, 39, 119, 0.1); color:#db2777;">🟣 Group: ${p.mikrotik_group}</span>`;
                    }

                    let expiredDisplay = '<span style="color:var(--text-muted);">بدون تحويل</span>';
                    if (p.expired_pool) {
                        expiredDisplay = `<span class="badge badge-danger" style="font-size: 0.85em;">🔴 Pool: ${p.expired_pool}</span>`;
                    } else if (p.expired_profile) {
                        expiredDisplay = `<span class="badge badge-danger" style="font-size: 0.85em;">🔴 Profile: ${p.expired_profile}</span>`;
                    }

                    let quotaDisplay = '<span class="badge badge-success" style="font-weight:600;">🟢 مفتوح (Unlimited)</span>';
                    if (p.quota_limit_mb && p.quota_limit_mb > 0) {
                        if (p.quota_limit_mb >= 1024) {
                            const gb = (p.quota_limit_mb / 1024).toFixed(p.quota_limit_mb % 1024 === 0 ? 0 : 1);
                            quotaDisplay = `<span class="badge" style="background:rgba(2,132,199,0.15); color:#0284c7; font-weight:700;">📊 ${gb} GB</span>`;
                        } else {
                            quotaDisplay = `<span class="badge" style="background:rgba(147,51,234,0.15); color:#9333ea; font-weight:700;">📊 ${p.quota_limit_mb} MB</span>`;
                        }
                    }

                    return `
                    <tr>
                        <td><strong>${p.name}</strong></td>
                        <td><span class="badge badge-success">${p.limit}</span></td>
                        <td>${quotaDisplay}</td>
                        <td>${poolDisplay}</td>
                        <td>${p.validity_days ? p.validity_days + ' يوم' : 'مفتوح'}</td>
                        <td><span class="badge badge-warning" style="font-weight:600;">${p.price > 0 ? p.price.toLocaleString() + ' د.ع' : '—'}</span></td>
                        ${(currentAdmin && currentAdmin.role === 'superadmin') ? `<td><span class="badge badge-success" style="font-weight:600;">${p.agent_price > 0 ? p.agent_price.toLocaleString() + ' د.ع' : '—'}</span></td>` : ''}
                        <td><span class="badge" style="background:rgba(79, 70, 229, 0.1); color:#4f46e5;">${p.nas_ip}</span></td>
                        <td><span class="badge badge-secondary">${p.simultaneous === "0" || !p.simultaneous ? 'لا محدود' : p.simultaneous + ' جهاز'}</span></td>
                        <td>${expiredDisplay}</td>
                        ${(currentAdmin && currentAdmin.role === 'superadmin') ? `<td><span class="badge badge-secondary">${escapeHtml(p.admin_name || 'System')}</span></td>` : ''}
                        <td>
                            <button class="btn btn-primary" style="padding:6px 12px; width:auto; border-radius:4px;" onclick="editProfile('${p.name}')">تعديل</button>
                            <button class="btn btn-danger" style="padding:6px 12px; width:auto; border-radius:4px;" onclick="deleteProfile('${p.name}')">حذف</button>
                        </td>
                    </tr>
                `}).join('');
            }

            if (userProfileSelect) {
                userProfileSelect.innerHTML = profiles.map(p => `<option value="${p.name}">${p.name}</option>`).join('');
                if (selectedUserProfile && profiles.some(p => p.name === selectedUserProfile)) {
                    userProfileSelect.value = selectedUserProfile;
                }
            }
        }
        updateDashboard();
    } catch (e) {
        console.error(e);
    }
}

async function createProfile() {
    const getVal = (id) => {
        const el = document.getElementById(id);
        if (!el) {
            console.error(`Element with ID "${id}" not found!`);
            return "";
        }
        return el.value || "";
    };

    // Calculate quota limit in MB
    let quotaLimitMB = 0;
    const quotaType = document.querySelector('input[name="prof_quota_type"]:checked')?.value || 'unlimited';
    const quotaVal = parseFloat(getVal('prof-quota-val')) || 0;
    if (quotaType === 'gb' && quotaVal > 0) {
        quotaLimitMB = Math.round(quotaVal * 1024);
    } else if (quotaType === 'mb' && quotaVal > 0) {
        quotaLimitMB = Math.round(quotaVal);
    }

    const payload = {
        original_name: getVal('prof-original-name').trim(),
        name: getVal('prof-name').trim(),
        download: getVal('prof-dl').trim(),
        upload: getVal('prof-ul').trim(),
        pool: document.getElementById('prof-pool')?.value.trim() || '',
        mikrotik_group: document.getElementById('prof-mikrotik-group')?.value.trim() || '',
        validity: getVal('prof-val').trim(),
        nas_ip: getVal('prof-nas'),
        price: parseFloat(getVal('prof-price')) || 0,
        agent_price: parseFloat(getVal('prof-agent-price')) || 0,
        simultaneous: getVal('prof-simultaneous') || "1",
        quota_limit_mb: quotaLimitMB,
        expired_pool: document.getElementById('prof-expired-pool')?.value.trim() || '',
        expired_profile: document.getElementById('prof-expired-profile')?.value.trim() || '',
        admin_id: parseInt(getVal('prof-owner') || "0")
    };

    if (!payload.name) return alert("يرجى إدخال اسم الباقة");

    try {
        const res = await apiFetch('/radius/api/profiles', {
            method: 'POST',
            body: JSON.stringify(payload)
        });

        const result = await res.json();
        if (res.ok) {
            alert("✅ تم حفظ الباقة بنجاح");
            resetProfileForm();
            loadProfiles();
            closeProfileModal();
        } else {
            alert("❌ فشل الحفظ: " + (result.error || result.message));
        }
    } catch (e) {
        console.error("Create profile failed", e);
        alert("❌ حدث خطأ أثناء الاتصال بالخادم");
    }
}

function editProfile(name) {
    const p = radiusProfilesCache.find(x => x.name === name);
    if (!p) return;

    document.getElementById('prof-original-name').value = name;
    document.getElementById('prof-name').value = p.name;
    // Speed parsing from "1M/10M"
    if (p.limit && p.limit.includes('/')) {
        const parts = p.limit.split('/');
        document.getElementById('prof-ul').value = parts[0].replace('M', '');
        document.getElementById('prof-dl').value = parts[1].replace('M', '');
    } else {
        document.getElementById('prof-ul').value = "";
        document.getElementById('prof-dl').value = "";
    }

    document.getElementById('prof-pool').value = p.pool || "";
    document.getElementById('prof-mikrotik-group').value = p.mikrotik_group || "";
    
    // Set radio buttons
    if (p.pool) {
        document.querySelector('input[name="prof_link_type"][value="pool"]').checked = true;
    } else if (p.mikrotik_group) {
        document.querySelector('input[name="prof_link_type"][value="group"]').checked = true;
    } else {
        document.querySelector('input[name="prof_link_type"][value="none"]').checked = true;
    }
    toggleProfileLinkType();

    document.getElementById('prof-val').value = p.validity_days || "30";
    document.getElementById('prof-price').value = p.price || 0;
    if (document.getElementById('prof-agent-price')) document.getElementById('prof-agent-price').value = p.agent_price || 0;
    document.getElementById('prof-nas').value = p.nas_ip || "ALL";
    document.getElementById('prof-simultaneous').value = p.simultaneous || "1";
    document.getElementById('prof-expired-pool').value = p.expired_pool || "";
    document.getElementById('prof-expired-profile').value = p.expired_profile || "";

    // Set expired radio buttons
    if (p.expired_pool) {
        document.querySelector('input[name="prof_expired_link_type"][value="pool"]').checked = true;
    } else if (p.expired_profile) {
        document.querySelector('input[name="prof_expired_link_type"][value="group"]').checked = true;
    } else {
        document.querySelector('input[name="prof_expired_link_type"][value="none"]').checked = true;
    }
    toggleProfileExpiredLinkType();

    // Set Quota radio & input
    if (p.quota_limit_mb && p.quota_limit_mb > 0) {
        if (p.quota_limit_mb >= 1024 && p.quota_limit_mb % 1024 === 0) {
            document.querySelector('input[name="prof_quota_type"][value="gb"]').checked = true;
            document.getElementById('prof-quota-val').value = p.quota_limit_mb / 1024;
        } else {
            document.querySelector('input[name="prof_quota_type"][value="mb"]').checked = true;
            document.getElementById('prof-quota-val').value = p.quota_limit_mb;
        }
    } else {
        document.querySelector('input[name="prof_quota_type"][value="unlimited"]').checked = true;
        document.getElementById('prof-quota-val').value = '';
    }
    toggleProfileQuotaType();

    if (document.getElementById('prof-owner')) document.getElementById('prof-owner').value = p.admin_id || "0";

    document.getElementById('prof-modal-title').innerText = 'تعديل باقة اشتراك: ' + p.name;
    document.getElementById('profile-modal').classList.add('active');
}

function resetProfileForm() {
    document.getElementById('prof-original-name').value = "";
    document.getElementById('prof-name').value = "";
    document.getElementById('prof-dl').value = "";
    document.getElementById('prof-ul').value = "";
    document.getElementById('prof-pool').value = "";
    document.getElementById('prof-mikrotik-group').value = "";
    document.querySelector('input[name="prof_link_type"][value="none"]').checked = true;
    toggleProfileLinkType();
    document.querySelector('input[name="prof_quota_type"][value="unlimited"]').checked = true;
    toggleProfileQuotaType();
    document.getElementById('prof-val').value = "30";
    document.getElementById('prof-price').value = "";
    if (document.getElementById('prof-agent-price')) document.getElementById('prof-agent-price').value = "";
    document.getElementById('prof-nas').value = "ALL";
    document.getElementById('prof-simultaneous').value = "1";
    document.getElementById('prof-expired-pool').value = "";
    document.getElementById('prof-expired-profile').value = "";
    document.querySelector('input[name="prof_expired_link_type"][value="none"]').checked = true;
    toggleProfileExpiredLinkType();
}

async function deleteProfile(name) {
    if (!confirm(`حذف باقة ${name}؟`)) return;
    const res = await apiFetch(`/radius/api/profiles/${encodeURIComponent(name)}`, { method: 'DELETE' });
    const result = await res.json();
    alert(result.message || result.error);
    loadProfiles();
}

function openProfileModal() {
    resetProfileForm();
    document.getElementById('prof-modal-title').innerText = 'إضافة باقة اشتراك جديدة';
    document.getElementById('profile-modal').classList.add('active');
}

function closeProfileModal() {
    document.getElementById('profile-modal').classList.remove('active');
}
