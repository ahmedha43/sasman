<?php
$totalDevs = count($devices);
$onlineDevs = 0;
foreach ($devices as $d) {
    if (($d['status'] ?? '') === 'online') {
        $onlineDevs++;
    }
}
$offlineDevs = $totalDevs - $onlineDevs;
?>

<div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
    <div>
        <h4 class="fw-bold text-light mb-1"><i class="fa-solid fa-satellite-dish me-2 text-primary"></i> فحص أجهزة المشتركين والصحونات (CPE Devices)</h4>
        <p class="text-muted mb-0 fs-7">مراقبة إشارات الأبراج والصحونات (Ubiquiti, MikroTik, Mimosa, Cambium) والتشخيص السحابي</p>
    </div>
    <button class="btn btn-primary rounded-pill px-4 fw-bold shadow-sm" data-bs-toggle="modal" data-bs-target="#addDeviceModal">
        <i class="fa-solid fa-plus me-2"></i> إضافة جهاز / صحن جديد
    </button>
</div>

<!-- KPI Cards -->
<div class="row g-3 mb-4">
    <div class="col-12 col-md-4">
        <div class="card glass-card border-primary border-opacity-50 shadow-sm rounded-4 p-3 h-100">
            <div class="d-flex align-items-center justify-content-between">
                <div>
                    <span class="text-muted fs-7 fw-semibold d-block mb-1">إجمالي الأجهزة المضافة</span>
                    <h3 class="fw-bold text-light mb-0"><?= $totalDevs ?> جهاز</h3>
                </div>
                <div class="stat-icon bg-primary-subtle text-primary rounded-3 p-3">
                    <i class="fa-solid fa-tower-broadcast fs-3"></i>
                </div>
            </div>
        </div>
    </div>

    <div class="col-12 col-md-4">
        <div class="card glass-card border-success border-opacity-50 shadow-sm rounded-4 p-3 h-100">
            <div class="d-flex align-items-center justify-content-between">
                <div>
                    <span class="text-muted fs-7 fw-semibold d-block mb-1">الأجهزة المتصلة (Online)</span>
                    <h3 class="fw-bold text-success mb-0"><?= $onlineDevs ?> جهاز</h3>
                </div>
                <div class="stat-icon bg-success-subtle text-success rounded-3 p-3">
                    <i class="fa-solid fa-signal fs-3"></i>
                </div>
            </div>
        </div>
    </div>

    <div class="col-12 col-md-4">
        <div class="card glass-card border-0 shadow-sm rounded-4 p-3 h-100">
            <div class="d-flex align-items-center justify-content-between">
                <div>
                    <span class="text-muted fs-7 fw-semibold d-block mb-1">الأجهزة غير المتاحة (Offline)</span>
                    <h3 class="fw-bold text-danger mb-0"><?= $offlineDevs ?> جهاز</h3>
                </div>
                <div class="stat-icon bg-danger-subtle text-danger rounded-3 p-3">
                    <i class="fa-solid fa-triangle-exclamation fs-3"></i>
                </div>
            </div>
        </div>
    </div>
</div>

<!-- Devices Table -->
<div class="card glass-card border-0 shadow-sm rounded-4 p-4">
    <div class="table-responsive">
        <table class="table table-dark table-hover align-middle mb-0">
            <thead class="table-secondary">
                <tr>
                    <th>#</th>
                    <th>اسم الجهاز</th>
                    <th>الشركة المصنعة (Vendor)</th>
                    <th>عنوان IP</th>
                    <th>قوة الإشارة (Signal)</th>
                    <th>حالة الاتصال</th>
                    <th>زمن الاستجابة</th>
                    <th class="text-center">العمليات</th>
                </tr>
            </thead>
            <tbody>
                <?php if (empty($devices)): ?>
                <tr>
                    <td colspan="8" class="text-center py-5 text-muted">
                        <i class="fa-solid fa-satellite-dish fs-1 d-block mb-2 text-secondary"></i>
                        لا توجد أجهزة مضافة أو مكتشفة حالياً
                    </td>
                </tr>
                <?php else: ?>
                <?php foreach ($devices as $i => $d): ?>
                <?php
                    $dId = $d['id'] ?? $i;
                    $dName = $d['name'] ?? "Device-{$i}";
                    $vendor = $d['vendor'] ?? $d['vendor_slug'] ?? 'Ubiquiti';
                    $ip = $d['ip'] ?? '192.168.1.20';
                    $signal = $d['signal'] ?? $d['signal_dbm'] ?? null;
                    $status = $d['status'] ?? 'online';
                    $latency = $d['latency_ms'] ?? 2;
                ?>
                <tr>
                    <td class="text-muted fs-8"><?= $i + 1 ?></td>
                    <td class="fw-bold text-light">
                        <i class="fa-solid fa-tower-cell me-1 text-info"></i> <?= htmlspecialchars($dName) ?>
                    </td>
                    <td><span class="badge bg-secondary"><?= htmlspecialchars(strtoupper($vendor)) ?></span></td>
                    <td class="font-monospace text-info"><?= htmlspecialchars($ip) ?></td>
                    <td>
                        <?php if ($signal !== null): ?>
                        <?php 
                            $sigVal = (int)$signal;
                            $sigClass = ($sigVal >= -65) ? 'bg-success text-white' : (($sigVal >= -75) ? 'bg-warning text-dark' : 'bg-danger text-white');
                        ?>
                        <span class="badge <?= $sigClass ?> px-2 py-1"><?= htmlspecialchars($signal) ?> dBm</span>
                        <?php else: ?>
                        <span class="text-muted fs-8">—</span>
                        <?php endif; ?>
                    </td>
                    <td>
                        <?php if ($status === 'online'): ?>
                        <span class="badge bg-success-subtle text-success border border-success px-2 py-1"><i class="fa-solid fa-circle-check me-1"></i> متصل</span>
                        <?php else: ?>
                        <span class="badge bg-danger-subtle text-danger border border-danger px-2 py-1"><i class="fa-solid fa-circle-xmark me-1"></i> غير متاح</span>
                        <?php endif; ?>
                    </td>
                    <td class="fs-8 text-muted"><?= htmlspecialchars($latency) ?> ms</td>
                    <td class="text-center">
                        <div class="btn-group btn-group-sm">
                            <form action="/devices/<?= urlencode($dId) ?>/poll" method="POST" class="d-inline">
                                <button type="submit" class="btn btn-outline-warning" title="فحص وقراءة البيانات لحظياً"><i class="fa-solid fa-rotate"></i></button>
                            </form>
                            <a href="http://<?= htmlspecialchars($ip) ?>" target="_blank" class="btn btn-outline-info" title="فتح واجهة الويب للجهاز">
                                <i class="fa-solid fa-arrow-up-right-from-square"></i>
                            </a>
                            <a href="/devices/<?= urlencode($dId) ?>/delete" class="btn btn-outline-danger" onclick="return confirm('حذف هذا الجهاز؟');" title="حذف">
                                <i class="fa-solid fa-trash"></i>
                            </a>
                        </div>
                    </td>
                </tr>
                <?php endforeach; ?>
                <?php endif; ?>
            </tbody>
        </table>
    </div>
</div>

<!-- Add Device Modal -->
<div class="modal fade" id="addDeviceModal" tabindex="-1" aria-labelledby="addDeviceModalLabel" aria-hidden="true">
    <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content glass-card border-secondary text-light">
            <div class="modal-header border-secondary">
                <h5 class="modal-title fw-bold" id="addDeviceModalLabel"><i class="fa-solid fa-plus me-2 text-primary"></i> إضافة جهاز / صحن شبكة</h5>
                <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" aria-label="Close"></button>
            </div>
            <form action="/devices" method="POST">
                <div class="modal-body">
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">اسم الجهاز (Device Name) *</label>
                        <input type="text" name="name" class="form-control bg-dark border-secondary text-light" required placeholder="LiteBeam-Sector-1">
                    </div>
                    <div class="row g-2 mb-3">
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">الشركة المصنعة</label>
                            <select name="vendor_slug" class="form-select bg-dark border-secondary text-light">
                                <option value="ubiquiti" selected>Ubiquiti (AirOS)</option>
                                <option value="mikrotik">MikroTik RouterOS</option>
                                <option value="mimosa">Mimosa</option>
                                <option value="cambium">Cambium Networks</option>
                            </select>
                        </div>
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">نوع الجهاز</label>
                            <select name="type_slug" class="form-select bg-dark border-secondary text-light">
                                <option value="cpe" selected>صحن مشترك (CPE / Station)</option>
                                <option value="ap">برج رئيسي (Access Point)</option>
                                <option value="ptp">ربط نقطي (Point-to-Point)</option>
                            </select>
                        </div>
                    </div>
                    <div class="row g-2 mb-3">
                        <div class="col-8">
                            <label class="form-label fs-7 fw-semibold">عنوان IP *</label>
                            <input type="text" name="ip" class="form-control bg-dark border-secondary text-light font-monospace" required placeholder="192.168.1.20">
                        </div>
                        <div class="col-4">
                            <label class="form-label fs-7 fw-semibold">المنفذ (Port)</label>
                            <input type="number" name="port" class="form-control bg-dark border-secondary text-light" value="80">
                        </div>
                    </div>
                    <div class="row g-2 mb-3">
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">اسم المستخدم (Login)</label>
                            <input type="text" name="username" class="form-control bg-dark border-secondary text-light" value="ubnt">
                        </div>
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">كلمة المرور</label>
                            <input type="text" name="password" class="form-control bg-dark border-secondary text-light" value="ubnt">
                        </div>
                    </div>
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">الوصف أو اسم المشترك / الموقع</label>
                        <input type="text" name="description" class="form-control bg-dark border-secondary text-light" placeholder="صحن المشترك أحمد - برج الشروق">
                    </div>
                </div>
                <div class="modal-footer border-secondary">
                    <button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal">إلغاء</button>
                    <button type="submit" class="btn btn-primary px-4 fw-bold">حفظ الجهاز</button>
                </div>
            </form>
        </div>
    </div>
</div>
