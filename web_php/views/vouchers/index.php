<!-- Header & Actions Toolbar -->
<div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-3 no-print">
    <div>
        <h4 class="fw-bold text-light mb-1"><i class="fa-solid fa-ticket me-2 text-warning"></i> أستوديو كروت الشحن والفئات (Vouchers Studio)</h4>
        <p class="text-muted mb-0 fs-7">توليد وإدارة وطباعة كروت الشحن على ورق A4 (شبكة كروت مصفوفة للقص) أو طابعات الإيصالات الحرارية (POS 80mm)</p>
    </div>
    <div class="d-flex flex-wrap gap-2">
        <?php if (!empty($perms['can_print_vouchers'])): ?>
        <button class="btn btn-primary rounded-pill px-4 fw-bold shadow" onclick="printAllVouchersA4()">
            <i class="fa-solid fa-print me-2"></i> طباعة صفحة A4 كاملة (شبكة كروت)
        </button>
        <?php endif; ?>
        <?php if (!empty($perms['can_generate_vouchers'])): ?>
        <button class="btn btn-warning rounded-pill px-4 fw-bold shadow text-dark" data-bs-toggle="modal" data-bs-target="#generateModal">
            <i class="fa-solid fa-wand-magic-sparkles me-2"></i> توليد كروت جديدة
        </button>
        <?php endif; ?>
        <?php if (!empty($perms['can_delete_vouchers'])): ?>
        <a href="/vouchers/clear" onclick="return confirm('هل أنت متأكد تماماً من مسح جميع الكروت غير المستخدمة؟');" class="btn btn-outline-danger rounded-pill px-3 fw-bold">
            <i class="fa-solid fa-trash-can me-1"></i> مسح غير المستخدم
        </a>
        <?php endif; ?>
    </div>
</div>

<!-- Pro KPI Summary Chips Bar -->
<div class="row g-2 mb-4 no-print">
    <div class="col-6 col-md-3">
        <div class="p-3 rounded-4 border border-warning border-opacity-50 text-center h-100 voucher-kpi-chip" style="background: rgba(241, 196, 15, 0.15);" onclick="setVoucherFilter('all')">
            <span class="text-warning fs-8 d-block"><i class="fa-solid fa-ticket me-1"></i> إجمالي الكروت</span>
            <span class="fs-4 fw-bold font-monospace text-light"><?= number_format($total_vouchers ?? count($vouchers)) ?></span>
        </div>
    </div>
    <div class="col-6 col-md-3">
        <div class="p-3 rounded-4 border border-success border-opacity-50 text-center h-100 voucher-kpi-chip" style="background: rgba(33, 140, 116, 0.15);" onclick="setVoucherFilter('unused')">
            <span class="text-success fs-8 d-block"><i class="fa-solid fa-circle-check me-1"></i> كروت جاهزة للشحن</span>
            <span class="fs-4 fw-bold font-monospace text-success"><?= number_format($unused_count ?? 0) ?></span>
        </div>
    </div>
    <div class="col-6 col-md-3">
        <div class="p-3 rounded-4 border border-secondary border-opacity-50 text-center h-100 voucher-kpi-chip" style="background: rgba(100, 116, 139, 0.15);" onclick="setVoucherFilter('used')">
            <span class="text-white-50 fs-8 d-block"><i class="fa-solid fa-lock me-1 text-secondary"></i> كروت مستخدمة</span>
            <span class="fs-4 fw-bold font-monospace text-white-50"><?= number_format($used_count ?? 0) ?></span>
        </div>
    </div>
    <div class="col-6 col-md-3">
        <div class="p-3 rounded-4 border border-primary border-opacity-50 text-center h-100 voucher-kpi-chip" style="background: rgba(63, 81, 181, 0.15);">
            <span class="text-info fs-8 d-block"><i class="fa-solid fa-coins me-1 text-primary"></i> القيمة الإجمالية للكروت المتاحة</span>
            <span class="fs-5 fw-bold font-monospace text-light mt-1 d-block"><?= number_format($total_worth ?? 0, 0) ?> <span class="fs-8">د.ع</span></span>
        </div>
    </div>
</div>

<!-- Interactive Search & Filter Toolbar -->
<div class="card glass-card border-0 shadow-sm rounded-4 p-3 mb-4 no-print">
    <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <!-- Quick Filter Pills -->
        <div class="d-flex flex-wrap gap-1" id="voucherFilterPills">
            <button class="btn btn-sm btn-outline-light rounded-pill px-3 fw-bold active-voucher-filter" onclick="setVoucherFilter('all', this)">
                <i class="fa-solid fa-list me-1"></i> كل الكروت (<?= count($vouchers) ?>)
            </button>
            <button class="btn btn-sm btn-outline-success rounded-pill px-3 fw-bold" onclick="setVoucherFilter('unused', this)">
                <i class="fa-solid fa-circle-check me-1"></i> جاهزة للشحن (<?= $unused_count ?? 0 ?>)
            </button>
            <button class="btn btn-sm btn-outline-secondary rounded-pill px-3 fw-bold" onclick="setVoucherFilter('used', this)">
                <i class="fa-solid fa-lock me-1"></i> مستخدمة (<?= $used_count ?? 0 ?>)
            </button>
        </div>

        <!-- Search Box -->
        <div class="position-relative" style="min-width: 280px; width: 100%;">
            <span class="position-absolute top-50 start-0 translate-middle-y ms-3 text-muted">
                <i class="fa-solid fa-magnifying-glass"></i>
            </span>
            <input type="text" id="voucherSearchInput" class="form-control form-control-sm bg-dark border-secondary text-light rounded-pill ps-5 pe-3 py-2" placeholder="ابحث برمز الكرت (PIN) أو الباقة..." onkeyup="filterVouchersTable()">
        </div>
    </div>
</div>

<!-- Pro Vouchers Table Card -->
<div class="card glass-card border-0 shadow-lg rounded-4 p-0 overflow-hidden no-print">
    <div class="table-responsive">
        <table class="table table-dark table-hover align-middle mb-0 vouchers-table-pro" id="vouchersTable">
            <thead>
                <tr>
                    <th class="ps-4">#</th>
                    <th>🔑 رمز الكرت (PIN Code)</th>
                    <th>📦 الباقة</th>
                    <th>⏱️ الصلاحية</th>
                    <th>💰 السعر</th>
                    <th>📌 الحالة</th>
                    <th class="text-center">🖨️ معاينة وطباعة</th>
                    <th class="text-center pe-4">⚙️ الإجراء</th>
                </tr>
            </thead>
            <tbody>
                <?php if (empty($vouchers)): ?>
                <tr>
                    <td colspan="8" class="text-center py-5 text-muted">
                        <i class="fa-solid fa-ticket-simple fs-1 d-block mb-3 text-secondary opacity-50"></i>
                        <h5>لا توجد كروت شحن مولدة حالياً</h5>
                        <p class="fs-8 text-muted">اضغط على زر "توليد كروت جديدة" لإنشاء دفعة كروت جاهزة للطباعة والبيع</p>
                    </td>
                </tr>
                <?php else: ?>
                <?php foreach ($vouchers as $i => $v): ?>
                <?php
                    $vCode = $v['code'] ?? $v['username'] ?? '';
                    $vProfile = $v['profile_name'] ?? $v['profile'] ?? 'Standard';
                    $vDays = $v['validity_days'] ?? 30;
                    $vPrice = (float)($v['price'] ?? 0);
                    $isUsed = !empty($v['is_used']) || (!empty($v['status']) && $v['status'] !== 'active');
                    $vId = $v['id'] ?? $vCode;
                    $usedBy = $v['used_by'] ?? '';
                ?>
                <tr class="voucher-row" 
                    data-code="<?= htmlspecialchars(strtolower($vCode)) ?>" 
                    data-profile="<?= htmlspecialchars(strtolower($vProfile)) ?>" 
                    data-status="<?= $isUsed ? 'used' : 'unused' ?>">
                    
                    <!-- Index -->
                    <td class="ps-4 text-muted fs-8 font-monospace"><?= $i + 1 ?></td>
                    
                    <!-- PIN Code -->
                    <td>
                        <div class="d-flex align-items-center gap-2">
                            <div class="voucher-pin-chip">
                                <i class="fa-solid fa-key text-warning me-2"></i>
                                <span class="font-monospace fw-bold text-light fs-6"><?= htmlspecialchars($vCode) ?></span>
                            </div>
                            <button class="btn btn-sm btn-link text-muted p-1 copy-pin-btn" onclick="copyPin('<?= htmlspecialchars($vCode, ENT_QUOTES) ?>', this)" title="نسخ رمز PIN">
                                <i class="fa-solid fa-copy fs-8"></i>
                            </button>
                        </div>
                    </td>

                    <!-- Profile -->
                    <td>
                        <span class="badge bg-primary-subtle text-primary border border-primary px-3 py-1 font-monospace fs-8">
                            <i class="fa-solid fa-gauge-high me-1"></i><?= htmlspecialchars($vProfile) ?>
                        </span>
                    </td>

                    <!-- Validity -->
                    <td class="fs-8 text-light font-monospace">
                        <i class="fa-solid fa-calendar-days text-warning me-1"></i> <?= htmlspecialchars($vDays) ?> يوماً
                    </td>

                    <!-- Price -->
                    <td>
                        <strong class="text-success font-monospace fs-7"><?= number_format($vPrice, 0) ?> د.ع</strong>
                    </td>

                    <!-- Status -->
                    <td>
                        <?php if (!$isUsed): ?>
                        <span class="badge bg-success-subtle text-success border border-success px-3 py-1 fs-9">
                            <i class="fa-solid fa-circle-check me-1"></i> جاهز للشحن
                        </span>
                        <?php else: ?>
                        <span class="badge bg-secondary-subtle text-secondary border border-secondary px-3 py-1 fs-9">
                            <i class="fa-solid fa-lock me-1"></i> مستخدم <?= $usedBy ? "(@{$usedBy})" : '' ?>
                        </span>
                        <?php endif; ?>
                    </td>

                    <!-- Print Actions -->
                    <td class="text-center">
                        <div class="btn-group btn-group-sm">
                            <button class="btn btn-outline-warning btn-sm rounded-pill px-3 me-1" onclick="openThermalReceipt('<?= htmlspecialchars($vCode, ENT_QUOTES) ?>', '<?= htmlspecialchars($vProfile, ENT_QUOTES) ?>', '<?= number_format($vPrice, 0) ?>', '<?= htmlspecialchars($vDays) ?>')" title="طباعة وصل حراري POS">
                                <i class="fa-solid fa-receipt me-1"></i> وصل حراري (POS)
                            </button>
                            <button class="btn btn-outline-info btn-sm rounded-pill px-3" onclick='printSingleVoucherA4(<?= json_encode($v, JSON_HEX_TAG | JSON_HEX_APOS | JSON_HEX_QUOT | JSON_HEX_AMP) ?>)' title="طباعة كرت A4 منفرد">
                                <i class="fa-solid fa-print me-1"></i> كرت A4
                            </button>
                        </div>
                    </td>

                    <!-- Delete Action -->
                    <td class="text-center pe-4">
                        <a href="/vouchers/<?= urlencode($vId) ?>/delete" class="btn btn-sm btn-outline-danger rounded-circle p-2" onclick="return confirm('هل أنت متأكد من حذف هذا الكرت؟');" title="حذف">
                            <i class="fa-solid fa-trash"></i>
                        </a>
                    </td>
                </tr>
                <?php endforeach; ?>
                <?php endif; ?>
            </tbody>
        </table>
    </div>
</div>

<!-- ============================================================
   1. THERMAL RECEIPT MODAL (POS 80mm)
============================================================ -->
<div class="modal fade" id="thermalModal" tabindex="-1" aria-hidden="true">
    <div class="modal-dialog modal-dialog-centered" style="max-width: 360px;">
        <div class="modal-content glass-card border-warning text-light">
            <div class="modal-header border-secondary no-print">
                <h6 class="modal-title fw-bold text-warning"><i class="fa-solid fa-receipt me-2"></i> معاينة الوصل الحراري (POS 80mm)</h6>
                <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" aria-label="Close"></button>
            </div>
            <div class="modal-body p-4" id="receiptPrintArea">
                <div class="bg-white text-dark p-4 rounded-3 text-center border shadow-sm font-monospace" style="border: 2px dashed #334155 !important;">
                    <div class="text-center mb-2">
                        <div class="bg-primary text-white p-2 rounded-3 d-inline-block mb-1 shadow-sm">
                            <i class="fa-solid fa-server fs-3"></i>
                        </div>
                        <h4 class="fw-bold mb-0 text-dark">SASMAN NETWORK</h4>
                        <small class="text-muted fs-9 d-block">كارت شحن وتجديد اشتراك فوري</small>
                    </div>
                    
                    <hr class="my-2 border-dark">
                    
                    <div class="d-flex justify-content-between fs-8 mb-1">
                        <span class="text-muted">الباقة المخصصة:</span>
                        <strong class="text-dark" id="recProfile">—</strong>
                    </div>
                    <div class="d-flex justify-content-between fs-8 mb-1">
                        <span class="text-muted">مدة الصلاحية:</span>
                        <strong class="text-dark" id="recDays">30 يوم</strong>
                    </div>
                    <div class="d-flex justify-content-between fs-8 mb-2">
                        <span class="text-muted">السعر المطلوب:</span>
                        <strong class="text-success fs-7" id="recPrice">— د.ع</strong>
                    </div>
                    
                    <div class="my-3 p-3 bg-light border border-2 border-dark rounded-3">
                        <span class="fs-9 text-muted d-block mb-1 fw-bold text-uppercase">رمز التفعيل (PIN Code)</span>
                        <div class="fs-2 fw-bold text-dark font-monospace" id="recCode" style="letter-spacing: 3px;">0000-0000</div>
                    </div>

                    <div class="p-2 bg-white d-inline-block border rounded mb-2">
                        <img id="recQR" src="" alt="QR Code" class="img-fluid" style="max-width: 140px;">
                    </div>
                    
                    <hr class="my-2 border-dark">
                    <p class="fs-9 text-muted mb-0 fw-bold">امسح كود QR أو ادخل الرمز في بوابة المشترك 🌐</p>
                </div>
            </div>
            <div class="modal-footer border-secondary no-print">
                <button type="button" class="btn btn-secondary btn-sm rounded-pill px-3" data-bs-dismiss="modal">إلغاء</button>
                <button type="button" class="btn btn-warning btn-sm px-4 fw-bold rounded-pill text-dark" onclick="printReceipt()"><i class="fa-solid fa-print me-1"></i> طباعة الوصل الآن</button>
            </div>
        </div>
    </div>
</div>

<!-- ============================================================
   2. GENERATE VOUCHERS MODAL
============================================================ -->
<div class="modal fade" id="generateModal" tabindex="-1" aria-labelledby="generateModalLabel" aria-hidden="true">
    <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content glass-card border-warning text-light">
            <div class="modal-header border-secondary">
                <h5 class="modal-title fw-bold text-warning" id="generateModalLabel"><i class="fa-solid fa-wand-magic-sparkles me-2"></i> توليد دفعة كروت شحن</h5>
                <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" aria-label="Close"></button>
            </div>
            <form action="/vouchers/generate" method="POST">
                <div class="modal-body">
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">عدد الكروت المطلوبة *</label>
                        <input type="number" name="count" class="form-control bg-dark border-secondary text-light font-monospace" required value="10" min="1" max="1000">
                    </div>
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">باقة الكرت *</label>
                        <select name="profile_name" class="form-select bg-dark border-secondary text-light" required>
                            <option value="">-- اختر الباقة --</option>
                            <?php foreach ($profiles as $p): ?>
                            <option value="<?= htmlspecialchars($p['name']) ?>"><?= htmlspecialchars($p['name']) ?> (<?= number_format($p['price'] ?? 0, 0) ?> د.ع)</option>
                            <?php endforeach; ?>
                        </select>
                    </div>
                    <div class="row g-2 mb-3">
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">نوع الرمز</label>
                            <select name="code_type" class="form-select bg-dark border-secondary text-light">
                                <option value="numeric">أرقام فقط (مثال: 84920482)</option>
                                <option value="alphanumeric">أرقام وحروف (مثال: S7K9M2P)</option>
                            </select>
                        </div>
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">طول الرمز</label>
                            <input type="number" name="code_length" class="form-control bg-dark border-secondary text-light font-monospace" value="8" min="6" max="16">
                        </div>
                    </div>
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">سعر الكرت (د.ع - اختياري للتجاوز)</label>
                        <input type="number" name="price" class="form-control bg-dark border-secondary text-light font-monospace" placeholder="اتركه فارغاً لاستخدام سعر الباقة">
                    </div>
                </div>
                <div class="modal-footer border-secondary">
                    <button type="button" class="btn btn-outline-secondary rounded-pill" data-bs-dismiss="modal">إلغاء</button>
                    <button type="submit" class="btn btn-warning px-4 fw-bold rounded-pill text-dark">
                        <i class="fa-solid fa-wand-magic-sparkles me-1"></i> توليد الكروت الآن
                    </button>
                </div>
            </form>
        </div>
    </div>
</div>

<!-- ============================================================
   3. A4 FULL SHEET PRINT TEMPLATE (Hidden from normal view)
============================================================ -->
<div id="a4PrintContainer" class="d-none">
    <!-- Populated by JavaScript with high-res vouchers grid -->
</div>

<style>
/* ============================================================
   PRO VOUCHERS STUDIO STYLING
============================================================ */
.voucher-kpi-chip {
    transition: transform 0.2s ease, box-shadow 0.2s ease;
    cursor: pointer;
}
.voucher-kpi-chip:hover {
    transform: translateY(-3px);
    box-shadow: 0 6px 16px rgba(0, 0, 0, 0.25);
}

.active-voucher-filter {
    background-color: #f59e0b !important;
    color: #0f172a !important;
    border-color: #f59e0b !important;
    box-shadow: 0 0 12px rgba(245, 158, 11, 0.5);
}

.vouchers-table-pro thead th {
    background: linear-gradient(180deg, #1e293b 0%, #0f172a 100%);
    color: #cbd5e1;
    font-weight: 700;
    padding: 14px 12px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.1);
    white-space: nowrap;
}
.vouchers-table-pro tbody tr {
    transition: background 0.2s ease;
    border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}
.vouchers-table-pro tbody tr:hover {
    background: rgba(255, 255, 255, 0.04) !important;
}

/* PIN Chip */
.voucher-pin-chip {
    background: rgba(0, 0, 0, 0.5);
    border: 1px solid rgba(245, 158, 11, 0.4);
    padding: 6px 14px;
    border-radius: 10px;
    display: inline-flex;
    align-items: center;
    box-shadow: 0 2px 8px rgba(245, 158, 11, 0.15);
}

/* Print CSS */
@media print {
    body * {
        visibility: hidden !important;
    }
    .no-print {
        display: none !important;
    }
    #a4PrintContainer, #a4PrintContainer * {
        visibility: visible !important;
    }
    #a4PrintContainer {
        display: block !important;
        position: absolute;
        left: 0;
        top: 0;
        width: 100%;
        background: #ffffff !important;
        color: #000000 !important;
        padding: 10mm;
    }
    
    .a4-voucher-grid {
        display: grid;
        grid-template-columns: repeat(2, 1fr);
        gap: 8mm;
    }
    
    .a4-voucher-card {
        border: 2px dashed #475569 !important;
        border-radius: 10px;
        padding: 12px 16px;
        background: #ffffff !important;
        color: #0f172a !important;
        display: flex;
        align-items: center;
        justify-content: space-between;
        page-break-inside: avoid;
    }

    #receiptPrintArea, #receiptPrintArea * {
        visibility: visible !important;
    }
    #receiptPrintArea {
        position: absolute;
        left: 0;
        top: 0;
        width: 80mm;
    }
}
</style>

<script>
let currentVoucherFilter = 'all';

function setVoucherFilter(filter, btn) {
    currentVoucherFilter = filter;
    document.querySelectorAll('#voucherFilterPills button').forEach(b => b.classList.remove('active-voucher-filter'));
    if (btn) btn.classList.add('active-voucher-filter');
    filterVouchersTable();
}

function filterVouchersTable() {
    const q = (document.getElementById('voucherSearchInput').value || '').toLowerCase().trim();
    const rows = document.querySelectorAll('.voucher-row');
    
    rows.forEach(row => {
        const code = row.getAttribute('data-code') || '';
        const profile = row.getAttribute('data-profile') || '';
        const status = row.getAttribute('data-status') || '';
        
        let matchFilter = true;
        if (currentVoucherFilter === 'unused') matchFilter = (status === 'unused');
        if (currentVoucherFilter === 'used') matchFilter = (status === 'used');
        
        let matchSearch = true;
        if (q) matchSearch = code.includes(q) || profile.includes(q);
        
        if (matchFilter && matchSearch) {
            row.style.display = '';
        } else {
            row.style.display = 'none';
        }
    });
}

function copyPin(pin, btn) {
    navigator.clipboard.writeText(pin);
    const icon = btn.querySelector('i');
    if (icon) {
        icon.className = 'fa-solid fa-check text-success fs-8';
        setTimeout(() => {
            icon.className = 'fa-solid fa-copy fs-8';
        }, 1500);
    }
}

function openThermalReceipt(code, profile, price, days) {
    document.getElementById('recCode').innerText = code;
    document.getElementById('recProfile').innerText = profile;
    document.getElementById('recPrice').innerText = price + ' د.ع';
    document.getElementById('recDays').innerText = days + ' يوماً';
    document.getElementById('recQR').src = 'https://api.qrserver.com/v1/create-qr-code/?size=140x140&data=' + encodeURIComponent(window.location.origin + '/portal?voucher=' + code);
    
    new bootstrap.Modal(document.getElementById('thermalModal')).show();
}

function printReceipt() {
    window.print();
}

function printSingleVoucherA4(v) {
    const code = v.code || v.username || '';
    const profile = v.profile_name || v.profile || '';
    const price = Number(v.price || 0).toLocaleString();
    const days = v.validity_days || 30;
    const qrUrl = 'https://api.qrserver.com/v1/create-qr-code/?size=100x100&data=' + encodeURIComponent(window.location.origin + '/portal?voucher=' + code);

    const container = document.getElementById('a4PrintContainer');
    container.innerHTML = `
        <div style="max-width: 400px; margin: 40px auto;">
            <div class="a4-voucher-card">
                <div style="flex: 1; padding-left: 15px;">
                    <h5 style="margin: 0 0 4px; font-weight: 800; color: #1e293b;">SASMAN NETWORK</h5>
                    <div style="font-size: 11px; color: #64748b; margin-bottom: 8px;">باقة: <strong>${profile}</strong> | الصلاحية: <strong>${days} يوم</strong></div>
                    <div style="background: #f8fafc; border: 1px solid #cbd5e1; border-radius: 8px; padding: 6px 12px; margin-bottom: 6px;">
                        <span style="font-size: 10px; color: #64748b; display: block;">رمز التفعيل PIN</span>
                        <strong style="font-size: 20px; font-family: monospace; letter-spacing: 2px; color: #0f172a;">${code}</strong>
                    </div>
                    <div style="font-size: 12px; font-weight: bold; color: #16a34a;">السعر: ${price} د.ع</div>
                </div>
                <div>
                    <img src="${qrUrl}" style="width: 90px; height: 90px; border-radius: 6px; border: 1px solid #cbd5e1; padding: 3px;" alt="QR">
                </div>
            </div>
        </div>
    `;
    window.print();
}

function printAllVouchersA4() {
    const rows = document.querySelectorAll('.voucher-row');
    let cardsHtml = '';
    
    rows.forEach(row => {
        if (row.style.display !== 'none') {
            const code = row.getAttribute('data-code') || '';
            const profile = row.getAttribute('data-profile') || '';
            const priceEl = row.querySelector('.text-success');
            const price = priceEl ? priceEl.innerText : '—';
            const qrUrl = 'https://api.qrserver.com/v1/create-qr-code/?size=90x90&data=' + encodeURIComponent(window.location.origin + '/portal?voucher=' + code);
            
            cardsHtml += `
                <div class="a4-voucher-card">
                    <div style="flex: 1; padding-left: 12px;">
                        <h6 style="margin: 0 0 2px; font-weight: 800; color: #0f172a;">SASMAN NETWORK</h6>
                        <div style="font-size: 10px; color: #64748b; margin-bottom: 6px;">باقة: <strong>${profile}</strong></div>
                        <div style="background: #f8fafc; border: 1px solid #cbd5e1; border-radius: 6px; padding: 4px 8px; margin-bottom: 4px;">
                            <span style="font-size: 9px; color: #64748b; display: block;">رمز PIN</span>
                            <strong style="font-size: 16px; font-family: monospace; letter-spacing: 1.5px; color: #0f172a;">${code.toUpperCase()}</strong>
                        </div>
                        <div style="font-size: 11px; font-weight: bold; color: #16a34a;">${price}</div>
                    </div>
                    <div>
                        <img src="${qrUrl}" style="width: 75px; height: 75px; border-radius: 4px; border: 1px solid #cbd5e1; padding: 2px;" alt="QR">
                    </div>
                </div>
            `;
        }
    });

    const container = document.getElementById('a4PrintContainer');
    container.innerHTML = `
        <div style="text-align: center; margin-bottom: 15px; border-bottom: 2px solid #0f172a; padding-bottom: 10px;">
            <h4 style="margin: 0; font-weight: 900; color: #0f172a;">شبكة SASMAN — كروت شحن راديوس والمايكروتك</h4>
            <div style="font-size: 11px; color: #64748b;">تاريخ الطباعة: ${new Date().toLocaleString()}</div>
        </div>
        <div class="a4-voucher-grid">
            ${cardsHtml}
        </div>
    `;
    window.print();
}
</script>
