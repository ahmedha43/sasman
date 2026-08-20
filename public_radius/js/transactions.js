let currentTransactionUser = '';

function openTransactionModal(encodedUser, type) {
    const username = decodeURIComponent(encodedUser);
    currentTransactionUser = username;
    const title = type === 'debt' ? '➕ إضافة ديون' : '➖ تسديد ديون';
    document.getElementById('transaction-modal-title').innerText = title;
    document.getElementById('transaction-modal-user').innerText = `المشترك: ${username}`;
    document.getElementById('transaction-type').value = type;
    document.getElementById('transaction-amount').value = '';
    document.getElementById('transaction-notes').value = '';
    document.getElementById('transaction-modal').classList.add('active');
}

function closeTransactionModal() {
    currentTransactionUser = '';
    document.getElementById('transaction-modal').classList.remove('active');
}

async function submitTransaction() {
    if (!currentTransactionUser) return;

    const type = document.getElementById('transaction-type').value;
    const amount = parseFloat(document.getElementById('transaction-amount').value);
    const notes = document.getElementById('transaction-notes').value.trim();

    if (!amount || amount <= 0) {
        alert('يرجى إدخال مبلغ صحيح أكبر من صفر');
        return;
    }

    const res = await apiFetch(`/radius/api/users/${encodeURIComponent(currentTransactionUser)}/transactions`, {
        method: 'POST',
        body: JSON.stringify({ type, amount, notes })
    });

    const result = await res.json();
    alert(result.message || result.error);

    if (res.ok) {
        closeTransactionModal();
        loadUsers();
    }
}

async function openUserDetails(encodedUser) {
    const username = decodeURIComponent(encodedUser);
    try {
        const res = await apiFetch(`/radius/api/users/${encodeURIComponent(username)}/details`);
        if (!res.ok) {
            const err = await res.json();
            alert(err.error || 'تعذر تحميل التفاصيل');
            return;
        }
        const data = await res.json();
        renderUserDetails(data);
        document.getElementById('user-details-modal').classList.add('active');
    } catch (e) {
        console.error(e);
        alert('خطأ في تحميل التفاصيل');
    }
}

function closeUserDetailsModal() {
    document.getElementById('user-details-modal').classList.remove('active');
}

function renderUserDetails(data) {
    const s = data.session || {};
    const statusMap = {
        online: { text: 'متصل', className: 'badge-success' },
        stale: { text: 'تأخر التحديث', className: 'badge-warning' },
        offline: { text: 'غير متصل', className: 'badge-secondary' },
        expired: { text: 'منتهي', className: 'badge-danger' },
        expired_online: { text: 'منتهي (متصل)', className: 'badge-warning' }
    };
    const st = statusMap[s.status] || statusMap.offline;
    const balanceClass = (data.balance || 0) > 0 ? 'badge-danger' : 'badge-success';

    const transactionsHtml = (data.transactions && data.transactions.length)
        ? data.transactions.map(t => `
            <tr>
                <td><span class="badge ${t.type === 'debt' ? 'badge-danger' : 'badge-success'}">${t.type === 'debt' ? 'ديون' : 'تسديد'}</span></td>
                <td style="font-weight:600; color:var(--text-main);">${t.amount.toLocaleString()} د.ع</td>
                <td>${t.notes || '-'}</td>
                <td style="color:var(--text-muted); font-size:13px;">${new Date(t.created_at).toLocaleString('ar')}</td>
            </tr>
        `).join('')
        : '<tr><td colspan="4" style="text-align:center; color:var(--text-muted);">لا توجد عمليات مالية</td></tr>';

    const sessionsHtml = (data.session_history && data.session_history.length)
        ? data.session_history.map(s => `
            <tr>
                <td>${s.started_at || '-'}</td>
                <td>${s.stopped_at || '<span class="badge badge-success" style="font-size:11px;">نشطة</span>'}</td>
                <td><span style="font-family:monospace; color:var(--text-main);">${s.ip || '-'}</span></td>
                <td>${s.download || '-'}</td>
                <td>${s.upload || '-'}</td>
                <td>${s.session_time ? formatDuration(s.session_time) : '-'}</td>
                <td><span style="font-family:monospace; font-size:12px; color:var(--text-muted);">${s.calling_station || '-'}</span></td>
            </tr>
        `).join('')
        : '<tr><td colspan="7" style="text-align:center; color:var(--text-muted);">لا توجد جلسات سابقة</td></tr>';

    const html = `
        <div class="grid" style="margin-bottom:20px;">
            <div class="card" style="margin-bottom:0;">
                <h4 style="color:var(--text-main); font-weight:700; margin-bottom:15px;">📋 البيانات الأساسية</h4>
                <table style="font-size:14px;">
                    <tr><td style="border:none; padding:6px 0; color:var(--text-muted);">المستخدم:</td><td style="border:none; padding:6px 0; font-weight:600; color:var(--text-main);">${escapeHtml(data.user)}</td></tr>
                    <tr><td style="border:none; padding:6px 0; color:var(--text-muted);">الاسم الكامل:</td><td style="border:none; padding:6px 0; color:var(--text-main);">${data.full_name ? escapeHtml(data.full_name) : '<span style="color:var(--text-muted);">غير محدد</span>'}</td></tr>
                    <tr><td style="border:none; padding:6px 0; color:var(--text-muted);">رقم الهاتف:</td><td style="border:none; padding:6px 0; direction:ltr; text-align:right; color:var(--text-main);">${data.phone ? escapeHtml(data.phone) : '<span style="color:var(--text-muted);">غير محدد</span>'}</td></tr>
                    <tr><td style="border:none; padding:6px 0; color:var(--text-muted);">الباقة:</td><td style="border:none; padding:6px 0;"><span class="badge" style="background:rgba(14, 165, 233, 0.1); color:#0284c7;">${escapeHtml(data.profile || 'بدون باقة')}</span></td></tr>
                    <tr><td style="border:none; padding:6px 0; color:var(--text-muted);">تاريخ الانتهاء:</td><td style="border:none; padding:6px 0; color:var(--text-main);">${data.expires_at ? escapeHtml(data.expires_at) : '<span style="color:var(--text-muted);">غير محدد</span>'}</td></tr>
                    <tr><td style="border:none; padding:6px 0; color:var(--text-muted);">الرصيد:</td><td style="border:none; padding:6px 0;"><span class="badge ${balanceClass}">${(data.balance || 0).toLocaleString()} د.ع</span></td></tr>
                    <tr><td style="border:none; padding:6px 0; color:var(--text-muted);">الحالة:</td><td style="border:none; padding:6px 0;"><span class="badge ${st.className}">${st.text}</span></td></tr>
                </table>
            </div>
            <div class="card" style="margin-bottom:0;">
                <h4 style="color:var(--text-main); font-weight:700; margin-bottom:15px;">🌐 تفاصيل الجلسة الحالية</h4>
                ${(s.online || s.ip || s.session_seconds) ? `
                    <table style="font-size:14px;">
                        <tr><td style="border:none; padding:6px 0; color:var(--text-muted);">الحالة:</td><td style="border:none; padding:6px 0;"><span class="badge ${st.className}">${st.text}</span></td></tr>
                        <tr><td style="border:none; padding:6px 0; color:var(--text-muted);">عنوان IP:</td><td style="border:none; padding:6px 0; font-family:monospace; font-weight:bold; color:var(--primary);">${escapeHtml(s.ip || 'غير محدد')}</td></tr>
                        <tr><td style="border:none; padding:6px 0; color:var(--text-muted);">الماك أدرس (MAC):</td><td style="border:none; padding:6px 0; font-family:monospace; font-weight:bold; color:var(--text-main);">${escapeHtml(s.calling_station || '-')}</td></tr>
                        <tr><td style="border:none; padding:6px 0; color:var(--text-muted);">مدة الاتصال (Uptime):</td><td style="border:none; padding:6px 0; font-weight:bold; color:var(--success); font-family:monospace;">⏱️ ${formatDuration(s.session_seconds || 0)}</td></tr>
                        <tr><td style="border:none; padding:6px 0; color:var(--text-muted);">وقت بدء الجلسة:</td><td style="border:none; padding:6px 0; font-family:monospace; color:var(--text-main);">${escapeHtml(s.started_at || '-')}</td></tr>
                        <tr><td style="border:none; padding:6px 0; color:var(--text-muted);">البيانات المستهلكة:</td><td style="border:none; padding:6px 0; font-family:monospace;"><span style="color:var(--info); font-weight:600;">⬇️ ${escapeHtml(s.download || '0 B')}</span> &nbsp;|&nbsp; <span style="color:var(--primary); font-weight:600;">⬆️ ${escapeHtml(s.upload || '0 B')}</span></td></tr>
                        <tr><td style="border:none; padding:6px 0; color:var(--text-muted);">سيرفر الراوتر (NAS):</td><td style="border:none; padding:6px 0; font-family:monospace; font-size:12px; color:var(--text-muted);">${escapeHtml(s.nas_ip || '-')}</td></tr>
                        <tr><td style="border:none; padding:6px 0; color:var(--text-muted);">كود الجلسة (Session ID):</td><td style="border:none; padding:6px 0; font-family:monospace; font-size:11px; color:var(--text-muted);">${escapeHtml(s.session_id || '-')}</td></tr>
                    </table>
                ` : '<div style="padding:20px 0; text-align:center; color:var(--text-muted); font-size:14px;"><i class="fa-solid fa-circle-xmark" style="font-size:24px; display:block; margin-bottom:8px; opacity:0.5;"></i>لا توجد جلسة نشطة حالياً</div>'}
            </div>
        </div>

        <div class="card" style="border-right:4px solid #8b5cf6;">
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:12px;">
                <h4 style="margin:0;">💰 السجل المالي</h4>
                <div class="action-row">
                    <button class="btn" style="background:#f59e0b; color:white; border-color:#f59e0b; width:auto; padding:6px 14px;" onclick="openTransactionModal('${encodeURIComponent(data.user)}', 'debt')">➕ إضافة ديون</button>
                    <button class="btn" style="background:#10b981; color:white; border-color:#10b981; width:auto; padding:6px 14px;" onclick="openTransactionModal('${encodeURIComponent(data.user)}', 'payment')">➖ تسديد ديون</button>
                </div>
            </div>
            <div style="overflow-x:auto;">
                <table>
                    <thead>
                        <tr>
                            <th>النوع</th>
                            <th>المبلغ</th>
                            <th>ملاحظات</th>
                            <th>التاريخ</th>
                        </tr>
                    </thead>
                    <tbody>${transactionsHtml}</tbody>
                </table>
            </div>
        </div>

        <div class="card" style="margin-top:16px; border-right:4px solid #0891b2;">
            <h4 style="margin:0 0 12px 0;">📡 سجل الجلسات</h4>
            <div style="overflow-x:auto;">
                <table>
                    <thead>
                        <tr>
                            <th>بدأت</th>
                            <th>انتهت</th>
                            <th>IP</th>
                            <th>تنزيل</th>
                            <th>رفع</th>
                            <th>المدة</th>
                            <th>MAC</th>
                        </tr>
                    </thead>
                    <tbody>${sessionsHtml}</tbody>
                </table>
            </div>
        </div>
    `;

    document.getElementById('user-details-content').innerHTML = html;
}

function formatDuration(seconds) {
    if (!seconds || seconds <= 0) return '0ث';
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = seconds % 60;
    if (h > 0) return `${h}س ${m}د ${s}ث`;
    if (m > 0) return `${m}د ${s}ث`;
    return `${s}ث`;
}
