<?php
$isConnected = !empty($config['connected']) || (!empty($qr['status']) && $qr['status'] === 'connected');
$connectedPhone = $config['phone_number'] ?? '';

$defaultTemplates = [
    'renew_paid' => "تم تجديد اشتراكك بنجاح ✅\nالمستخدم: {username}\nالباقة: {profile}\nالسعر: {price} د.ع\nالمدة: {validity_days} يوم\nحالة الدفع: مدفوع",
    'renew_debt' => "تم تجديد اشتراكك ⏳\nالمستخدم: {username}\nالباقة: {profile}\nالسعر: {price} د.ع\nالمدة: {validity_days} يوم\nملاحظة: تمت إضافة المبلغ كديون\nرصيدك الحالي: {balance} د.ع",
    'add_debt' => "تم إضافة ديون 📋\nالمستخدم: {username}\nالمبلغ: {amount} د.ع\nالملاحظات: {notes}\nرصيدك الحالي: {balance} د.ع",
    'payment' => "تم تسديد ديون ✅\nالمستخدم: {username}\nالمبلغ: {amount} د.ع\nالملاحظات: {notes}\nرصيدك الحالي: {balance} د.ع",
    'expiry_reminder' => "تنبيه انتهاء الاشتراك ⚠️\nعزيزي {full_name}، نود إعلامك أن اشتراكك في باقة {profile} سينتهي قريباً.\nتاريخ الانتهاء: {expiry_date}\nيرجى التجديد لضمان استمرار الخدمة.",
    'debt_reminder' => "تذكير بالديون المستحقة 📋\nعزيزي {full_name}، نود تذكيرك بأن لديك ديوناً مستحقة بمبلغ {balance} د.ع.\nيرجى التواصل مع الوكيل لتسوية الحساب في أقرب وقت ممكن.\nشكراً لتعاملكم معنا 🙏",
];

foreach ($defaultTemplates as $k => $v) {
    if (empty($templates[$k])) {
        $templates[$k] = $v;
    }
}
?>

<div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
    <div>
        <h4 class="fw-bold text-light mb-1"><i class="fa-brands fa-whatsapp me-2 text-success"></i> أستوديو رسائل وقوالب الواتساب (WhatsApp Studio)</h4>
        <p class="text-muted mb-0 fs-7">ربط بوت الواتساب، تخصيص قوالب الإشعارات، وإرسال التنبيهات التلقائية وفواتير الديون</p>
    </div>
    <div class="d-flex gap-2">
        <form action="/whatsapp/send-debt-reminder" method="POST" class="d-inline">
            <button type="submit" class="btn btn-outline-danger rounded-pill px-3 fw-bold" onclick="return confirm('إرسال رسائل تذكير الديون لجميع المشتركين الذين عليهم ديون؟');">
                <i class="fa-solid fa-bullhorn me-1"></i> إرسال تذكير الديون للجميع
            </button>
        </form>
        <button class="btn btn-success rounded-pill px-4 fw-bold shadow-sm" data-bs-toggle="modal" data-bs-target="#testModal">
            <i class="fa-solid fa-paper-plane me-2"></i> إرسال رسالة تجريبية
        </button>
    </div>
</div>

<div class="row g-4 mb-4">
    <!-- WhatsApp Connection & QR Card -->
    <div class="col-12 col-lg-5">
        <div class="card glass-card border-0 shadow-sm rounded-4 p-4 h-100 text-center">
            <div class="d-flex justify-content-between align-items-center mb-3">
                <h5 class="fw-bold text-light mb-0"><i class="fa-brands fa-whatsapp me-2 text-success"></i> حالة ربط البوت</h5>
                <?php if ($isConnected): ?>
                <span class="badge bg-success-subtle text-success border border-success px-3 py-1"><i class="fa-solid fa-circle-check me-1"></i> متصل بنجاح</span>
                <?php else: ?>
                <span class="badge bg-warning-subtle text-warning border border-warning px-3 py-1"><i class="fa-solid fa-qrcode me-1"></i> بانتظار المسح</span>
                <?php endif; ?>
            </div>

            <p class="text-muted fs-8 mb-3">امسح الرمز التالي من خلال تطبيق واتساب (الأجهزة المرتبطة) لربط المنظومة:</p>
            
            <div class="qr-container bg-white p-3 rounded-4 d-inline-block mx-auto mb-3 shadow" style="max-width: 230px;">
                <?php if (!empty($qr['qr_image'])): ?>
                <img src="<?= htmlspecialchars($qr['qr_image']) ?>" alt="WhatsApp QR" class="img-fluid rounded" style="max-height: 200px;">
                <?php elseif (!empty($qr['qr'])): ?>
                <img src="https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=<?= urlencode($qr['qr']) ?>" alt="WhatsApp QR" class="img-fluid rounded" style="max-height: 200px;">
                <?php elseif ($isConnected): ?>
                <div class="py-4 text-dark">
                    <i class="fa-solid fa-circle-check text-success fs-1 d-block mb-2"></i>
                    <span class="fs-7 fw-bold text-success">الواتساب متصل ويعمل بنشاط ✓</span>
                    <?php if ($connectedPhone): ?>
                    <small class="d-block text-muted mt-1 font-monospace"><?= htmlspecialchars($connectedPhone) ?></small>
                    <?php endif; ?>
                </div>
                <?php else: ?>
                <div class="py-4 text-dark">
                    <i class="fa-solid fa-spinner fa-spin text-primary fs-1 d-block mb-2"></i>
                    <span class="fs-8 text-muted">جاري توليد كود QR...</span>
                </div>
                <?php endif; ?>
            </div>

            <div class="d-flex justify-content-center gap-2">
                <a href="/whatsapp" class="btn btn-outline-light btn-sm rounded-pill px-3">
                    <i class="fa-solid fa-rotate me-1"></i> تحديث الرمز
                </a>
                <a href="/whatsapp/logout" onclick="return confirm('فصل جلسة الواتساب الحالية؟');" class="btn btn-outline-danger btn-sm rounded-pill px-3">
                    <i class="fa-solid fa-power-off me-1"></i> تسجيل الخروج
                </a>
            </div>
        </div>
    </div>

    <!-- WhatsApp Settings Form Card -->
    <div class="col-12 col-lg-7">
        <div class="card glass-card border-0 shadow-sm rounded-4 p-4 h-100">
            <h5 class="fw-bold text-light mb-3"><i class="fa-solid fa-sliders me-2 text-primary"></i> إعدادات التنبيهات التلقائية</h5>
            
            <form action="/whatsapp/config" method="POST">
                <div class="bg-dark p-3 rounded-3 border border-secondary mb-3">
                    <div class="form-check form-switch mb-0">
                        <input class="form-check-input" type="checkbox" name="enabled" id="waEnabled" value="1" <?= !empty($config['enabled']) ? 'checked' : '' ?>>
                        <label class="form-check-label text-light fw-bold fs-7" for="waEnabled">
                            <i class="fa-solid fa-receipt me-1 text-success"></i> إرسال وصولات التجديد والفواتير آلياً عند كل عملية
                        </label>
                    </div>
                </div>

                <div class="bg-dark p-3 rounded-3 border border-secondary mb-3">
                    <div class="form-check form-switch mb-2">
                        <input class="form-check-input" type="checkbox" name="reminder_enabled" id="waReminder" value="1" <?= !empty($config['reminder_enabled']) ? 'checked' : '' ?>>
                        <label class="form-check-label text-light fw-bold fs-7" for="waReminder">
                            <i class="fa-solid fa-bell me-1 text-warning"></i> إرسال تنبيهات تلقائية قبل انتهاء الاشتراك
                        </label>
                    </div>
                    <div class="mt-2 pt-2 border-top border-secondary">
                        <label class="form-label fs-8 text-muted mb-1">وقت إرسال التنبيه التلقائي:</label>
                        <select name="reminder_hours" class="form-select form-select-sm bg-dark border-secondary text-light">
                            <option value="24" <?= ($config['reminder_hours'] ?? 48) == 24 ? 'selected' : '' ?>>قبل 24 ساعة (يوم واحد)</option>
                            <option value="48" <?= ($config['reminder_hours'] ?? 48) == 48 ? 'selected' : '' ?>>قبل 48 ساعة (يومان - موصى به)</option>
                            <option value="72" <?= ($config['reminder_hours'] ?? 48) == 72 ? 'selected' : '' ?>>قبل 72 ساعة (3 أيام)</option>
                        </select>
                    </div>
                </div>

                <button type="submit" class="btn btn-primary px-4 py-2 rounded-pill fw-bold">
                    <i class="fa-solid fa-floppy-disk me-2"></i> حفظ إعدادات الواتساب
                </button>
            </form>
        </div>
    </div>
</div>

<!-- Message Templates Studio -->
<div class="card glass-card border-0 shadow-sm rounded-4 p-4">
    <div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-3">
        <div>
            <h5 class="fw-bold text-light mb-1"><i class="fa-solid fa-pen-ruler me-2 text-warning"></i> محرر قوالب الرسائل المخصصة</h5>
            <p class="text-muted fs-8 mb-0">يمكنك تعديل نصوص وقوالب الرسائل واستخدام المتغيرات الذكية ليتم تعويضها تلقائياً</p>
        </div>
    </div>

    <!-- Available Variables Pill Box -->
    <div class="bg-dark p-3 rounded-3 border border-secondary mb-4">
        <span class="text-muted fs-8 d-block mb-2"><i class="fa-solid fa-code me-1 text-info"></i> المتغيرات الذكية المتاحة (انقر للنسخ):</span>
        <div class="d-flex flex-wrap gap-2">
            <span class="badge bg-secondary font-monospace cursor-pointer" onclick="copyVar('{username}')">{username} اسم المشترك</span>
            <span class="badge bg-secondary font-monospace cursor-pointer" onclick="copyVar('{full_name}')">{full_name} الاسم الكامل</span>
            <span class="badge bg-secondary font-monospace cursor-pointer" onclick="copyVar('{profile}')">{profile} الباقة</span>
            <span class="badge bg-secondary font-monospace cursor-pointer" onclick="copyVar('{price}')">{price} السعر</span>
            <span class="badge bg-secondary font-monospace cursor-pointer" onclick="copyVar('{validity_days}')">{validity_days} الأيام</span>
            <span class="badge bg-secondary font-monospace cursor-pointer" onclick="copyVar('{balance}')">{balance} الرصيد/الدين</span>
            <span class="badge bg-secondary font-monospace cursor-pointer" onclick="copyVar('{expiry_date}')">{expiry_date} تاريخ الانتهاء</span>
            <span class="badge bg-secondary font-monospace cursor-pointer" onclick="copyVar('{amount}')">{amount} المبلغ</span>
            <span class="badge bg-secondary font-monospace cursor-pointer" onclick="copyVar('{notes}')">{notes} الملاحظات</span>
        </div>
    </div>

    <!-- Template Tabs -->
    <ul class="nav nav-pills gap-2 mb-3" id="templatePills" role="tablist">
        <li class="nav-item" role="presentation">
            <button class="nav-link active rounded-pill px-3 py-1 fs-8 fw-bold" data-bs-toggle="pill" data-bs-target="#tmpl-renew-paid" type="button">
                🟢 تجديد مدفوع
            </button>
        </li>
        <li class="nav-item" role="presentation">
            <button class="nav-link rounded-pill px-3 py-1 fs-8 fw-bold" data-bs-toggle="pill" data-bs-target="#tmpl-renew-debt" type="button">
                🔴 تجديد بدين
            </button>
        </li>
        <li class="nav-item" role="presentation">
            <button class="nav-link rounded-pill px-3 py-1 fs-8 fw-bold" data-bs-toggle="pill" data-bs-target="#tmpl-payment" type="button">
                💵 تسديد دفعة
            </button>
        </li>
        <li class="nav-item" role="presentation">
            <button class="nav-link rounded-pill px-3 py-1 fs-8 fw-bold" data-bs-toggle="pill" data-bs-target="#tmpl-add-debt" type="button">
                📋 إضافة دين
            </button>
        </li>
        <li class="nav-item" role="presentation">
            <button class="nav-link rounded-pill px-3 py-1 fs-8 fw-bold" data-bs-toggle="pill" data-bs-target="#tmpl-expiry-reminder" type="button">
                ⚠️ تنبيه الانتهاء
            </button>
        </li>
        <li class="nav-item" role="presentation">
            <button class="nav-link rounded-pill px-3 py-1 fs-8 fw-bold" data-bs-toggle="pill" data-bs-target="#tmpl-debt-reminder" type="button">
                📢 تذكير الديون
            </button>
        </li>
    </ul>

    <div class="tab-content" id="templatePillsContent">
        <!-- 1. Renew Paid -->
        <div class="tab-pane fade show active" id="tmpl-renew-paid" role="tabpanel">
            <form action="/whatsapp/templates" method="POST">
                <input type="hidden" name="template_key" value="renew_paid">
                <div class="mb-3">
                    <label class="form-label fs-7 fw-semibold">نص رسالة التجديد الواصل / المدفوع نقداً</label>
                    <textarea name="template_text" class="form-control bg-dark border-secondary text-light font-monospace" rows="5"><?= htmlspecialchars($templates['renew_paid'] ?? '') ?></textarea>
                </div>
                <button type="submit" class="btn btn-success rounded-pill px-4 fw-bold"><i class="fa-solid fa-floppy-disk me-1"></i> حفظ القالب</button>
            </form>
        </div>

        <!-- 2. Renew Debt -->
        <div class="tab-pane fade" id="tmpl-renew-debt" role="tabpanel">
            <form action="/whatsapp/templates" method="POST">
                <input type="hidden" name="template_key" value="renew_debt">
                <div class="mb-3">
                    <label class="form-label fs-7 fw-semibold">نص رسالة التجديد الآجل / بالدين</label>
                    <textarea name="template_text" class="form-control bg-dark border-secondary text-light font-monospace" rows="5"><?= htmlspecialchars($templates['renew_debt'] ?? '') ?></textarea>
                </div>
                <button type="submit" class="btn btn-success rounded-pill px-4 fw-bold"><i class="fa-solid fa-floppy-disk me-1"></i> حفظ القالب</button>
            </form>
        </div>

        <!-- 3. Payment -->
        <div class="tab-pane fade" id="tmpl-payment" role="tabpanel">
            <form action="/whatsapp/templates" method="POST">
                <input type="hidden" name="template_key" value="payment">
                <div class="mb-3">
                    <label class="form-label fs-7 fw-semibold">نص وصل تسديد دفعة مالية</label>
                    <textarea name="template_text" class="form-control bg-dark border-secondary text-light font-monospace" rows="5"><?= htmlspecialchars($templates['payment'] ?? '') ?></textarea>
                </div>
                <button type="submit" class="btn btn-success rounded-pill px-4 fw-bold"><i class="fa-solid fa-floppy-disk me-1"></i> حفظ القالب</button>
            </form>
        </div>

        <!-- 4. Add Debt -->
        <div class="tab-pane fade" id="tmpl-add-debt" role="tabpanel">
            <form action="/whatsapp/templates" method="POST">
                <input type="hidden" name="template_key" value="add_debt">
                <div class="mb-3">
                    <label class="form-label fs-7 fw-semibold">نص إشعار تسجيل دين جديد على المشترك</label>
                    <textarea name="template_text" class="form-control bg-dark border-secondary text-light font-monospace" rows="5"><?= htmlspecialchars($templates['add_debt'] ?? '') ?></textarea>
                </div>
                <button type="submit" class="btn btn-success rounded-pill px-4 fw-bold"><i class="fa-solid fa-floppy-disk me-1"></i> حفظ القالب</button>
            </form>
        </div>

        <!-- 5. Expiry Reminder -->
        <div class="tab-pane fade" id="tmpl-expiry-reminder" role="tabpanel">
            <form action="/whatsapp/templates" method="POST">
                <input type="hidden" name="template_key" value="expiry_reminder">
                <div class="mb-3">
                    <label class="form-label fs-7 fw-semibold">نص رسالة التنبيه بقرب انتهاء الاشتراك</label>
                    <textarea name="template_text" class="form-control bg-dark border-secondary text-light font-monospace" rows="5"><?= htmlspecialchars($templates['expiry_reminder'] ?? '') ?></textarea>
                </div>
                <button type="submit" class="btn btn-success rounded-pill px-4 fw-bold"><i class="fa-solid fa-floppy-disk me-1"></i> حفظ القالب</button>
            </form>
        </div>

        <!-- 6. Debt Reminder -->
        <div class="tab-pane fade" id="tmpl-debt-reminder" role="tabpanel">
            <form action="/whatsapp/templates" method="POST">
                <input type="hidden" name="template_key" value="debt_reminder">
                <div class="mb-3">
                    <label class="form-label fs-7 fw-semibold">نص رسالة المطالبة بالديون المستحقة</label>
                    <textarea name="template_text" class="form-control bg-dark border-secondary text-light font-monospace" rows="5"><?= htmlspecialchars($templates['debt_reminder'] ?? '') ?></textarea>
                </div>
                <button type="submit" class="btn btn-success rounded-pill px-4 fw-bold"><i class="fa-solid fa-floppy-disk me-1"></i> حفظ القالب</button>
            </form>
        </div>
    </div>
</div>

<!-- Test Message Modal -->
<div class="modal fade" id="testModal" tabindex="-1" aria-labelledby="testModalLabel" aria-hidden="true">
    <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content glass-card border-success text-light">
            <div class="modal-header border-secondary">
                <h5 class="modal-title fw-bold text-success" id="testModalLabel"><i class="fa-solid fa-paper-plane me-2"></i> إرسال رسالة تجريبية عبر الواتساب</h5>
                <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" aria-label="Close"></button>
            </div>
            <form action="/whatsapp/test" method="POST">
                <div class="modal-body">
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">رقم الهاتف المستلم (مع المفتاح الدولي) *</label>
                        <input type="text" name="phone" class="form-control bg-dark border-secondary text-light font-monospace" required placeholder="9647701234567">
                    </div>
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">نص الرسالة *</label>
                        <textarea name="message" class="form-control bg-dark border-secondary text-light" rows="4" required>تحية طيبة، هذه رسالة تجريبية من منظومة SASMAN MikroTik Manager عبر واتساب بوت ✅</textarea>
                    </div>
                </div>
                <div class="modal-footer border-secondary">
                    <button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal">إلغاء</button>
                    <button type="submit" class="btn btn-success px-4 fw-bold">إرسال الرسالة الآن</button>
                </div>
            </form>
        </div>
    </div>
</div>

<script>
function copyVar(val) {
    navigator.clipboard.writeText(val);
    alert('تم نسخ المتغير: ' + val);
}
</script>
