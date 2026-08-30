<div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
    <div>
        <h4 class="fw-bold text-light mb-1"><i class="fa-solid fa-gears me-2 text-primary"></i> إعدادات النظام، النسخ، واستيراد وتصدير SAS</h4>
        <p class="text-muted mb-0 fs-7">إدارة حساب المدير، النسخ الاحتياطي الآلي، استعادة قاعدة البيانات، واستيراد المشتركين من SAS4 و Excel</p>
    </div>
    <div class="d-flex flex-wrap gap-2">
        <a href="/settings/export/excel" class="btn btn-outline-success rounded-pill px-3 fw-bold shadow-sm">
            <i class="fa-solid fa-file-excel me-2"></i> تصدير المشتركين (Excel)
        </a>
        <a href="/settings/backup/download" class="btn btn-primary rounded-pill px-3 fw-bold shadow-sm">
            <i class="fa-solid fa-download me-2"></i> تحميل نسخة SQLite
        </a>
    </div>
</div>

<div class="card glass-card border-0 shadow-sm rounded-4 p-4">
    <!-- Settings Tabs Navigation -->
    <ul class="nav nav-pills gap-2 mb-4 bg-dark p-2 rounded-4 border border-secondary" id="settingsTabs" role="tablist">
        <li class="nav-item" role="presentation">
            <button class="nav-link active rounded-pill px-3 fw-bold fs-7" data-bs-toggle="pill" data-bs-target="#tab-sas4" type="button">
                <i class="fa-solid fa-file-import me-2 text-warning"></i> استيراد من SAS4
            </button>
        </li>
        <li class="nav-item" role="presentation">
            <button class="nav-link rounded-pill px-3 fw-bold fs-7" data-bs-toggle="pill" data-bs-target="#tab-excel" type="button">
                <i class="fa-solid fa-file-excel me-2 text-success"></i> استيراد وتصدير Excel
            </button>
        </li>
        <li class="nav-item" role="presentation">
            <button class="nav-link rounded-pill px-3 fw-bold fs-7" data-bs-toggle="pill" data-bs-target="#tab-database" type="button">
                <i class="fa-solid fa-database me-2 text-info"></i> النسخ واستعادة البيانات
            </button>
        </li>
        <li class="nav-item" role="presentation">
            <button class="nav-link rounded-pill px-3 fw-bold fs-7" data-bs-toggle="pill" data-bs-target="#tab-telegram" type="button">
                <i class="fa-brands fa-telegram me-2 text-info"></i> النسخ عبر تيليجرام
            </button>
        </li>
        <li class="nav-item" role="presentation">
            <button class="nav-link rounded-pill px-3 fw-bold fs-7" data-bs-toggle="pill" data-bs-target="#tab-security" type="button">
                <i class="fa-solid fa-shield-halved me-2 text-danger"></i> الأمان وكلمة المرور
            </button>
        </li>
        <li class="nav-item" role="presentation">
            <button class="nav-link rounded-pill px-3 fw-bold fs-7" data-bs-toggle="pill" data-bs-target="#tab-tunnel" type="button">
                <i class="fa-solid fa-cloud me-2 text-primary"></i> النفق السحابي
            </button>
        </li>
    </ul>

    <div class="tab-content" id="settingsTabsContent">
        <!-- 1. TAB: SAS4 DIRECT MIGRATION -->
        <div class="tab-pane fade show active" id="tab-sas4" role="tabpanel">
            <div class="bg-dark p-4 rounded-4 border border-secondary">
                <div class="d-flex align-items-center gap-3 mb-3">
                    <div class="bg-warning bg-opacity-25 text-warning p-3 rounded-4">
                        <i class="fa-solid fa-cloud-arrow-down fs-2"></i>
                    </div>
                    <div>
                        <h5 class="fw-bold text-light mb-1">استيراد وترحيل البيانات مباشرة من منظومة SAS4</h5>
                        <p class="text-muted fs-8 mb-0">سحب كافة بيانات المشتركين، الباقات، الأرصدة، والديون من سيرفر SAS4 القديم ومطابقتها فوراً في SASMAN</p>
                    </div>
                </div>

                <div class="alert alert-warning bg-opacity-10 border border-warning text-warning fs-8 rounded-3 mb-4">
                    <i class="fa-solid fa-triangle-exclamation me-2"></i> تأكد من كتابة عنوان سيرفر SAS4 كاملاً مع المنفذ إن وجد (مثال: <code>http://192.168.1.50/</code> أو <code>https://sas4.myserver.com/</code>).
                </div>

                <form action="/settings/import/sas4" method="POST">
                    <div class="row g-3 mb-4">
                        <div class="col-12 col-md-6">
                            <label class="form-label fs-7 fw-semibold">رابط سيرفر SAS4 (Server URL) *</label>
                            <input type="text" name="url" class="form-control bg-dark border-secondary text-light font-monospace" placeholder="http://192.168.1.50/" required>
                        </div>
                        <div class="col-12 col-md-3">
                            <label class="form-label fs-7 fw-semibold">اسم مستخدم مدير SAS4 *</label>
                            <input type="text" name="username" class="form-control bg-dark border-secondary text-light font-monospace" placeholder="admin" required>
                        </div>
                        <div class="col-12 col-md-3">
                            <label class="form-label fs-7 fw-semibold">كلمة مرور مدير SAS4 *</label>
                            <input type="password" name="password" class="form-control bg-dark border-secondary text-light font-monospace" placeholder="••••••••" required>
                        </div>
                    </div>

                    <button type="submit" class="btn btn-warning px-4 py-2 rounded-pill fw-bold text-dark" onclick="return confirm('هل أنت متأكد من بدء عملية استيراد المشتركين والباقات من SAS4؟');">
                        <i class="fa-solid fa-bolt me-2"></i> بدء سحب واستيراد البيانات من SAS4
                    </button>
                </form>
            </div>
        </div>

        <!-- 2. TAB: EXCEL IMPORT & EXPORT -->
        <div class="tab-pane fade" id="tab-excel" role="tabpanel">
            <div class="row g-4">
                <div class="col-12 col-lg-6">
                    <div class="bg-dark p-4 rounded-4 border border-secondary h-100">
                        <h5 class="fw-bold text-success mb-2"><i class="fa-solid fa-file-excel me-2"></i> تصدير كافة المشتركين (Export to Excel)</h5>
                        <p class="text-muted fs-8 mb-4">تحميل ملف إكسيل (.xlsx) منسق ومصنف يحتوي على كامل بيانات المشتركين، أرقام الهواتف، الباقات، وتواريخ الانتهاء</p>
                        
                        <div class="p-3 bg-dark border border-secondary rounded-3 text-center mb-4">
                            <i class="fa-solid fa-file-arrow-down text-success fs-1 mb-2"></i>
                            <span class="text-light d-block fs-7 fw-bold">تصدير قاعدة بيانات المشتركين الحالية</span>
                            <small class="text-muted fs-9">ملف إكسيل قياسي جاهز للطباعة والأرشفة</small>
                        </div>

                        <a href="/settings/export/excel" class="btn btn-success w-100 py-2 rounded-pill fw-bold">
                            <i class="fa-solid fa-download me-2"></i> تنزيل ملف الإكسيل الآن
                        </a>
                    </div>
                </div>

                <div class="col-12 col-lg-6">
                    <div class="bg-dark p-4 rounded-4 border border-secondary h-100">
                        <h5 class="fw-bold text-primary mb-2"><i class="fa-solid fa-file-arrow-up me-2"></i> استيراد المشتركين من ملف Excel</h5>
                        <p class="text-muted fs-8 mb-3">رفع ملف إكسيل (.xlsx) يحتوي على أسماء المستخدمين والباقات لإضافتهم دفعة واحدة للنظام</p>

                        <form action="/settings/import/excel" method="POST" enctype="multipart/form-data">
                            <div class="mb-4">
                                <label class="form-label fs-7 fw-semibold">اختر ملف الإكسيل (.xlsx) *</label>
                                <input type="file" name="file" accept=".xlsx" class="form-control bg-dark border-secondary text-light" required>
                                <small class="text-muted fs-9 d-block mt-1">يجب أن يحتوي الملف على أعمدة: username, password, profile_name</small>
                            </div>

                            <button type="submit" class="btn btn-primary w-100 py-2 rounded-pill fw-bold">
                                <i class="fa-solid fa-upload me-2"></i> رفع واستيراد ملف الإكسيل
                            </button>
                        </form>
                    </div>
                </div>
            </div>
        </div>

        <!-- 3. TAB: DATABASE MAINTENANCE & RESTORE -->
        <div class="tab-pane fade" id="tab-database" role="tabpanel">
            <div class="row g-4">
                <div class="col-12 col-lg-6">
                    <div class="bg-dark p-4 rounded-4 border border-secondary h-100">
                        <h5 class="fw-bold text-info mb-2"><i class="fa-solid fa-database me-2"></i> النسخ الاحتياطي المحلي (Backup)</h5>
                        <p class="text-muted fs-8 mb-4">تحميل نسخة احتياطية فورية من قاعدة بيانات SQLite بكل الجداول والمشتركين والعمليات المالية</p>

                        <div class="row g-2 mb-4">
                            <div class="col-6">
                                <div class="p-3 bg-dark border border-secondary rounded-3 text-center">
                                    <span class="text-muted fs-8 d-block">نوع المحرك</span>
                                    <strong class="text-light fs-7">SQLite 3 (WAL)</strong>
                                </div>
                            </div>
                            <div class="col-6">
                                <div class="p-3 bg-dark border border-secondary rounded-3 text-center">
                                    <span class="text-muted fs-8 d-block">الحالة</span>
                                    <strong class="text-success fs-7"><i class="fa-solid fa-circle-check me-1"></i> جاهزة</strong>
                                </div>
                            </div>
                        </div>

                        <a href="/settings/backup/download" class="btn btn-info w-100 py-2 rounded-pill fw-bold text-dark">
                            <i class="fa-solid fa-download me-2"></i> تحميل ملف النسخة الاحتياطية (.sqlite)
                        </a>
                    </div>
                </div>

                <div class="col-12 col-lg-6">
                    <div class="bg-dark p-4 rounded-4 border border-secondary h-100">
                        <h5 class="fw-bold text-danger mb-2"><i class="fa-solid fa-clock-rotate-left me-2"></i> استعادة قاعدة البيانات (Restore)</h5>
                        <p class="text-muted fs-8 mb-3">رفع ملف قاعدة بيانات سابق (.sqlite أو .db) لاستعادة كافة الحسابات والبيانات بدقة</p>

                        <form action="/settings/database/restore" method="POST" enctype="multipart/form-data">
                            <div class="mb-4">
                                <label class="form-label fs-7 fw-semibold">اختر ملف النسخة الاحتياطية (.sqlite / .db) *</label>
                                <input type="file" name="file" accept=".sqlite,.db,.sql" class="form-control bg-dark border-secondary text-light" required>
                                <small class="text-danger fs-9 d-block mt-1">تحذير: استعادة النسخة ستستبدل البيانات الحالية ويعاد تشغيل الخدمة تلقائياً</small>
                            </div>

                            <button type="submit" class="btn btn-danger w-100 py-2 rounded-pill fw-bold" onclick="return confirm('تنبيه: هل أنت متأكد من رغبتك في استعادة قاعدة البيانات؟ سيتم استبدال البيانات الحالية.');">
                                <i class="fa-solid fa-rotate-left me-2"></i> استعادة قاعدة البيانات الآن
                            </button>
                        </form>
                    </div>
                </div>
            </div>
        </div>

        <!-- 4. TAB: TELEGRAM CLOUD BACKUP -->
        <div class="tab-pane fade" id="tab-telegram" role="tabpanel">
            <div class="bg-dark p-4 rounded-4 border border-secondary">
                <div class="d-flex flex-column flex-md-row justify-content-between align-items-md-center gap-3 mb-4 pb-3 border-bottom border-secondary">
                    <div>
                        <h5 class="fw-bold text-light mb-1"><i class="fa-brands fa-telegram me-2 text-info"></i> النسخ الاحتياطي التلقائي إلى تيليجرام</h5>
                        <p class="text-muted fs-8 mb-0">إرسال نسخة مشفرة ومضغوطة من قاعدة البيانات تلقائياً إلى محادثة أو قناة تيليجرام خاصة بك</p>
                    </div>
                    <form action="/settings/backup/telegram/test" method="POST" class="d-inline">
                        <button type="submit" class="btn btn-outline-info rounded-pill px-4 fw-bold">
                            <i class="fa-solid fa-paper-plane me-2"></i> إرسال نسخة تجريبية لتيليجرام الآن
                        </button>
                    </form>
                </div>

                <form action="/settings/telegram" method="POST">
                    <div class="form-check form-switch mb-4">
                        <input class="form-check-input" type="checkbox" name="enabled" id="tgEnabled" value="1" <?= !empty($telegram['enabled']) ? 'checked' : '' ?>>
                        <label class="form-check-label text-light fw-bold fs-7" for="tgEnabled">
                            تفعيل النسخ الاحتياطي المجدول عبر بوت تيليجرام
                        </label>
                    </div>

                    <div class="row g-3 mb-3">
                        <div class="col-12 col-md-6">
                            <label class="form-label fs-7 fw-semibold">رمز توكن البوت (Bot Token) *</label>
                            <input type="text" name="bot_token" class="form-control bg-dark border-secondary text-light font-monospace" placeholder="123456789:ABCdefGhIJKlmNoPQRsTUVwxyZ" value="<?= htmlspecialchars($telegram['bot_token'] ?? '') ?>">
                        </div>
                        <div class="col-12 col-md-6">
                            <label class="form-label fs-7 fw-semibold">معرف المحادثة أو القناة (Chat ID) *</label>
                            <input type="text" name="chat_id" class="form-control bg-dark border-secondary text-light font-monospace" placeholder="مثال: -1001234567890 أو 12345678" value="<?= htmlspecialchars($telegram['chat_id'] ?? '') ?>">
                        </div>
                    </div>

                    <div class="mb-4">
                        <label class="form-label fs-7 fw-semibold">تكرار الإرسال التلقائي</label>
                        <select name="interval_hours" class="form-select bg-dark border-secondary text-light" style="max-width: 300px;">
                            <option value="6" <?= ($telegram['interval_hours'] ?? 24) == 6 ? 'selected' : '' ?>>كل 6 ساعات (موصى به للشبكات الكبيرة)</option>
                            <option value="12" <?= ($telegram['interval_hours'] ?? 24) == 12 ? 'selected' : '' ?>>كل 12 ساعة</option>
                            <option value="24" <?= ($telegram['interval_hours'] ?? 24) == 24 ? 'selected' : '' ?>>كل 24 ساعة (يومياً)</option>
                        </select>
                    </div>

                    <button type="submit" class="btn btn-info px-4 py-2 rounded-pill fw-bold text-dark">
                        <i class="fa-solid fa-floppy-disk me-2"></i> حفظ إعدادات تيليجرام
                    </button>
                </form>
            </div>
        </div>

        <!-- 5. TAB: SECURITY & PASSWORD -->
        <div class="tab-pane fade" id="tab-security" role="tabpanel">
            <div class="row g-4">
                <div class="col-12 col-lg-5">
                    <div class="bg-dark p-4 rounded-4 border border-secondary text-center h-100">
                        <div class="avatar bg-primary text-white p-4 rounded-circle d-inline-block mb-3 shadow">
                            <i class="fa-solid fa-user-shield fs-1"></i>
                        </div>
                        <h4 class="fw-bold text-light mb-1 font-monospace">@<?= htmlspecialchars($admin['username'] ?? 'admin') ?></h4>
                        <span class="badge bg-primary-subtle text-primary border border-primary px-3 py-1 mb-3">مدير رئيسي (Super Admin)</span>
                        
                        <div class="list-group list-group-flush bg-transparent text-start border-top border-secondary pt-3">
                            <div class="list-group-item bg-transparent text-light border-0 d-flex justify-content-between py-2 fs-7">
                                <span class="text-muted">الرصيد المالي للمدير:</span>
                                <strong class="text-success font-monospace"><?= number_format($admin['balance'] ?? 0, 0) ?> د.ع</strong>
                            </div>
                            <div class="list-group-item bg-transparent text-light border-0 d-flex justify-content-between py-2 fs-7">
                                <span class="text-muted">حالة الحساب:</span>
                                <span class="badge bg-success">نشط ومفعل</span>
                            </div>
                        </div>
                    </div>
                </div>

                <div class="col-12 col-lg-7">
                    <div class="bg-dark p-4 rounded-4 border border-secondary">
                        <h5 class="fw-bold text-light mb-3"><i class="fa-solid fa-key me-2 text-warning"></i> تغيير كلمة مرور المدير</h5>
                        <form action="/settings/password" method="POST">
                            <div class="mb-3">
                                <label class="form-label fs-7 fw-semibold">كلمة المرور الحالية *</label>
                                <input type="password" name="old_password" class="form-control bg-dark border-secondary text-light font-monospace" required placeholder="••••••••">
                            </div>
                            <div class="mb-3">
                                <label class="form-label fs-7 fw-semibold">كلمة المرور الجديدة *</label>
                                <input type="password" name="new_password" class="form-control bg-dark border-secondary text-light font-monospace" required placeholder="••••••••" minlength="4">
                            </div>
                            <div class="mb-4">
                                <label class="form-label fs-7 fw-semibold">تأكيد كلمة المرور الجديدة *</label>
                                <input type="password" name="confirm_password" class="form-control bg-dark border-secondary text-light font-monospace" required placeholder="••••••••" minlength="4">
                            </div>
                            <button type="submit" class="btn btn-warning px-4 py-2 rounded-pill fw-bold">
                                <i class="fa-solid fa-floppy-disk me-2"></i> تحديث كلمة المرور
                            </button>
                        </form>
                    </div>
                </div>
            </div>
        </div>

        <!-- 6. TAB: CLOUD TUNNEL -->
        <div class="tab-pane fade" id="tab-tunnel" role="tabpanel">
            <div class="bg-dark p-4 rounded-4 border border-secondary">
                <h5 class="fw-bold text-light mb-3"><i class="fa-solid fa-cloud me-2 text-primary"></i> إعدادات النفق السحابي والنطاق الفرعي (Cloud Tunnel)</h5>
                <p class="text-muted fs-7 mb-4">ربط الخادم المحلي بالنفق السحابي الآمن لتجاوز الـ NAT والـ Double NAT والوصول للوحة من أي مكان في العالم</p>

                <form action="/settings/tunnel" method="POST">
                    <div class="row g-3 mb-4">
                        <div class="col-12 col-md-6">
                            <label class="form-label fs-7 fw-semibold">النطاق الفرعي للوكيل (Subdomain) *</label>
                            <input type="text" name="subdomain" class="form-control bg-dark border-secondary text-light font-monospace" placeholder="ahmed100" value="<?= htmlspecialchars($tunnel['subdomain'] ?? 'ahmed100') ?>" required>
                        </div>
                        <div class="col-12 col-md-6">
                            <label class="form-label fs-7 fw-semibold">الدومين المركزي (Central Server)</label>
                            <input type="text" name="central_domain" class="form-control bg-dark border-secondary text-light font-monospace" placeholder="sas-man.net" value="<?= htmlspecialchars($tunnel['central_domain'] ?? 'sas-man.net') ?>">
                        </div>
                    </div>

                    <button type="submit" class="btn btn-primary px-4 py-2 rounded-pill fw-bold">
                        <i class="fa-solid fa-floppy-disk me-2"></i> حفظ إعدادات النفق
                    </button>
                </form>
            </div>
        </div>
    </div>
</div>
