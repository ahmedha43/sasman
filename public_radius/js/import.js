async function handleSAS4Migration() {
    const url = document.getElementById('sas4-url').value.trim();
    const username = document.getElementById('sas4-user').value.trim();
    const password = document.getElementById('sas4-pass').value.trim();
    const statusDiv = document.getElementById('sas4-status');

    if (!url || !username || !password) {
        statusDiv.style.color = 'var(--danger)';
        statusDiv.innerText = '⚠️ يرجى إدخال الرابط واسم المستخدم وكلمة المرور.';
        return;
    }

    statusDiv.style.color = 'var(--primary)';
    statusDiv.innerText = '⏳ جارِ جلب البيانات من SAS 4... قد يستغرق هذا بعض الوقت.';

    try {
        const resp = await fetch('/radius/api/import/sas4', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${localStorage.getItem('radius_token')}`
            },
            body: JSON.stringify({ url, username, password })
        });

        const result = await resp.json();

        if (resp.ok) {
            statusDiv.style.color = 'var(--success)';
            statusDiv.innerText = `✅ ${result.message}`;
            // Refresh stats to show new counts
            if (typeof fetchStats === 'function') fetchStats();
        } else {
            statusDiv.style.color = 'var(--danger)';
            statusDiv.innerText = `❌ فشل الجلب: ${result.error || 'خطأ غير معروف'}`;
        }
    } catch (err) {
        statusDiv.style.color = 'var(--danger)';
        statusDiv.innerText = `❌ خطأ في الاتصال بالسيرفر: ${err.message}`;
    }
}

async function handleSystemReset() {
    if (!confirm('🚨 هل أنت متأكد تماماً؟ سيتم حذف جميع المشتركين والباقات والسجلات نهائياً!')) {
        return;
    }

    if (!confirm('❗ هذه هي الفرصة الأخيرة، هل تريد حقاً تصفير النظام بالكامل؟')) {
        return;
    }

    try {
        const resp = await fetch('/radius/api/system/reset', {
            method: 'POST',
            headers: {
                'Authorization': `Bearer ${localStorage.getItem('radius_token')}`
            }
        });

        const result = await resp.json();

        if (resp.ok) {
            alert('✅ ' + result.message);
            window.location.reload();
        } else {
            alert('❌ فشل الحذف: ' + (result.error || 'خطأ غير معروف'));
        }
    } catch (err) {
        alert('❌ خطأ في الاتصال بالسيرفر: ' + err.message);
    }
}

async function importFromExcel() {
    const fileInput = document.getElementById('sas4-excel-file');
    const statusDiv = document.getElementById('sas4-status');

    if (!fileInput || !fileInput.files || fileInput.files.length === 0) {
        statusDiv.style.color = 'var(--danger)';
        statusDiv.innerText = '⚠️ يرجى اختيار ملف Excel (.xlsx) أولاً.';
        return;
    }

    const file = fileInput.files[0];
    const formData = new FormData();
    formData.append('file', file);

    statusDiv.style.color = 'var(--primary)';
    statusDiv.innerText = '⏳ جارِ استيراد البيانات من ملف Excel... قد يستغرق هذا بعض الوقت.';

    try {
        const resp = await fetch('/radius/api/import/excel', {
            method: 'POST',
            headers: {
                'Authorization': `Bearer ${localStorage.getItem('radius_token')}`
            },
            body: formData
        });

        const result = await resp.json();

        if (resp.ok) {
            statusDiv.style.color = 'var(--success)';
            statusDiv.innerText = `✅ ${result.message}`;
            // Refresh stats to show new counts
            if (typeof fetchStats === 'function') fetchStats();
            // Clear file input
            fileInput.value = '';
        } else {
            statusDiv.style.color = 'var(--danger)';
            statusDiv.innerText = `❌ فشل الاستيراد: ${result.error || 'خطأ غير معروف'}`;
        }
    } catch (err) {
        statusDiv.style.color = 'var(--danger)';
        statusDiv.innerText = `❌ خطأ في الاتصال بالسيرفر: ${err.message}`;
    }
}

async function exportToExcel() {
    const statusDiv = document.getElementById('sas4-status');
    statusDiv.style.color = 'var(--primary)';
    statusDiv.innerText = '⏳ جارِ تحضير وتصدير ملف Excel...';

    try {
        const url = `/radius/api/export/excel`;
        const a = document.createElement('a');
        a.href = url;
        a.download = 'users_export.xlsx';
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        
        statusDiv.style.color = 'var(--success)';
        statusDiv.innerText = '✅ تم تصدير وتحميل ملف Excel بنجاح.';
    } catch (err) {
        statusDiv.style.color = 'var(--danger)';
        statusDiv.innerText = `❌ فشل التصدير: ${err.message}`;
    }
}

// ==========================================================================
// Scheduled Internet Shutdown (Blackout Scheduler)
// ==========================================================================

// ==========================================================================
// Scheduled Internet Shutdown (Blackout Scheduler)
// ==========================================================================

let shutdownDates = [];
let shutdownEditIndex = -1;
let excludedUsers = [];

function escapeHtml(value) {
    return String(value ?? '')
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}

function formatDateArabic(dateStr) {
    if (!dateStr) return '';
    try {
        // Append T12:00:00 to avoid timezone shift to previous day
        const date = new Date(dateStr + 'T12:00:00');
        if (isNaN(date.getTime())) return dateStr;
        
        return date.toLocaleDateString('ar-EG', { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' });
    } catch (e) {
        return dateStr;
    }
}

async function loadShutdownConfig() {
    try {
        const resp = await apiFetch('/radius/api/auth/shutdown/config');
        if (!resp.ok) return;
        const config = await resp.json();

        const enabledEl   = document.getElementById('shutdown-enabled');
        const startEl     = document.getElementById('shutdown-start-time');
        const endEl       = document.getElementById('shutdown-end-time');

        if (!enabledEl) return; // Card not present for this admin role

        enabledEl.value = config.enabled ? '1' : '0';
        if (startEl) startEl.value = config.start_time || '06:00:00';
        if (endEl)   endEl.value   = config.end_time   || '07:30:00';
        
        shutdownDates = Array.isArray(config.dates) ? config.dates.map(d => d.trim()).filter(d => d.length > 0) : [];
        excludedUsers = Array.isArray(config.excluded_users) ? config.excluded_users.map(u => u.trim()).filter(u => u.length > 0) : [];
        shutdownEditIndex = -1;
        renderShutdownDates();
        renderExcludedUsers();
        populateUsersDatalist();
    } catch (e) {
        console.error('[shutdown] Failed to load shutdown config:', e);
    }
}

function populateUsersDatalist() {
    const datalist = document.getElementById('users-datalist');
    if (!datalist) return;
    
    datalist.innerHTML = '';
    
    let usersList = typeof radiusUsersCache !== 'undefined' ? radiusUsersCache : [];
    if (usersList.length === 0) {
        apiFetch('/radius/api/users')
            .then(res => res.json())
            .then(users => {
                if (Array.isArray(users)) {
                    if (typeof radiusUsersCache !== 'undefined') {
                        radiusUsersCache = users;
                    }
                    fillDatalist(users);
                }
            })
            .catch(err => console.error('[shutdown] Failed to preload users for exclusions:', err));
    } else {
        fillDatalist(usersList);
    }

    function fillDatalist(list) {
        datalist.innerHTML = list
            .map(u => `<option value="${escapeHtml(u.user)}">${escapeHtml(u.full_name ? u.full_name + ' (' + u.user + ')' : u.user)}</option>`)
            .join('');
    }
}

function renderShutdownDates() {
    const container = document.getElementById('shutdown-dates-container');
    if (!container) return;

    container.innerHTML = '';

    if (shutdownDates.length === 0) {
        container.innerHTML = `
            <div style="text-align: center; padding: 25px; border: 2px dashed var(--border); border-radius: 12px; color: var(--text-muted); background: rgba(0,0,0,0.01); font-size: 0.9rem;">
                <i class="fa-regular fa-calendar-xmark" style="font-size: 2rem; margin-bottom: 10px; display: block; color: var(--danger); opacity: 0.7;"></i>
                لا توجد تواريخ مضافة حالياً لقطع الخدمة. اختر تاريخاً من الأعلى واضغط "إضافة اليوم".
            </div>
        `;
        return;
    }

    // Sort dates chronologically
    const sortedIndices = shutdownDates
        .map((date, idx) => ({ date, originalIndex: idx }))
        .sort((a, b) => new Date(a.date) - new Date(b.date));

    sortedIndices.forEach(({ date, originalIndex }) => {
        const isEditing = originalIndex === shutdownEditIndex;

        if (isEditing) {
            container.innerHTML += `
                <div class="date-item-row edit-mode" style="display: flex; justify-content: space-between; align-items: center; padding: 10px 16px; background: rgba(59, 130, 246, 0.05); border: 2px solid #3b82f6; border-radius: 8px; box-shadow: 0 2px 8px rgba(59, 130, 246, 0.08);">
                    <div style="display: flex; align-items: center; gap: 12px; flex: 1;">
                        <span style="color: #3b82f6; font-size: 1.2rem; display: flex; align-items: center;"><i class="fa-solid fa-calendar-check"></i></span>
                        <input type="date" id="edit-date-input-${originalIndex}" value="${date}" style="padding: 8px 12px; border-radius: 6px; border: 1px solid #3b82f6; font-family: monospace; font-size: 0.95rem; flex: 1; max-width: 220px; outline: none; background: white;">
                    </div>
                    <div style="display: flex; gap: 8px; align-items: center;">
                        <button class="btn" onclick="saveEditShutdownDate(${originalIndex})" style="width: auto; padding: 8px 14px; font-size: 0.85rem; background: #10b981; color: white; border: none; display: flex; align-items: center; gap: 6px; border-radius: 6px; cursor: pointer; font-weight: bold;">
                            <i class="fa-solid fa-check"></i> حفظ
                        </button>
                        <button class="btn" onclick="cancelEditShutdownDate()" style="width: auto; padding: 8px 14px; font-size: 0.85rem; background: #6b7280; color: white; border: none; display: flex; align-items: center; gap: 6px; border-radius: 6px; cursor: pointer; font-weight: bold;">
                            <i class="fa-solid fa-xmark"></i> إلغاء
                        </button>
                    </div>
                </div>
            `;
        } else {
            container.innerHTML += `
                <div class="date-item-row" style="display: flex; justify-content: space-between; align-items: center; padding: 12px 16px; background: white; border: 1px solid var(--border); border-radius: 8px; transition: all 0.25s ease; box-shadow: 0 1px 3px rgba(0,0,0,0.01);">
                    <div style="display: flex; align-items: center; gap: 12px;">
                        <span style="color: var(--danger); font-size: 1.2rem; display: flex; align-items: center;"><i class="fa-solid fa-calendar-day"></i></span>
                        <div>
                            <strong style="color: var(--text); font-family: monospace; font-size: 0.95rem; direction: ltr; display: inline-block;">${date}</strong>
                            <div style="font-size: 0.8rem; color: var(--text-muted); margin-top: 2px;">${formatDateArabic(date)}</div>
                        </div>
                    </div>
                    <div style="display: flex; gap: 8px; align-items: center;">
                        <button class="btn" onclick="startEditShutdownDate(${originalIndex})" style="width: auto; padding: 8px 14px; font-size: 0.85rem; background: #3b82f6; color: white; border: none; display: flex; align-items: center; gap: 6px; border-radius: 6px; cursor: pointer; font-weight: bold;">
                            <i class="fa-solid fa-pen-to-square"></i> تعديل
                        </button>
                        <button class="btn" onclick="deleteShutdownDate(${originalIndex})" style="width: auto; padding: 8px 14px; font-size: 0.85rem; background: #ef4444; color: white; border: none; display: flex; align-items: center; gap: 6px; border-radius: 6px; cursor: pointer; font-weight: bold;">
                            <i class="fa-solid fa-trash"></i> حذف
                        </button>
                    </div>
                </div>
            `;
        }
    });
}

function addShutdownDate() {
    const picker = document.getElementById('shutdown-date-input');
    if (!picker) return;

    const dateVal = picker.value.trim();
    if (!dateVal) {
        alert('⚠️ يرجى اختيار تاريخ أولاً.');
        return;
    }

    if (shutdownDates.includes(dateVal)) {
        alert('⚠️ هذا التاريخ مضاف بالفعل في الجدول.');
        return;
    }

    shutdownDates.push(dateVal);
    shutdownDates.sort();
    
    renderShutdownDates();
    picker.value = '';
    showToast('✅ تم إضافة التاريخ بنجاح', 'success');
}

function deleteShutdownDate(index) {
    if (confirm('هل أنت متأكد من حذف هذا التاريخ من جدول القطع؟')) {
        shutdownDates.splice(index, 1);
        if (shutdownEditIndex === index) {
            shutdownEditIndex = -1;
        } else if (shutdownEditIndex > index) {
            shutdownEditIndex--;
        }
        renderShutdownDates();
        showToast('🗑️ تم حذف التاريخ', 'info');
    }
}

function startEditShutdownDate(index) {
    shutdownEditIndex = index;
    renderShutdownDates();
}

function cancelEditShutdownDate() {
    shutdownEditIndex = -1;
    renderShutdownDates();
}

function saveEditShutdownDate(index) {
    const input = document.getElementById(`edit-date-input-${index}`);
    if (!input) return;

    const newVal = input.value.trim();
    if (!newVal) {
        alert('⚠️ لا يمكن ترك التاريخ فارغاً.');
        return;
    }

    const exists = shutdownDates.some((d, idx) => d === newVal && idx !== index);
    if (exists) {
        alert('⚠️ هذا التاريخ مضاف بالفعل في الجدول.');
        return;
    }

    shutdownDates[index] = newVal;
    shutdownDates.sort();
    
    shutdownEditIndex = -1;
    renderShutdownDates();
    showToast('✅ تم تعديل التاريخ بنجاح', 'success');
}

function renderExcludedUsers() {
    const container = document.getElementById('shutdown-excluded-container');
    if (!container) return;

    container.innerHTML = '';

    if (excludedUsers.length === 0) {
        container.innerHTML = `
            <div style="width: 100%; padding: 15px; text-align: center; color: var(--text-muted); border: 1px solid var(--border); border-radius: 8px; background: rgba(0,0,0,0.01); font-size: 0.85rem;">
                لا توجد حسابات مستثناة حالياً. اكتب اسم مستخدم في الحقل أعلاه واضغط "إضافة للاستثناء".
            </div>
        `;
        return;
    }

    excludedUsers.forEach(username => {
        container.innerHTML += `
            <div class="excluded-user-chip" style="display: inline-flex; align-items: center; gap: 8px; padding: 8px 14px; background: rgba(16, 185, 129, 0.08); border: 1px solid rgba(16, 185, 129, 0.25); color: #065f46; border-radius: 20px; font-size: 0.85rem; font-weight: 600; box-shadow: 0 1px 2px rgba(0,0,0,0.02); margin: 2px;">
                <span><i class="fa-solid fa-user-shield" style="color: #10b981; margin-left: 4px;"></i>${escapeHtml(username)}</span>
                <button type="button" onclick="deleteExcludedUser('${escapeHtml(username)}')" style="background: none; border: none; color: #ef4444; cursor: pointer; padding: 0 4px; font-size: 1.1rem; display: flex; align-items: center; line-height: 1;" title="إزالة من الاستثناء">×</button>
            </div>
        `;
    });
}

function addExcludedUser() {
    const input = document.getElementById('shutdown-exclude-input');
    if (!input) return;

    const username = input.value.trim();
    if (!username) {
        alert('⚠️ يرجى كتابة أو اختيار اسم مستخدم أولاً.');
        return;
    }

    if (excludedUsers.some(u => u.toLowerCase() === username.toLowerCase())) {
        alert('⚠️ هذا المستخدم مضاف بالفعل في قائمة الاستثناء.');
        return;
    }

    excludedUsers.push(username);
    renderExcludedUsers();
    input.value = '';
    showToast('✅ تم إضافة المستخدم لقائمة الاستثناء بنجاح', 'success');
}

function deleteExcludedUser(username) {
    if (confirm(`هل أنت متأكد من إزالة المستخدم "${username}" من قائمة الاستثناء؟`)) {
        excludedUsers = excludedUsers.filter(u => u.toLowerCase() !== username.toLowerCase());
        renderExcludedUsers();
        showToast('🗑️ تم إزالة المستخدم من الاستثناء', 'info');
    }
}

async function saveShutdownConfig() {
    const enabledEl = document.getElementById('shutdown-enabled');
    const startEl   = document.getElementById('shutdown-start-time');
    const endEl     = document.getElementById('shutdown-end-time');

    if (!enabledEl || !startEl || !endEl) {
        alert('❌ تعذر الوصول إلى حقول الإعداد.');
        return;
    }

    const enabled    = enabledEl.value === '1';
    const start_time = startEl.value.trim();
    const end_time   = endEl.value.trim();

    if (!start_time || !end_time) {
        alert('⚠️ يرجى إدخال وقت البدء ووقت الانتهاء.');
        return;
    }

    try {
        const resp = await apiFetch('/radius/api/auth/shutdown/config', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ enabled, start_time, end_time, dates: shutdownDates, excluded_users: excludedUsers })
        });

        const result = await resp.json();

        if (resp.ok) {
            alert('✅ ' + (result.message || 'تم حفظ الإعدادات بنجاح'));
        } else {
            alert('❌ ' + (result.error || 'فشل حفظ الإعدادات'));
        }
    } catch (e) {
        console.error('[shutdown] Failed to save shutdown config:', e);
        alert('❌ خطأ في الاتصال بالسيرفر: ' + e.message);
    }
}

window.loadShutdownConfig  = loadShutdownConfig;
window.saveShutdownConfig  = saveShutdownConfig;
window.addShutdownDate      = addShutdownDate;
window.deleteShutdownDate   = deleteShutdownDate;
window.startEditShutdownDate = startEditShutdownDate;
window.cancelEditShutdownDate = cancelEditShutdownDate;
window.saveEditShutdownDate = saveEditShutdownDate;
window.addExcludedUser      = addExcludedUser;
window.deleteExcludedUser   = deleteExcludedUser;

