/**
 * SASMAN Network Devices Module
 * Manages Switches, PtP Links, and Sector APs with Vendor-Agnostic Architecture
 */

function formatBytes(bytes, decimals = 1) {
    const b = parseInt(bytes, 10);
    if (!b || isNaN(b) || b <= 0) return '0 B';
    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
    const i = Math.floor(Math.log(b) / Math.log(k));
    if (i < 0) return '0 B';
    const index = Math.min(i, sizes.length - 1);
    return parseFloat((b / Math.pow(k, index)).toFixed(dm)) + ' ' + sizes[index];
}

function formatDuration(seconds) {
    const s = parseInt(seconds, 10);
    if (!s || isNaN(s) || s <= 0) return '0 ثانية';

    const days = Math.floor(s / 86400);
    const hours = Math.floor((s % 86400) / 3600);
    const minutes = Math.floor((s % 3600) / 60);
    const remainingSeconds = s % 60;

    const parts = [];
    if (days > 0) parts.push(`${days} يوم`);
    if (hours > 0) parts.push(`${hours} س`);
    if (minutes > 0) parts.push(`${minutes} د`);
    if (parts.length === 0) {
        parts.push(`${remainingSeconds} ث`);
    }

    return parts.join(' ');
}

let allDevicesCache = [];
let allVendorsCache = [];
let allTypesCache = [];
let currentDeviceFilter = 'all';
let currentVendorFilter = 'all';
let currentStatusFilter = 'all';
let currentSearchQuery = '';
let activeDetailDevice = null;
let currentDetailTab = 'overview';
let activePollInterval = null;

async function loadDevices() {
    try {
        await Promise.all([
            fetchDevicesSummary(),
            fetchVendorsAndTypes(),
            fetchDevicesList()
        ]);
    } catch (e) {
        console.error('[NetworkDevices] Load failed:', e);
    }
}

async function fetchVendorsAndTypes() {
    try {
        const [vRes, tRes] = await Promise.all([
            apiFetch('/radius/api/devices/vendors'),
            apiFetch('/radius/api/devices/types')
        ]);
        if (vRes.ok) allVendorsCache = await vRes.json();
        if (tRes.ok) allTypesCache = await tRes.json();

        populateVendorAndTypeSelects();
    } catch (e) {
        console.error('[NetworkDevices] Failed to fetch vendors/types:', e);
    }
}

async function fetchDevicesSummary() {
    try {
        const res = await apiFetch('/radius/api/devices/summary');
        if (!res.ok) return;
        const stats = await res.json();

        const elTotal = document.getElementById('stat-dev-total');
        const elOnline = document.getElementById('stat-dev-online');
        const elOffline = document.getElementById('stat-dev-offline');
        const elSwitches = document.getElementById('stat-dev-switches');
        const elLinks = document.getElementById('stat-dev-links');
        const elSectors = document.getElementById('stat-dev-sectors');
        const elClients = document.getElementById('stat-dev-clients');

        if (elTotal) elTotal.textContent = stats.total_devices || 0;
        if (elOnline) elOnline.textContent = stats.online_devices || 0;
        if (elOffline) elOffline.textContent = stats.offline_devices || 0;
        if (elSwitches) elSwitches.textContent = stats.switches_count || 0;
        if (elLinks) elLinks.textContent = stats.links_count || 0;
        if (elSectors) elSectors.textContent = stats.sectors_count || 0;
        if (elClients) elClients.textContent = stats.total_clients || 0;
    } catch (e) {
        console.error(e);
    }
}

async function fetchDevicesList() {
    const tableBody = document.getElementById('devices-tbody');
    if (!tableBody) return;

    try {
        let url = '/radius/api/devices?';
        if (currentDeviceFilter !== 'all') url += `type=${encodeURIComponent(currentDeviceFilter)}&`;
        if (currentVendorFilter !== 'all') url += `vendor=${encodeURIComponent(currentVendorFilter)}&`;
        if (currentStatusFilter !== 'all') url += `status=${encodeURIComponent(currentStatusFilter)}&`;

        const res = await apiFetch(url);
        if (!res.ok) throw new Error('فشل جلب قائمة الأجهزة');
        allDevicesCache = await res.json() || [];

        renderDevicesList(allDevicesCache);
    } catch (e) {
        console.error(e);
        if (tableBody) {
            tableBody.innerHTML = `<tr><td colspan="8" style="text-align:center; color:var(--danger); padding:24px;">❌ حدث خطأ في تحميل الأجهزة</td></tr>`;
        }
    }
}

function renderDevicesList(devices) {
    const tbody = document.getElementById('devices-tbody');
    if (!tbody) return;

    let filtered = devices;
    if (currentSearchQuery) {
        const q = currentSearchQuery.toLowerCase();
        filtered = filtered.filter(d => 
            (d.name && d.name.toLowerCase().includes(q)) ||
            (d.ip && d.ip.includes(q)) ||
            (d.model_name && d.model_name.toLowerCase().includes(q)) ||
            (d.location && d.location.toLowerCase().includes(q)) ||
            (d.board_name && d.board_name.toLowerCase().includes(q))
        );
    }

    if (filtered.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="8" style="text-align:center; padding:40px 20px; color:var(--text-muted);">
                    <div style="font-size:38px; margin-bottom:12px;">📡</div>
                    <div style="font-size:16px; font-weight:700; color:var(--text-main);">لا توجد أجهزة مطابقة</div>
                    <div style="font-size:13px; margin-top:4px;">اضغط على زر "إضافة جهاز جديد" للبدء في مراقبة السويتشات وروابط الأبراج</div>
                </td>
            </tr>
        `;
        return;
    }

    tbody.innerHTML = filtered.map(d => {
        const statusBadge = getStatusBadge(d.status);
        const typeBadge = getTypeBadge(d.type_slug, d.type_name);
        const vendorBadge = `<span class="badge" style="background:#2d3748; color:#cbd5e0;"><i class="fa-solid fa-microchip"></i> ${escapeHtml(d.vendor_name || 'Vendor')}</span>`;
        
        let extraInfo = '';
        if (d.type_slug === 'switch') {
            extraInfo = `<span style="font-size:11.5px; color:var(--text-muted);"><i class="fa-solid fa-ethernet"></i> ${d.interfaces_count || 0} منفذ</span>`;
        } else if (d.type_slug === 'sector') {
            extraInfo = `<span style="font-size:11.5px; color:#48bb78; font-weight:600;"><i class="fa-solid fa-users"></i> ${d.clients_count || 0} مشترك</span>`;
        } else if (d.type_slug === 'link') {
            const ccq = d.wireless_info ? `${d.wireless_info.ccq}%` : '—';
            extraInfo = `<span style="font-size:11.5px; color:#4299e1; font-weight:600;"><i class="fa-solid fa-tower-cell"></i> CCQ: ${ccq}</span>`;
        }

        const cpuBadge = d.status === 'online' ? `
            <div style="display:flex; align-items:center; gap:6px;">
                <div style="flex:1; background:rgba(255,255,255,0.08); height:6px; border-radius:3px; overflow:hidden; min-width:40px;">
                    <div style="width:${Math.min(d.cpu_load || 0, 100)}%; background:${d.cpu_load > 80 ? 'var(--danger)' : 'var(--primary)'}; height:100%;"></div>
                </div>
                <span style="font-size:11px; font-family:monospace; font-weight:700;">${d.cpu_load || 0}%</span>
            </div>
        ` : '<span style="color:var(--text-muted); font-size:11px;">—</span>';

        const uptimeStr = d.status === 'online' && d.uptime_seconds ? formatDuration(d.uptime_seconds) : '—';

        return `
            <tr style="transition: background 0.15s ease;">
                <td>
                    <div style="display:flex; align-items:center; gap:10px;">
                        <div style="width:36px; height:36px; border-radius:8px; background:rgba(66, 153, 225, 0.12); color:#4299e1; display:flex; align-items:center; justify-content:center; font-size:16px;">
                            <i class="${getDeviceIcon(d.type_slug)}"></i>
                        </div>
                        <div>
                            <div style="font-weight:700; color:var(--text-main); font-size:13.5px; cursor:pointer;" onclick="openDeviceDetailModal(${d.id})">${escapeHtml(d.name)}</div>
                            <div style="font-size:11px; color:var(--text-muted);">${escapeHtml(d.location || d.board_name || 'برج افتراضي')}</div>
                        </div>
                    </div>
                </td>
                <td>
                    <div style="font-family:monospace; font-size:13px; font-weight:700; color:var(--text-main);">${escapeHtml(d.ip)}</div>
                    <div style="font-size:11px; color:var(--text-muted);">بورت: ${d.port || 8728}</div>
                </td>
                <td>
                    <div style="display:flex; flex-direction:column; gap:3px;">
                        <div>${typeBadge}</div>
                        <div>${vendorBadge}</div>
                    </div>
                </td>
                <td>
                    <div style="font-size:12.5px; font-weight:600; color:var(--text-main);">${escapeHtml(d.model_name || d.board_name || '—')}</div>
                    <div style="font-size:11px; color:var(--text-muted); font-family:monospace;">${escapeHtml(d.os_version || 'RouterOS')}</div>
                </td>
                <td>${statusBadge}</td>
                <td>${cpuBadge}</td>
                <td>
                    <div style="font-size:12px;">${uptimeStr}</div>
                    <div>${extraInfo}</div>
                </td>
                <td style="text-align:left;">
                    <div style="display:flex; gap:6px; justify-content:flex-end;">
                        <button class="btn btn-sm" style="background:#2b6cb0; border-color:#2b6cb0; padding:5px 10px; font-size:11.5px;" onclick="openDeviceDetailModal(${d.id})" title="عرض التفاصيل 360°">
                            <i class="fa-solid fa-chart-pie"></i> تفاصيل
                        </button>
                        <button class="btn btn-dark btn-sm" style="padding:5px 8px;" onclick="triggerDevicePoll(${d.id})" title="فحص فوري">
                            <i class="fa-solid fa-rotate"></i>
                        </button>
                        <button class="btn btn-dark btn-sm" style="padding:5px 8px;" onclick="openEditDeviceModal(${d.id})" title="تعديل">
                            <i class="fa-solid fa-pen"></i>
                        </button>
                        <button class="btn btn-danger btn-sm" style="padding:5px 8px;" onclick="deleteDevice(${d.id}, '${escapeHtml(d.name)}')" title="حذف">
                            <i class="fa-solid fa-trash"></i>
                        </button>
                    </div>
                </td>
            </tr>
        `;
    }).join('');
}

function getStatusBadge(status) {
    switch (status) {
        case 'online':
            return '<span class="badge" style="background:#22543d; color:#9ae6b4;"><i class="fa-solid fa-circle-check"></i> متصل</span>';
        case 'offline':
            return '<span class="badge" style="background:#742a2a; color:#feb2b2;"><i class="fa-solid fa-circle-xmark"></i> غير متصل</span>';
        case 'warning':
            return '<span class="badge" style="background:#744210; color:#fbd38d;"><i class="fa-solid fa-triangle-exclamation"></i> تحذير</span>';
        default:
            return '<span class="badge" style="background:#4a5568; color:#cbd5e0;"><i class="fa-solid fa-circle-question"></i> قيد الفحص</span>';
    }
}

function getTypeBadge(typeSlug, typeName) {
    switch (typeSlug) {
        case 'switch':
            return `<span class="badge" style="background:#2c5282; color:#bee3f8;"><i class="fa-solid fa-diagram-project"></i> Switch</span>`;
        case 'link':
            return `<span class="badge" style="background:#553c9a; color:#e9d8fd;"><i class="fa-solid fa-arrows-left-right-to-line"></i> PtP Link</span>`;
        case 'sector':
            return `<span class="badge" style="background:#285e61; color:#b2f5ea;"><i class="fa-solid fa-tower-broadcast"></i> Sector AP</span>`;
        default:
            return `<span class="badge" style="background:#2d3748; color:#e2e8f0;">${escapeHtml(typeName || typeSlug || 'Device')}</span>`;
    }
}

function getDeviceIcon(typeSlug) {
    switch (typeSlug) {
        case 'switch': return 'fa-solid fa-diagram-project';
        case 'link': return 'fa-solid fa-arrows-left-right-to-line';
        case 'sector': return 'fa-solid fa-tower-broadcast';
        default: return 'fa-solid fa-server';
    }
}

function populateVendorAndTypeSelects() {
    const vSelect = document.getElementById('dev-form-vendor');
    const tSelect = document.getElementById('dev-form-type');
    const filterType = document.getElementById('dev-filter-type');
    const filterVendor = document.getElementById('dev-filter-vendor');

    if (vSelect && allVendorsCache.length > 0) {
        vSelect.innerHTML = allVendorsCache.map(v => `<option value="${v.slug}">${escapeHtml(v.name)}</option>`).join('');
    }
    if (tSelect && allTypesCache.length > 0) {
        tSelect.innerHTML = allTypesCache.map(t => `<option value="${t.slug}">${escapeHtml(t.name)}</option>`).join('');
    }
    if (filterVendor && allVendorsCache.length > 0) {
        filterVendor.innerHTML = '<option value="all">جميع الشركات</option>' + allVendorsCache.map(v => `<option value="${v.slug}">${escapeHtml(v.name)}</option>`).join('');
    }
    if (filterType && allTypesCache.length > 0) {
        filterType.innerHTML = '<option value="all">جميع الأنواع</option>' + allTypesCache.map(t => `<option value="${t.slug}">${escapeHtml(t.name)}</option>`).join('');
    }
}

function filterDevicesByType(type) {
    currentDeviceFilter = type;
    fetchDevicesList();
}

function filterDevicesByVendor(vendor) {
    currentVendorFilter = vendor;
    fetchDevicesList();
}

function filterDevicesByStatus(status) {
    currentStatusFilter = status;
    fetchDevicesList();
}

function handleDeviceSearch(val) {
    currentSearchQuery = val.trim();
    renderDevicesList(allDevicesCache);
}

// ─── ADD & DISCOVERY WIZARD ──────────────────────────────────────────────────

function openAddDeviceModal() {
    document.getElementById('device-modal-title').innerText = 'إضافة واكتشاف جهاز شبكة جديد';
    document.getElementById('dev-form-id').value = '';
    document.getElementById('dev-form-name').value = '';
    document.getElementById('dev-form-ip').value = '';
    document.getElementById('dev-form-port').value = '8728';
    document.getElementById('dev-form-user').value = 'admin';
    document.getElementById('dev-form-pass').value = '';
    document.getElementById('dev-form-interval').value = '30';
    document.getElementById('dev-form-location').value = '';
    document.getElementById('dev-form-notes').value = '';
    document.getElementById('dev-form-monitored').checked = true;
    
    // Reset discovery preview box
    const previewBox = document.getElementById('dev-discovery-preview');
    if (previewBox) {
        previewBox.style.display = 'none';
        previewBox.innerHTML = '';
    }

    document.getElementById('device-modal').classList.add('active');
}

function closeDeviceModal() {
    document.getElementById('device-modal').classList.remove('active');
}

async function testDeviceConnection() {
    const vendorSlug = document.getElementById('dev-form-vendor').value;
    const ip = document.getElementById('dev-form-ip').value.trim();
    const port = parseInt(document.getElementById('dev-form-port').value || '8728');
    const username = document.getElementById('dev-form-user').value.trim();
    const password = document.getElementById('dev-form-pass').value;
    const authType = document.getElementById('dev-form-auth-type')?.value || 'api';

    if (!ip) {
        alert('يرجى إدخال عنوان IP الخاص بالجهاز أولاً');
        return;
    }

    const btn = document.getElementById('dev-test-btn');
    const origHtml = btn.innerHTML;
    btn.disabled = true;
    btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> جاري الاتصال...';

    try {
        const res = await apiFetch('/radius/api/devices/test-connection', {
            method: 'POST',
            body: JSON.stringify({ vendor_slug: vendorSlug, ip, port, username, password, auth_type: authType })
        });
        const data = await res.json();
        if (data.success) {
            alert('✅ ' + data.message);
        } else {
            alert('❌ ' + data.message);
        }
    } catch (e) {
        alert('خطأ في الاتصال بالسيرفر');
    } finally {
        btn.disabled = false;
        btn.innerHTML = origHtml;
    }
}

async function discoverDeviceDetails() {
    const vendorSlug = document.getElementById('dev-form-vendor').value;
    const ip = document.getElementById('dev-form-ip').value.trim();
    const port = parseInt(document.getElementById('dev-form-port').value || '8728');
    const username = document.getElementById('dev-form-user').value.trim();
    const password = document.getElementById('dev-form-pass').value;
    const authType = document.getElementById('dev-form-auth-type')?.value || 'api';

    if (!ip) {
        alert('يرجى إدخال عنوان IP الخاص بالجهاز أولاً');
        return;
    }

    const btn = document.getElementById('dev-discover-btn');
    const origHtml = btn.innerHTML;
    btn.disabled = true;
    btn.innerHTML = '<i class="fa-solid fa-radar fa-spin"></i> جاري الفحص والاكتشاف...';

    try {
        const res = await apiFetch('/radius/api/devices/discover', {
            method: 'POST',
            body: JSON.stringify({ vendor_slug: vendorSlug, ip, port, username, password, auth_type: authType })
        });
        const data = await res.json();
        if (data.success && data.discovery) {
            const disc = data.discovery;
            
            // Auto fill fields
            if (!document.getElementById('dev-form-name').value && disc.device_name) {
                document.getElementById('dev-form-name').value = disc.device_name;
            }
            if (disc.suggested_type) {
                document.getElementById('dev-form-type').value = disc.suggested_type;
            }

            // Show discovery box
            const previewBox = document.getElementById('dev-discovery-preview');
            if (previewBox) {
                previewBox.style.display = 'block';
                previewBox.innerHTML = `
                    <div style="background:rgba(40, 167, 69, 0.1); border:1px solid #28a745; border-radius:8px; padding:12px; margin-top:12px;">
                        <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:8px;">
                            <strong style="color:#28a745;"><i class="fa-solid fa-circle-check"></i> تم اكتشاف الجهاز بنجاح!</strong>
                            <span class="badge badge-primary">${disc.suggested_type ? disc.suggested_type.toUpperCase() : 'DEVICE'}</span>
                        </div>
                        <div style="display:grid; grid-template-columns:1fr 1fr; gap:6px; font-size:12px; color:var(--text-main);">
                            <div><strong>الموديل:</strong> ${escapeHtml(disc.model_name || disc.board_name || '—')}</div>
                            <div><strong>النظام:</strong> ${escapeHtml(disc.os_version || '—')}</div>
                            <div><strong>السيريال:</strong> <span style="font-family:monospace;">${escapeHtml(disc.serial_number || '—')}</span></div>
                            <div><strong>المعالج:</strong> ${disc.cpu_load || 0}%</div>
                            <div><strong>المنافذ:</strong> ${disc.interfaces ? disc.interfaces.length : 0} منفذ</div>
                            <div><strong>اللاسلكي:</strong> ${disc.has_wireless ? 'مدعوم ✅' : 'غير متوفر ❌'}</div>
                        </div>
                    </div>
                `;
            }
        } else {
            alert('❌ ' + (data.message || 'فشل اكتشاف تفاصيل الجهاز'));
        }
    } catch (e) {
        alert('خطأ في عملية الاكتشاف');
    } finally {
        btn.disabled = false;
        btn.innerHTML = origHtml;
    }
}

async function handleSaveDevice(e) {
    e.preventDefault();

    const id = document.getElementById('dev-form-id').value;
    const vendorSlug = document.getElementById('dev-form-vendor').value;
    const typeSlug = document.getElementById('dev-form-type').value;
    const name = document.getElementById('dev-form-name').value.trim();
    const ip = document.getElementById('dev-form-ip').value.trim();
    const port = parseInt(document.getElementById('dev-form-port').value || '8728');
    const username = document.getElementById('dev-form-user').value.trim();
    const password = document.getElementById('dev-form-pass').value;
    const authType = document.getElementById('dev-form-auth-type')?.value || 'api';
    const pollIntervalSec = parseInt(document.getElementById('dev-form-interval').value || '30');
    const location = document.getElementById('dev-form-location').value.trim();
    const notes = document.getElementById('dev-form-notes').value.trim();
    const isMonitored = document.getElementById('dev-form-monitored').checked;

    if (!name || !ip) {
        alert('يرجى ملء اسم الجهاز وعنوان IP');
        return;
    }

    try {
        let res;
        if (id) {
            // Update
            res = await apiFetch(`/radius/api/devices/${id}`, {
                method: 'PUT',
                body: JSON.stringify({ name, ip, port, username, password, auth_type: authType, poll_interval_sec: pollIntervalSec, is_monitored: isMonitored, location, notes })
            });
        } else {
            // Create
            res = await apiFetch('/radius/api/devices', {
                method: 'POST',
                body: JSON.stringify({ vendor_slug: vendorSlug, type_slug: typeSlug, name, ip, port, username, password, auth_type: authType, poll_interval_sec: pollIntervalSec, is_monitored: isMonitored, location, notes })
            });
        }

        const data = await res.json();
        if (res.ok && (data.success || data.device)) {
            closeDeviceModal();
            loadDevices();
            alert('✅ تم حفظ الجهاز بنجاح!');
        } else {
            alert('❌ ' + (data.error || data.message || 'فشل حفظ الجهاز'));
        }
    } catch (e) {
        alert('خطأ في إرسال البيانات');
    }
}

async function openEditDeviceModal(id) {
    try {
        const res = await apiFetch(`/radius/api/devices/${id}`);
        if (!res.ok) throw new Error('فشل جلب بيانات الجهاز');
        const detail = await res.json();
        const d = detail.device;

        document.getElementById('device-modal-title').innerText = 'تعديل بيانات الجهاز: ' + d.name;
        document.getElementById('dev-form-id').value = d.id;
        document.getElementById('dev-form-vendor').value = d.vendor_slug;
        document.getElementById('dev-form-type').value = d.type_slug;
        document.getElementById('dev-form-name').value = d.name;
        document.getElementById('dev-form-ip').value = d.ip;
        document.getElementById('dev-form-port').value = d.port;
        document.getElementById('dev-form-interval').value = d.poll_interval_sec || 30;
        document.getElementById('dev-form-location').value = d.location || '';
        document.getElementById('dev-form-notes').value = d.notes || '';
        document.getElementById('dev-form-monitored').checked = d.is_monitored;

        // Leave password empty unless user wants to change it
        document.getElementById('dev-form-pass').value = '';

        const previewBox = document.getElementById('dev-discovery-preview');
        if (previewBox) previewBox.style.display = 'none';

        document.getElementById('device-modal').classList.add('active');
    } catch (e) {
        alert('خطأ في تحميل تفاصيل الجهاز للتعديل');
    }
}

async function deleteDevice(id, name) {
    if (!confirm(`هل أنت متأكد من رغبتك في حذف الجهاز "${name}"؟ سيتم مسح كافة سجلات القياسات والمراقبة التابعة له.`)) {
        return;
    }

    try {
        const res = await apiFetch(`/radius/api/devices/${id}`, { method: 'DELETE' });
        if (res.ok) {
            loadDevices();
        } else {
            const d = await res.json();
            alert(d.error || 'فشل حذف الجهاز');
        }
    } catch (e) {
        alert('خطأ في الاتصال');
    }
}

async function triggerDevicePoll(id) {
    try {
        const res = await apiFetch(`/radius/api/devices/${id}/poll`, { method: 'POST' });
        const data = await res.json();
        if (data.success) {
            loadDevices();
            if (activeDetailDevice && activeDetailDevice.id === id) {
                openDeviceDetailModal(id);
            }
        } else {
            alert('فشل فحص الجهاز: ' + (data.error || 'الجهاز غير قابل للوصول'));
        }
    } catch (e) {
        console.error(e);
    }
}

// ─── 360° DEVICE DETAILS MODAL ───────────────────────────────────────────────

async function openDeviceDetailModal(id) {
    try {
        const res = await apiFetch(`/radius/api/devices/${id}`);
        if (!res.ok) throw new Error('فشل جلب تفاصيل الجهاز');
        const detail = await res.json();
        activeDetailDevice = detail.device;

        renderDevice360Modal(detail);
        document.getElementById('device-details-modal').classList.add('active');
    } catch (e) {
        alert('تعذر فتح تفاصيل الجهاز: ' + e.message);
    }
}

function closeDeviceDetailModal() {
    activeDetailDevice = null;
    document.getElementById('device-details-modal').classList.remove('active');
}

function switchDetailTab(tabName) {
    currentDetailTab = tabName;
    document.querySelectorAll('.dev-detail-tab-btn').forEach(b => b.classList.remove('active'));
    document.querySelectorAll('.dev-detail-tab-pane').forEach(p => p.classList.remove('active'));

    const btn = document.getElementById(`dev-tab-btn-${tabName}`);
    const pane = document.getElementById(`dev-tab-pane-${tabName}`);
    if (btn) btn.classList.add('active');
    if (pane) pane.classList.add('active');
}

function renderDevice360Modal(detail) {
    const d = detail.device;
    const ifaces = detail.interfaces || [];
    const wireless = detail.wireless;
    const clients = detail.clients || [];
    const events = detail.recent_events || [];

    document.getElementById('dev-modal-header-title').textContent = d.name;
    document.getElementById('dev-modal-header-meta').innerHTML = `
        <span><i class="fa-solid fa-network-wired"></i> ${escapeHtml(d.ip)}</span>
        <span>•</span>
        <span>${escapeHtml(d.vendor_name)} ${escapeHtml(d.model_name || d.board_name || '')}</span>
        <span>•</span>
        <span>${getStatusBadge(d.status)}</span>
    `;

    // Overview Tab Content
    const overviewContainer = document.getElementById('dev-tab-pane-overview');
    if (overviewContainer) {
        overviewContainer.innerHTML = buildOverviewHTML(d, ifaces, wireless, clients);
    }

    // Type-Tailored Secondary Tabs
    const tabBtnPorts = document.getElementById('dev-tab-btn-ports');
    const tabBtnWireless = document.getElementById('dev-tab-btn-wireless');
    const tabBtnClients = document.getElementById('dev-tab-btn-clients');

    if (d.type_slug === 'switch') {
        if (tabBtnPorts) tabBtnPorts.style.display = 'inline-block';
        if (tabBtnWireless) tabBtnWireless.style.display = 'none';
        if (tabBtnClients) tabBtnClients.style.display = 'none';
        renderPortsTab(ifaces);
    } else if (d.type_slug === 'link') {
        if (tabBtnPorts) tabBtnPorts.style.display = 'inline-block';
        if (tabBtnWireless) tabBtnWireless.style.display = 'inline-block';
        if (tabBtnClients) tabBtnClients.style.display = 'none';
        renderWirelessLinkTab(wireless, ifaces);
        renderPortsTab(ifaces);
    } else if (d.type_slug === 'sector') {
        if (tabBtnPorts) tabBtnPorts.style.display = 'none';
        if (tabBtnWireless) tabBtnWireless.style.display = 'inline-block';
        if (tabBtnClients) tabBtnClients.style.display = 'inline-block';
        renderWirelessSectorTab(wireless);
        renderClientsTab(clients);
    }

    // Events Tab
    renderEventsTab(events);

    // Switch to default overview
    switchDetailTab('overview');
}

function buildOverviewHTML(d, ifaces, wireless, clients) {
    const memPercent = d.memory_total > 0 ? Math.round((d.memory_used / d.memory_total) * 100) : 0;
    const storagePercent = d.storage_total > 0 ? Math.round((d.storage_used / d.storage_total) * 100) : 0;

    let typeHero = '';
    if (d.type_slug === 'switch') {
        typeHero = `
            <div style="background:var(--bg-app); border:1px solid var(--border); border-radius:10px; padding:16px; margin-bottom:16px;">
                <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:12px;">
                    <h4 style="margin:0; font-size:14px;"><i class="fa-solid fa-ethernet" style="color:var(--primary);"></i> لوحة المنافذ السريعة (Port Status)</h4>
                    <span style="font-size:12px; color:var(--text-muted);">${ifaces.length} منفذ معرف</span>
                </div>
                <div style="display:grid; grid-template-columns:repeat(auto-fill, minmax(65px, 1fr)); gap:6px;">
                    ${ifaces.map(i => {
                        const isUp = i.status === 'up';
                        const isSfp = i.is_sfp;
                        const isPoe = i.is_poe;
                        return `
                            <div style="border:1px solid ${isUp ? '#28a745' : 'var(--border)'}; background:${isUp ? 'rgba(40,167,69,0.12)' : 'rgba(0,0,0,0.2)'}; border-radius:6px; padding:6px; text-align:center;">
                                <div style="font-size:10px; font-weight:700; color:${isUp ? '#28a745' : 'var(--text-muted)'}; white-space:nowrap; overflow:hidden; text-overflow:ellipsis;">${escapeHtml(i.name)}</div>
                                <div style="font-size:13px; margin:2px 0;">${isSfp ? '🔌' : (isPoe ? '⚡' : '🌐')}</div>
                                <div style="font-size:9px; color:var(--text-muted);">${i.speed || (isUp ? 'UP' : 'DOWN')}</div>
                            </div>
                        `;
                    }).join('')}
                </div>
            </div>
        `;
    } else if (d.type_slug === 'link' && wireless) {
        typeHero = `
            <div style="background:linear-gradient(135deg, rgba(85,60,154,0.2), rgba(66,153,225,0.1)); border:1px solid rgba(85,60,154,0.4); border-radius:10px; padding:16px; margin-bottom:16px;">
                <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:14px;">
                    <h4 style="margin:0; font-size:15px; color:#b794f4;"><i class="fa-solid fa-arrows-left-right-to-line"></i> حالة الربط اللاسلكي الحية (Live Wireless Link)</h4>
                    <span class="badge" style="background:#553c9a; color:#fff;">${wireless.mode || 'PTP'}</span>
                </div>
                <div style="display:grid; grid-template-columns:repeat(auto-fit, minmax(130px, 1fr)); gap:12px; text-align:center;">
                    <div style="background:rgba(0,0,0,0.25); padding:10px; border-radius:8px;">
                        <div style="font-size:11px; color:var(--text-muted);">قوة الإشارة (Signal)</div>
                        <div style="font-size:18px; font-weight:800; color:${wireless.signal_strength > -65 ? '#48bb78' : '#ecc94b'}; font-family:monospace;">${wireless.signal_strength || 0} dBm</div>
                    </div>
                    <div style="background:rgba(0,0,0,0.25); padding:10px; border-radius:8px;">
                        <div style="font-size:11px; color:var(--text-muted);">جودة البث (CCQ)</div>
                        <div style="font-size:18px; font-weight:800; color:${wireless.ccq >= 90 ? '#48bb78' : '#f56565'}; font-family:monospace;">${wireless.ccq || 0}%</div>
                    </div>
                    <div style="background:rgba(0,0,0,0.25); padding:10px; border-radius:8px;">
                        <div style="font-size:11px; color:var(--text-muted);">التردد / العرض</div>
                        <div style="font-size:14px; font-weight:700; color:var(--text-main); font-family:monospace;">${wireless.frequency || 0} MHz <span style="font-size:11px;">(${wireless.channel_width || '40M'})</span></div>
                    </div>
                    <div style="background:rgba(0,0,0,0.25); padding:10px; border-radius:8px;">
                        <div style="font-size:11px; color:var(--text-muted);">السرعات (TX / RX)</div>
                        <div style="font-size:12px; font-weight:700; color:#4299e1; font-family:monospace;">⬆️ ${wireless.tx_rate || '—'}<br>⬇️ ${wireless.rx_rate || '—'}</div>
                    </div>
                </div>
            </div>
        `;
    } else if (d.type_slug === 'sector' && wireless) {
        typeHero = `
            <div style="background:linear-gradient(135deg, rgba(40,94,97,0.2), rgba(72,187,120,0.1)); border:1px solid rgba(40,94,97,0.4); border-radius:10px; padding:16px; margin-bottom:16px;">
                <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:14px;">
                    <h4 style="margin:0; font-size:15px; color:#81e6d9;"><i class="fa-solid fa-tower-broadcast"></i> راديو السكتور والمشتركين المتصلين</h4>
                    <span class="badge" style="background:#234e52; color:#b2f5ea;"><i class="fa-solid fa-users"></i> ${clients.length} مشترك متصل</span>
                </div>
                <div style="display:grid; grid-template-columns:repeat(auto-fit, minmax(130px, 1fr)); gap:12px; text-align:center;">
                    <div style="background:rgba(0,0,0,0.25); padding:10px; border-radius:8px;">
                        <div style="font-size:11px; color:var(--text-muted);">التردد الحقيقي</div>
                        <div style="font-size:16px; font-weight:800; color:#38b2ac; font-family:monospace;">${wireless.frequency || 0} MHz</div>
                    </div>
                    <div style="background:rgba(0,0,0,0.25); padding:10px; border-radius:8px;">
                        <div style="font-size:11px; color:var(--text-muted);">عرض القناة</div>
                        <div style="font-size:15px; font-weight:700; color:var(--text-main); font-family:monospace;">${wireless.channel_width || '—'}</div>
                    </div>
                    <div style="background:rgba(0,0,0,0.25); padding:10px; border-radius:8px;">
                        <div style="font-size:11px; color:var(--text-muted);">الضوضاء (Noise)</div>
                        <div style="font-size:15px; font-weight:700; color:#cbd5e0; font-family:monospace;">${wireless.noise_floor || 0} dBm</div>
                    </div>
                    <div style="background:rgba(0,0,0,0.25); padding:10px; border-radius:8px;">
                        <div style="font-size:11px; color:var(--text-muted);">قوة البث (TX Power)</div>
                        <div style="font-size:15px; font-weight:700; color:#f6ad55; font-family:monospace;">${wireless.tx_power || 0} dBm</div>
                    </div>
                </div>
            </div>
        `;
    }

    return `
        ${typeHero}

        <!-- Hardware & Resource Cards -->
        <div style="display:grid; grid-template-columns:repeat(auto-fit, minmax(220px, 1fr)); gap:14px; margin-bottom:16px;">
            <div style="background:var(--bg-app); border:1px solid var(--border); border-radius:10px; padding:14px;">
                <h5 style="margin:0 0 10px 0; font-size:13px; color:var(--text-muted);"><i class="fa-solid fa-microchip"></i> استهلاك المعالج (CPU)</h5>
                <div style="display:flex; justify-content:space-between; align-items:baseline; margin-bottom:6px;">
                    <span style="font-size:24px; font-weight:800; font-family:monospace; color:${d.cpu_load > 80 ? 'var(--danger)' : 'var(--primary)'};">${d.cpu_load || 0}%</span>
                    <span style="font-size:11px; color:var(--text-muted);">${d.architecture || 'ARM'}</span>
                </div>
                <div style="background:rgba(255,255,255,0.08); height:6px; border-radius:3px; overflow:hidden;">
                    <div style="width:${Math.min(d.cpu_load || 0, 100)}%; background:${d.cpu_load > 80 ? 'var(--danger)' : 'var(--primary)'}; height:100%;"></div>
                </div>
            </div>

            <div style="background:var(--bg-app); border:1px solid var(--border); border-radius:10px; padding:14px;">
                <h5 style="margin:0 0 10px 0; font-size:13px; color:var(--text-muted);"><i class="fa-solid fa-memory"></i> استهلاك الذاكرة (RAM)</h5>
                <div style="display:flex; justify-content:space-between; align-items:baseline; margin-bottom:6px;">
                    <span style="font-size:24px; font-weight:800; font-family:monospace; color:var(--text-main);">${memPercent}%</span>
                    <span style="font-size:11px; color:var(--text-muted);">${formatBytes(d.memory_used)} / ${formatBytes(d.memory_total)}</span>
                </div>
                <div style="background:rgba(255,255,255,0.08); height:6px; border-radius:3px; overflow:hidden;">
                    <div style="width:${memPercent}%; background:#38b2ac; height:100%;"></div>
                </div>
            </div>

            <div style="background:var(--bg-app); border:1px solid var(--border); border-radius:10px; padding:14px;">
                <h5 style="margin:0 0 10px 0; font-size:13px; color:var(--text-muted);"><i class="fa-solid fa-hard-drive"></i> وحدة التخزين (Disk)</h5>
                <div style="display:flex; justify-content:space-between; align-items:baseline; margin-bottom:6px;">
                    <span style="font-size:24px; font-weight:800; font-family:monospace; color:var(--text-main);">${storagePercent}%</span>
                    <span style="font-size:11px; color:var(--text-muted);">${formatBytes(d.storage_used)} / ${formatBytes(d.storage_total)}</span>
                </div>
                <div style="background:rgba(255,255,255,0.08); height:6px; border-radius:3px; overflow:hidden;">
                    <div style="width:${storagePercent}%; background:#ed8936; height:100%;"></div>
                </div>
            </div>

            <div style="background:var(--bg-app); border:1px solid var(--border); border-radius:10px; padding:14px;">
                <h5 style="margin:0 0 10px 0; font-size:13px; color:var(--text-muted);"><i class="fa-solid fa-temperature-half"></i> الصحة والحرارة (Health)</h5>
                <div style="display:flex; justify-content:space-between; align-items:baseline; margin-bottom:4px;">
                    <span style="font-size:20px; font-weight:800; font-family:monospace; color:${d.temperature > 65 ? 'var(--danger)' : '#48bb78'};">${d.temperature ? d.temperature + ' °C' : '—'}</span>
                    <span style="font-size:13px; font-family:monospace; color:var(--text-muted);">${d.voltage ? d.voltage + ' V' : ''}</span>
                </div>
                <div style="font-size:11px; color:var(--text-muted); margin-top:6px;">وقت التشغيل: ${formatDuration(d.uptime_seconds)}</div>
            </div>
        </div>

        <!-- Identity & System Table -->
        <div style="background:var(--bg-app); border:1px solid var(--border); border-radius:10px; padding:16px;">
            <h4 style="margin:0 0 12px 0; font-size:14px;"><i class="fa-solid fa-circle-info" style="color:var(--primary);"></i> معلومات النظام والجهاز</h4>
            <table style="width:100%; font-size:13px; border-collapse:collapse;">
                <tr><td style="padding:6px; color:var(--text-muted); width:35%;">اسم الجهاز:</td><td style="font-weight:700;">${escapeHtml(d.name)}</td></tr>
                <tr><td style="padding:6px; color:var(--text-muted);">الشركة المصنعة (Vendor):</td><td>${escapeHtml(d.vendor_name)}</td></tr>
                <tr><td style="padding:6px; color:var(--text-muted);">الموديل (Board Model):</td><td style="font-weight:600;">${escapeHtml(d.model_name || d.board_name || '—')}</td></tr>
                <tr><td style="padding:6px; color:var(--text-muted);">إصدار النظام (OS Version):</td><td style="font-family:monospace;">${escapeHtml(d.os_version || '—')}</td></tr>
                <tr><td style="padding:6px; color:var(--text-muted);">الرقم التسلسلي (Serial Number):</td><td style="font-family:monospace; font-weight:700; color:var(--primary);">${escapeHtml(d.serial_number || '—')}</td></tr>
                <tr><td style="padding:6px; color:var(--text-muted);">الموقع / البرج:</td><td>${escapeHtml(d.location || 'غير محدد')}</td></tr>
                <tr><td style="padding:6px; color:var(--text-muted);">ملاحظات:</td><td>${escapeHtml(d.notes || '—')}</td></tr>
            </table>
        </div>
    `;
}

function renderPortsTab(ifaces) {
    const pane = document.getElementById('dev-tab-pane-ports');
    if (!pane) return;

    if (!ifaces || ifaces.length === 0) {
        pane.innerHTML = '<div style="text-align:center; padding:30px; color:var(--text-muted);">لا توجد منافذ مسجلة لهذا الجهاز</div>';
        return;
    }

    pane.innerHTML = `
        <div style="overflow-x:auto;">
            <table class="table" style="width:100%; font-size:12.5px;">
                <thead>
                    <tr>
                        <th>المنفذ</th>
                        <th>النوع</th>
                        <th>الحالة</th>
                        <th>السرعة</th>
                        <th>Duplex</th>
                        <th>الماك (MAC)</th>
                        <th>PoE / SFP</th>
                        <th>الترافيك (⬇️ RX / ⬆️ TX)</th>
                    </tr>
                </thead>
                <tbody>
                    ${ifaces.map(i => {
                        const isUp = i.status === 'up';
                        const statusBadge = isUp ? '<span class="badge" style="background:#22543d; color:#9ae6b4;">UP</span>' : '<span class="badge" style="background:#4a5568; color:#cbd5e0;">DOWN</span>';
                        let extraBadge = '—';
                        if (i.is_sfp) {
                            extraBadge = `<span class="badge" style="background:#2c5282; color:#bee3f8;">SFP ${i.sfp_temp ? i.sfp_temp + '°C' : ''}</span>`;
                        } else if (i.is_poe) {
                            extraBadge = `<span class="badge" style="background:#744210; color:#fbd38d;">⚡ PoE ${i.poe_power_watt ? i.poe_power_watt + 'W' : ''}</span>`;
                        }

                        return `
                            <tr>
                                <td><strong>${escapeHtml(i.name)}</strong></td>
                                <td><span style="font-family:monospace; color:var(--text-muted);">${escapeHtml(i.type)}</span></td>
                                <td>${statusBadge}</td>
                                <td><span style="font-family:monospace;">${escapeHtml(i.speed || '—')}</span></td>
                                <td>${escapeHtml(i.duplex || '—')}</td>
                                <td><span style="font-family:monospace; font-size:11px;">${escapeHtml(i.mac_address || '—')}</span></td>
                                <td>${extraBadge}</td>
                                <td style="font-family:monospace; font-size:11.5px;">
                                    ⬇️ ${formatBytes(i.rx_bytes)}<br>
                                    ⬆️ ${formatBytes(i.tx_bytes)}
                                </td>
                            </tr>
                        `;
                    }).join('')}
                </tbody>
            </table>
        </div>
    `;
}

function renderWirelessLinkTab(wireless, ifaces) {
    const pane = document.getElementById('dev-tab-pane-wireless');
    if (!pane) return;

    if (!wireless) {
        pane.innerHTML = '<div style="text-align:center; padding:30px; color:var(--text-muted);">لا تتوفر بيانات راديو لاسلكي لهذا الجهاز</div>';
        return;
    }

    pane.innerHTML = `
        <div style="background:var(--bg-app); border:1px solid var(--border); border-radius:10px; padding:16px; margin-bottom:16px;">
            <h4 style="margin:0 0 14px 0; font-size:15px; color:#b794f4;"><i class="fa-solid fa-satellite-dish"></i> تفاصيل الرابط اللاسلكي (Point-to-Point Details)</h4>
            <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px; font-size:13px;">
                <div><strong>اسم الراديو / Interface:</strong> ${escapeHtml(wireless.interface_name)}</div>
                <div><strong>النمط (Mode):</strong> ${escapeHtml(wireless.mode)}</div>
                <div><strong>اسم الشبكة (SSID):</strong> <span style="font-weight:700; color:var(--primary);">${escapeHtml(wireless.ssid)}</span></div>
                <div><strong>التردد (Frequency):</strong> ${wireless.frequency} MHz</div>
                <div><strong>عرض القناة (Channel Width):</strong> ${wireless.channel_width}</div>
                <div><strong>قوة الإشارة (Signal Strength):</strong> <span style="font-weight:800; font-family:monospace; color:#48bb78;">${wireless.signal_strength} dBm</span></div>
                <div><strong>الضوضاء (Noise Floor):</strong> ${wireless.noise_floor} dBm</div>
                <div><strong>نسبة الإشارة للضوضاء (SNR):</strong> ${wireless.snr} dB</div>
                <div><strong>جودة الربط (Overall CCQ):</strong> <span style="font-weight:800; font-family:monospace; color:#4299e1;">${wireless.ccq}%</span></div>
                <div><strong>المسافة التقديرية (Distance):</strong> ${wireless.distance_km ? wireless.distance_km + ' km' : '—'}</div>
                <div><strong>معدل الإرسال (TX Rate):</strong> ${wireless.tx_rate || '—'}</div>
                <div><strong>معدل الاستقبال (RX Rate):</strong> ${wireless.rx_rate || '—'}</div>
                <div><strong>ماك الطرف البعيد (Remote MAC):</strong> <span style="font-family:monospace;">${escapeHtml(wireless.remote_mac || '—')}</span></div>
                <div><strong>جهاز الطرف البعيد:</strong> ${escapeHtml(wireless.remote_device_info || '—')}</div>
            </div>
        </div>
    `;
}

function renderWirelessSectorTab(wireless) {
    const pane = document.getElementById('dev-tab-pane-wireless');
    if (!pane) return;

    if (!wireless) {
        pane.innerHTML = '<div style="text-align:center; padding:30px; color:var(--text-muted);">لا تتوفر بيانات سكتور لاسلكي</div>';
        return;
    }

    pane.innerHTML = `
        <div style="background:var(--bg-app); border:1px solid var(--border); border-radius:10px; padding:16px;">
            <h4 style="margin:0 0 14px 0; font-size:15px; color:#81e6d9;"><i class="fa-solid fa-tower-broadcast"></i> إعدادات راديو السكتور (Sector Radio)</h4>
            <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px; font-size:13px;">
                <div><strong>المنفذ اللاسلكي:</strong> ${escapeHtml(wireless.interface_name)}</div>
                <div><strong>النمط:</strong> ${escapeHtml(wireless.mode)}</div>
                <div><strong>اسم السكتور (SSID):</strong> <span style="font-weight:700; color:#38b2ac;">${escapeHtml(wireless.ssid)}</span></div>
                <div><strong>التردد:</strong> ${wireless.frequency} MHz</div>
                <div><strong>عرض القناة:</strong> ${wireless.channel_width}</div>
                <div><strong>قوة البث (TX Power):</strong> ${wireless.tx_power} dBm</div>
                <div><strong>الضوضاء (Noise Floor):</strong> ${wireless.noise_floor} dBm</div>
                <div><strong>المشتركون المتصلون:</strong> <span class="badge badge-success">${wireless.connected_clients || 0} مشترك</span></div>
            </div>
        </div>
    `;
}

function renderClientsTab(clients) {
    const pane = document.getElementById('dev-tab-pane-clients');
    if (!pane) return;

    if (!clients || clients.length === 0) {
        pane.innerHTML = '<div style="text-align:center; padding:30px; color:var(--text-muted);">لا يوجد مشتركون متصلون بالسكتور حالياً</div>';
        return;
    }

    pane.innerHTML = `
        <div style="overflow-x:auto;">
            <table class="table" style="width:100%; font-size:12.5px;">
                <thead>
                    <tr>
                        <th>عنوان الماك (MAC)</th>
                        <th>عنوان IP</th>
                        <th>اسم الجهاز / العميل</th>
                        <th>الإشارة (Signal)</th>
                        <th>CCQ</th>
                        <th>السرعات (TX / RX)</th>
                        <th>مدة الاتصال</th>
                        <th>الاستهلاك</th>
                    </tr>
                </thead>
                <tbody>
                    ${clients.map(c => {
                        const sigColor = c.signal > -65 ? '#48bb78' : (c.signal > -75 ? '#ecc94b' : '#f56565');
                        return `
                            <tr>
                                <td><span style="font-family:monospace; font-weight:700;">${escapeHtml(c.mac_address)}</span></td>
                                <td><span style="font-family:monospace; color:var(--primary);">${escapeHtml(c.ip_address || '—')}</span></td>
                                <td><strong>${escapeHtml(c.hostname || 'Station Client')}</strong></td>
                                <td><span class="badge" style="background:rgba(0,0,0,0.3); color:${sigColor}; font-family:monospace; font-weight:700;">${c.signal} dBm</span></td>
                                <td><span style="font-family:monospace; font-weight:700; color:#4299e1;">${c.ccq}%</span></td>
                                <td style="font-family:monospace; font-size:11.5px;">
                                    ⬆️ ${c.tx_rate || '—'}<br>
                                    ⬇️ ${c.rx_rate || '—'}
                                </td>
                                <td>${formatDuration(c.uptime_seconds)}</td>
                                <td style="font-family:monospace; font-size:11.5px;">
                                    ⬇️ ${formatBytes(c.rx_bytes)}<br>
                                    ⬆️ ${formatBytes(c.tx_bytes)}
                                </td>
                            </tr>
                        `;
                    }).join('')}
                </tbody>
            </table>
        </div>
    `;
}

function renderEventsTab(events) {
    const pane = document.getElementById('dev-tab-pane-events');
    if (!pane) return;

    if (!events || events.length === 0) {
        pane.innerHTML = '<div style="text-align:center; padding:30px; color:var(--text-muted);">لا توجد أحداث مسجلة حديثاً</div>';
        return;
    }

    pane.innerHTML = `
        <div style="display:flex; flex-direction:column; gap:8px;">
            ${events.map(e => {
                let badgeClass = 'badge-secondary';
                let icon = 'fa-info-circle';
                if (e.severity === 'critical') { badgeClass = 'badge-danger'; icon = 'fa-circle-exclamation'; }
                else if (e.severity === 'warning') { badgeClass = 'badge-warning'; icon = 'fa-triangle-exclamation'; }
                else if (e.event_type === 'online') { badgeClass = 'badge-success'; icon = 'fa-circle-check'; }

                return `
                    <div style="background:var(--bg-app); border:1px solid var(--border); border-radius:8px; padding:10px 14px; display:flex; justify-content:space-between; align-items:center;">
                        <div style="display:flex; align-items:center; gap:10px;">
                            <span class="badge ${badgeClass}"><i class="fa-solid ${icon}"></i> ${e.event_type.toUpperCase()}</span>
                            <span style="font-size:13px; color:var(--text-main);">${escapeHtml(e.message)}</span>
                        </div>
                        <span style="font-size:11px; color:var(--text-muted); font-family:monospace;">${new Date(e.created_at).toLocaleTimeString('ar-IQ')}</span>
                    </div>
                `;
            }).join('')}
        </div>
    `;
}

// Global expose
window.loadDevices = loadDevices;
window.filterDevicesByType = filterDevicesByType;
window.filterDevicesByVendor = filterDevicesByVendor;
window.filterDevicesByStatus = filterDevicesByStatus;
window.handleDeviceSearch = handleDeviceSearch;
window.openAddDeviceModal = openAddDeviceModal;
window.closeDeviceModal = closeDeviceModal;
window.testDeviceConnection = testDeviceConnection;
window.discoverDeviceDetails = discoverDeviceDetails;
window.handleSaveDevice = handleSaveDevice;
window.openEditDeviceModal = openEditDeviceModal;
window.deleteDevice = deleteDevice;
window.triggerDevicePoll = triggerDevicePoll;
window.openDeviceDetailModal = openDeviceDetailModal;
window.closeDeviceDetailModal = closeDeviceDetailModal;
window.switchDetailTab = switchDetailTab;
