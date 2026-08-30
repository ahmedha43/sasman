<div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
    <div>
        <h4 class="fw-bold text-light mb-1"><i class="fa-solid fa-clock-rotate-left me-2 text-info"></i> سجل النشاطات والرقابة (Audit Trail)</h4>
        <p class="text-muted mb-0 fs-7">توثيق كافة الإجراءات والعمليات التي تمت على النظام مع تفاصيل المنفذ وعنوان IP</p>
    </div>
    <div class="d-flex gap-2">
        <form action="/logs/clear" method="POST" class="d-inline">
            <button type="submit" class="btn btn-outline-danger rounded-pill px-3" onclick="return confirm('مسح سجل النشاطات القديمة؟');">
                <i class="fa-solid fa-trash-can me-1"></i> مسح السجلات
            </button>
        </form>
    </div>
</div>

<!-- Search & Filter Card -->
<div class="card glass-card border-0 shadow-sm rounded-4 p-3 mb-4">
    <form action="/logs" method="GET" class="row g-2 align-items-center">
        <div class="col-12 col-md-5">
            <input type="text" name="search" class="form-control form-control-sm bg-dark border-secondary text-light rounded-pill px-3" placeholder="بحث بالاسم أو الهدف أو التفاصيل..." value="<?= htmlspecialchars($search ?? '') ?>">
        </div>
        <div class="col-12 col-md-4">
            <select name="action_type" class="form-select form-select-sm bg-dark border-secondary text-light rounded-pill px-3">
                <option value="">-- جميع العمليات --</option>
                <option value="تجديد مشترك" <?= ($action_type ?? '') === 'تجديد مشترك' ? 'selected' : '' ?>>تجديد مشترك</option>
                <option value="إضافة مشترك" <?= ($action_type ?? '') === 'إضافة مشترك' ? 'selected' : '' ?>>إضافة مشترك</option>
                <option value="تعديل مشترك" <?= ($action_type ?? '') === 'تعديل مشترك' ? 'selected' : '' ?>>تعديل مشترك</option>
                <option value="حذف مشترك" <?= ($action_type ?? '') === 'حذف مشترك' ? 'selected' : '' ?>>حذف مشترك</option>
                <option value="سجل مالي" <?= ($action_type ?? '') === 'سجل مالي' ? 'selected' : '' ?>>سجل مالي</option>
                <option value="إنشاء حساب وكيل" <?= ($action_type ?? '') === 'إنشاء حساب وكيل' ? 'selected' : '' ?>>إنشاء حساب وكيل</option>
            </select>
        </div>
        <div class="col-12 col-md-3 d-flex gap-2">
            <button type="submit" class="btn btn-sm btn-primary rounded-pill px-4 fw-bold w-100">
                <i class="fa-solid fa-filter me-1"></i> تصفية
            </button>
            <a href="/logs" class="btn btn-sm btn-outline-secondary rounded-pill px-3">إلغاء</a>
        </div>
    </form>
</div>

<!-- Logs Table Card -->
<div class="card glass-card border-0 shadow-sm rounded-4 p-4">
    <div class="table-responsive">
        <table class="table table-dark table-hover align-middle mb-0">
            <thead class="table-secondary">
                <tr>
                    <th>#</th>
                    <th>المنفذ (Admin)</th>
                    <th>نوع العملية</th>
                    <th>الهدف / المشترك</th>
                    <th>التفاصيل والملاحظات</th>
                    <th>عنوان IP</th>
                    <th>التاريخ والوقت</th>
                </tr>
            </thead>
            <tbody>
                <?php if (empty($logs)): ?>
                <tr>
                    <td colspan="7" class="text-center py-5 text-muted">
                        <i class="fa-solid fa-receipt fs-1 d-block mb-2 text-secondary"></i>
                        لا توجد نشاطات مسجلة مطابقة للبحث
                    </td>
                </tr>
                <?php else: ?>
                <?php foreach ($logs as $i => $l): ?>
                <?php
                    $adminName = $l['admin_username'] ?? 'admin';
                    $actType = $l['action_type'] ?? 'عملية';
                    $target = $l['target'] ?? '—';
                    $details = $l['details'] ?? '—';
                    $ip = $l['ip_address'] ?? '127.0.0.1';
                    $created = $l['created_at'] ?? '—';
                ?>
                <tr>
                    <td class="text-muted fs-8"><?= $i + 1 ?></td>
                    <td class="fw-bold text-info font-monospace fs-7">
                        <span class="badge bg-secondary font-monospace">@<?= htmlspecialchars($adminName) ?></span>
                    </td>
                    <td>
                        <span class="badge bg-primary-subtle text-primary border border-primary px-2 py-1"><?= htmlspecialchars($actType) ?></span>
                    </td>
                    <td class="fw-bold text-light font-monospace fs-7"><?= htmlspecialchars($target) ?></td>
                    <td class="fs-8 text-light"><?= htmlspecialchars($details) ?></td>
                    <td class="font-monospace fs-8 text-muted"><?= htmlspecialchars($ip) ?></td>
                    <td class="fs-8 text-muted"><?= htmlspecialchars($created) ?></td>
                </tr>
                <?php endforeach; ?>
                <?php endif; ?>
            </tbody>
        </table>
    </div>
</div>
