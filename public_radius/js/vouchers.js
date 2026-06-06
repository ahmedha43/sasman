let voucherGroupsCache = {};

async function loadVouchers() {
    try {
        const res = await apiFetch('/radius/api/vouchers');
        if (!res.ok) return;
        const vouchers = await res.json();
        renderVouchers(vouchers);
        
        const role = localStorage.getItem('sasman_role');
        const clearBtn = document.getElementById('clear-vouchers-btn');
        if (clearBtn) {
            clearBtn.style.display = role === 'superadmin' ? 'inline-block' : 'none';
        }
    } catch (e) {
        console.error('Failed to load vouchers', e);
    }
}

function renderVouchers(vouchers) {
    const list = document.getElementById('vouchers-list');
    if (!list) return;
    voucherGroupsCache = {};

    if (!Array.isArray(vouchers) || vouchers.length === 0) {
        list.innerHTML = '<tr><td colspan="8" style="text-align:center; padding:20px;">لا توجد كروت حاليا</td></tr>';
        return;
    }

    list.innerHTML = groupVouchers(vouchers).map(group => renderVoucherGroup(group)).join('');
}

function groupVouchers(vouchers) {
    const groups = [];
    const byKey = {};

    vouchers.forEach(v => {
        const createdAt = v.created_at || '';
        const key = v.batch_id || `legacy-${v.profile_name}-${v.created_by}-${createdAt}`;
        if (!byKey[key]) {
            byKey[key] = {
                key,
                profile_name: v.profile_name,
                validity_days: Number(v.validity_days || 0),
                price: Number(v.price || 0),
                created_at: createdAt,
                vouchers: []
            };
            groups.push(byKey[key]);
        }
        byKey[key].vouchers.push(v);
    });

    return groups;
}

function renderVoucherGroup(group) {
    voucherGroupsCache[group.key] = group;
    const total = group.vouchers.length;
    const used = group.vouchers.filter(v => Number(v.is_used) === 1).length;
    const available = total - used;
    const created = group.created_at ? new Date(group.created_at).toLocaleString('ar-IQ') : '-';

    return `
        <tr style="background:rgba(79, 70, 229, 0.05);">
            <td colspan="8" style="padding:12px; border-bottom: 2px solid var(--border);">
                <div style="display:flex; justify-content:space-between; align-items:center; gap:12px; flex-wrap:wrap;">
                    <div>
                        <strong style="color:var(--text-main);">مجموعة كروت: ${escapeVoucherText(group.profile_name)}</strong>
                        <span style="color:var(--text-muted); margin-inline-start:8px; font-size:13px;">${total} كرت | المتاح ${available} | المستخدم ${used} | ${created}</span>
                    </div>
                    <div>
                        <button class="btn btn-danger" style="width:auto; padding:6px 14px; font-size:12px; background:var(--danger); border-color:var(--danger); margin-inline-end: 8px;" onclick="deleteVoucherBatch('${escapeVoucherAttr(group.key)}')">حذف المجموعة</button>
                        <button class="btn btn-primary" style="width:auto; padding:6px 14px; font-size:12px;" onclick="printVoucherGroup('${escapeVoucherAttr(group.key)}')">طباعة المجموعة</button>
                    </div>
                </div>
            </td>
        </tr>
        ${group.vouchers.map(v => renderVoucherRow(v)).join('')}
    `;
}

function renderVoucherRow(v) {
    const used = Number(v.is_used) === 1;
    return `
        <tr class="${used ? 'used-row' : ''}">
            <td><code style="background:var(--bg-app); color:var(--text-main); padding:2px 6px; border-radius:4px; font-weight:bold; font-family:monospace;">${escapeVoucherText(v.code)}</code></td>
            <td>${escapeVoucherText(v.profile_name)}</td>
            <td>${Number(v.validity_days || 0)} يوم</td>
            <td>${Number(v.price || 0).toLocaleString()} د.ع</td>
            <td>
                <span class="badge ${used ? 'badge-danger' : 'badge-success'}">
                    ${used ? 'مستخدم' : 'متاح'}
                </span>
            </td>
            <td>${escapeVoucherText(v.used_by || '-')}</td>
            <td>${v.created_at ? new Date(v.created_at).toLocaleString('ar-IQ') : '-'}</td>
            <td>
                <button class="btn btn-secondary" style="padding:4px 8px; font-size:12px;" onclick="printVoucher('${escapeVoucherAttr(v.code)}', '${escapeVoucherAttr(v.profile_name)}', ${Number(v.validity_days || 0)})">طباعة</button>
                <button class="btn btn-danger" style="padding:4px 8px; font-size:12px; background:var(--danger); border-color:var(--danger);" onclick="deleteVoucher(${v.id})">حذف</button>
            </td>
        </tr>
    `;
}

async function showGenerateVoucherModal() {
    const select = document.getElementById('vch-profile-select');
    if (!select) return;

    await loadVoucherProfiles(select);
    openVoucherModal('voucher-modal');
}

async function loadVoucherProfiles(select) {
    const selectedProfile = select.value;
    select.innerHTML = '<option value="">جاري تحميل الباقات...</option>';

    try {
        const res = await apiFetch('/radius/api/profiles');
        if (!res.ok) throw new Error(`profiles request failed: ${res.status}`);

        const profiles = await res.json();
        if (!Array.isArray(profiles) || profiles.length === 0) {
            select.innerHTML = '<option value="">لا توجد باقات</option>';
            return;
        }

        select.innerHTML = '<option value="">-- اختر الباقة --</option>' + profiles.map(profile => {
            const name = profile.name || '';
            const price = Number(profile.price || 0);
            const validity = Number(profile.validity_days || 0);
            const details = [
                validity > 0 ? `${validity} يوم` : 'مفتوح',
                price > 0 ? `${price.toLocaleString()} د.ع` : ''
            ].filter(Boolean).join(' - ');
            return `<option value="${escapeVoucherAttr(name)}">${escapeVoucherText(name)}${details ? ` (${escapeVoucherText(details)})` : ''}</option>`;
        }).join('');

        if (selectedProfile && profiles.some(profile => profile.name === selectedProfile)) {
            select.value = selectedProfile;
        }
    } catch (e) {
        console.error('Failed to load voucher profiles', e);
        select.innerHTML = '<option value="">فشل تحميل الباقات</option>';
    }
}

function openVoucherModal(id) {
    if (typeof window.openModal === 'function') {
        window.openModal(id);
        return;
    }

    const el = document.getElementById(id);
    if (el) el.classList.add('active');
}

function closeVoucherModal(id) {
    if (typeof window.closeModal === 'function') {
        window.closeModal(id);
        return;
    }

    const el = document.getElementById(id);
    if (el) el.classList.remove('active');
}

function escapeVoucherText(value) {
    return String(value).replace(/[&<>"']/g, char => ({
        '&': '&amp;',
        '<': '&lt;',
        '>': '&gt;',
        '"': '&quot;',
        "'": '&#39;'
    }[char]));
}

function escapeVoucherAttr(value) {
    return escapeVoucherText(value);
}

async function generateVouchers() {
    const profile = document.getElementById('vch-profile-select').value;
    const count = parseInt(document.getElementById('vch-count').value, 10);
    const price = parseFloat(document.getElementById('vch-price').value) || 0;
    const codeType = document.getElementById('vch-code-type').value || 'alphanumeric';
    const codeLength = parseInt(document.getElementById('vch-code-length').value, 10) || 10;

    if (!profile) {
        alert('يرجى اختيار باقة');
        return;
    }
    if (!count || count < 1) {
        alert('يرجى إدخال عدد صحيح للكروت');
        return;
    }

    const btn = document.getElementById('vch-submit-btn');
    btn.disabled = true;
    btn.innerText = 'جاري التوليد...';

    try {
        const res = await apiFetch('/radius/api/vouchers/generate', {
            method: 'POST',
            body: JSON.stringify({ 
                profile_name: profile, 
                count: count, 
                price: price,
                code_type: codeType,
                code_length: codeLength
            })
        });
        const result = await res.json();
        if (result.error) {
            alert(result.error);
        } else {
            alert(result.message);
            closeVoucherModal('voucher-modal');
            loadVouchers();
        }
    } catch (e) {
        alert('حدث خطأ أثناء التوليد');
    } finally {
        btn.disabled = false;
        btn.innerText = 'توليد الآن';
    }
}

function printVoucher(code, profile, days) {
    printVoucherDocument([{
        code,
        profile_name: profile,
        validity_days: days
    }], `طباعة كرت - ${code}`);
}

function printVoucherGroup(groupKey) {
    const group = voucherGroupsCache[groupKey];
    if (!group) {
        alert('تعذر العثور على المجموعة للطباعة');
        return;
    }

    printVoucherDocument(group.vouchers, `طباعة مجموعة كروت - ${group.profile_name}`);
}

function printVoucherDocument(vouchers, title) {
    const printWindow = window.open('', '_blank');
    if (!printWindow) {
        alert('يرجى السماح بفتح النوافذ المنبثقة للطباعة');
        return;
    }

    const portalURL = window.location.origin + "/portal";

    printWindow.document.write(`
        <html dir="rtl" lang="ar">
        <head>
            <title>${escapeVoucherText(title)}</title>
            <link rel="stylesheet" href="/radius/fonts/google-fonts.css">
            <script src="/radius/vendor/qrcode/qrcode.min.js"></script>
            <style>
                * { box-sizing: border-box; }
                body { 
                    font-family: 'Cairo', Tahoma, Arial, sans-serif; 
                    margin: 0; 
                    padding: 20px; 
                    background: #f1f5f9; 
                    color: #1e293b; 
                }
                .toolbar {
                    position: sticky;
                    top: 0;
                    z-index: 100;
                    display: flex;
                    justify-content: space-between;
                    align-items: center;
                    margin: -20px -20px 20px;
                    padding: 15px 30px;
                    background: #fff;
                    box-shadow: 0 2px 4px rgba(0,0,0,0.1);
                }
                .toolbar button {
                    border: 0;
                    border-radius: 8px;
                    background: #2563eb;
                    color: #fff;
                    padding: 12px 24px;
                    font-weight: 700;
                    font-family: inherit;
                    cursor: pointer;
                    transition: background 0.2s;
                }
                .toolbar button:hover { background: #1d4ed8; }
                
                .sheet { 
                    display: grid; 
                    grid-template-columns: repeat(auto-fill, minmax(350px, 1fr)); 
                    gap: 20px; 
                }
                
                .card {
                    background: #fff;
                    border-radius: 16px;
                    overflow: hidden;
                    box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
                    border: 1px solid #e2e8f0;
                    position: relative;
                    display: flex;
                    flex-direction: column;
                    page-break-inside: avoid;
                    min-height: 220px;
                }
                
                .card-header {
                    background: linear-gradient(135deg, #1e3a8a 0%, #2563eb 100%);
                    color: white;
                    padding: 15px;
                    text-align: center;
                }
                
                .card-logo {
                    font-size: 18px;
                    font-weight: 800;
                    letter-spacing: 1px;
                }
                
                .card-body {
                    padding: 15px;
                    display: flex;
                    flex: 1;
                    gap: 15px;
                    align-items: center;
                }
                
                .voucher-info {
                    flex: 1;
                    text-align: right;
                }
                
                .profile-badge {
                    display: inline-block;
                    padding: 4px 10px;
                    background: #dbeafe;
                    color: #1e40af;
                    border-radius: 6px;
                    font-size: 12px;
                    font-weight: 700;
                    margin-bottom: 8px;
                }
                
                .validity {
                    font-size: 14px;
                    color: #64748b;
                    margin-bottom: 10px;
                }
                
                .code-container {
                    background: #f8fafc;
                    border: 2px dashed #cbd5e1;
                    padding: 10px;
                    border-radius: 8px;
                    text-align: center;
                }
                
                .code-label {
                    font-size: 10px;
                    color: #94a3b8;
                    display: block;
                    margin-bottom: 2px;
                }
                
                .code-value {
                    font-size: 22px;
                    font-weight: 800;
                    letter-spacing: 2px;
                    color: #1e293b;
                    font-family: monospace;
                    direction: ltr;
                }
                
                .qr-container {
                    width: 100px;
                    height: 100px;
                    background: #fff;
                    padding: 5px;
                    border: 1px solid #e2e8f0;
                    border-radius: 8px;
                }
                
                .qr-container img {
                    width: 100%;
                    height: 100%;
                }
                
                .card-footer {
                    background: #f8fafc;
                    padding: 8px;
                    text-align: center;
                    font-size: 10px;
                    color: #64748b;
                    border-top: 1px solid #e2e8f0;
                }

                @media print {
                    body { background: #fff; padding: 0; }
                    .toolbar { display: none; }
                    .sheet { 
                        grid-template-columns: repeat(2, 1fr); 
                        gap: 10mm; 
                        padding: 10mm;
                    }
                    .card { 
                        box-shadow: none; 
                        border: 1px solid #000; 
                    }
                }
            </style>
        </head>
        <body>
            <div class="toolbar">
                <strong>${escapeVoucherText(title)} - ${vouchers.length} كرت</strong>
                <button onclick="window.print()">طباعة الكروت 🖨️</button>
            </div>
            <div class="sheet">
                ${vouchers.map(v => {
                    const fullVoucherURL = portalURL + "?code=" + encodeURIComponent(v.code);
                    return `
                    <div class="card">
                        <div class="card-header">
                            <div class="card-logo">SASMAN NETWORK</div>
                        </div>
                        <div class="card-body">
                            <div class="qr-container" data-qr="${escapeVoucherAttr(fullVoucherURL)}"></div>
                            <div class="voucher-info">
                                <div class="profile-badge">${escapeVoucherText(v.profile_name)}</div>
                                <div class="validity">مدة الاشتراك: <strong>${Number(v.validity_days || 0)} يوم</strong></div>
                                <div class="code-container">
                                    <span class="code-label">كود التفعيل</span>
                                    <span class="code-value">${escapeVoucherText(v.code)}</span>
                                </div>
                            </div>
                        </div>
                        <div class="card-footer">
                            امسح كود QR أو ادخل الكود يدوياً في بوابة المشترك لتفعيل الخدمة
                        </div>
                    </div>
                `}).join('')}
            </div>
            <script>
                document.querySelectorAll('.qr-container').forEach(function(el) {
                    new QRCode(el, {
                        text: el.getAttribute('data-qr'),
                        width: 90,
                        height: 90,
                        correctLevel: QRCode.CorrectLevel.M
                    });
                });
            </script>
        </body>
        </html>
    `);
    printWindow.document.close();
}

window.addEventListener('tabChanged', (e) => {
    if (e.detail.tab === 'vouchers') {
        loadVouchers();
    }
});

async function deleteVoucher(id) {
    if (!confirm('هل أنت متأكد من حذف هذا الكرت؟ لا يمكن التراجع عن هذا الإجراء.')) return;
    
    try {
        const res = await apiFetch(`/radius/api/vouchers/${id}`, { method: 'DELETE' });
        const result = await res.json();
        if (!res.ok) throw new Error(result.error || 'فشل في الحذف');
        alert(result.message || 'تم الحذف بنجاح');
        loadVouchers();
    } catch (e) {
        alert(e.message);
    }
}

async function deleteVoucherBatch(batchId) {
    if (!confirm('هل أنت متأكد من حذف جميع الكروت في هذه المجموعة؟')) return;
    
    try {
        const res = await apiFetch(`/radius/api/vouchers/batch/${encodeURIComponent(batchId)}`, { method: 'DELETE' });
        const result = await res.json();
        if (!res.ok) throw new Error(result.error || 'فشل في حذف المجموعة');
        alert(result.message || 'تم حذف المجموعة بنجاح');
        loadVouchers();
    } catch (e) {
        alert(e.message);
    }
}

async function clearAllVouchers() {
    if (!confirm('تنبيه خطير! هل أنت متأكد من أنك تريد ضبط المصنع للكروت وحذف جميع الكروت من النظام تماماً؟ هذا الإجراء لا يمكن التراجع عنه!')) return;
    if (!confirm('تأكيد أخير: هل أنت متأكد حقاً من حذف جميع الكروت؟')) return;
    
    try {
        const res = await apiFetch(`/radius/api/vouchers/all/clear`, { method: 'DELETE' });
        const result = await res.json();
        if (!res.ok) throw new Error(result.error || 'فشل في مسح جميع الكروت');
        alert(result.message || 'تم مسح جميع الكروت بنجاح');
        loadVouchers();
    } catch (e) {
        alert(e.message);
    }
}
