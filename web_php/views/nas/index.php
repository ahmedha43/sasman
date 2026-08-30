<div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
    <div>
        <h4 class="fw-bold text-light mb-1"><i class="fa-solid fa-network-wired me-2 text-info"></i> إدارة راوترات المايكروتك (MikroTik / NAS & RadSec)</h4>
        <p class="text-muted mb-0 fs-7">ربط راوترات الشبكة، شهادات التشفير RadSec mTLS، وسكربتات التهيئة التلقائية</p>
    </div>
    <div class="d-flex gap-2">
        <form action="/nas/quick-setup" method="POST" class="d-inline">
            <button type="submit" class="btn btn-outline-success rounded-pill px-3 fw-bold" onclick="return confirm('تطبيق إعداد الراديوس السريع على المايكروتك المتصل؟');">
                <i class="fa-solid fa-bolt me-1"></i> إعداد راديوس سريع
            </button>
        </form>
        <button class="btn btn-primary rounded-pill px-4 fw-bold shadow-sm" data-bs-toggle="modal" data-bs-target="#addNasModal">
            <i class="fa-solid fa-plus me-2"></i> إضافة راوتر مايكروتك
        </button>
    </div>
</div>

<div class="row g-4 mb-4">
    <!-- Router Info Card -->
    <div class="col-12 col-lg-7">
        <div class="card glass-card border-0 shadow-sm rounded-4 p-4 h-100">
            <h5 class="fw-bold text-light mb-3"><i class="fa-solid fa-shield-halved me-2 text-info"></i> حالة بروتوكول التشفير و RadSec</h5>
            
            <div class="list-group list-group-flush bg-transparent">
                <div class="list-group-item bg-transparent text-light border-secondary d-flex justify-content-between align-items-center py-3">
                    <span class="text-muted">حالة الاتصال المباشر:</span>
                    <?php if (!empty($nas['connected'])): ?>
                    <span class="badge bg-success px-3 py-2 fs-7"><i class="fa-solid fa-circle-check me-1"></i> متصل بنجاح</span>
                    <?php else: ?>
                    <span class="badge bg-danger px-3 py-2 fs-7"><i class="fa-solid fa-circle-xmark me-1"></i> غير متصل</span>
                    <?php endif; ?>
                </div>

                <div class="list-group-item bg-transparent text-light border-secondary d-flex justify-content-between align-items-center py-3">
                    <span class="text-muted">بروتوكول المصادقة:</span>
                    <span class="badge bg-primary-subtle text-primary border border-primary font-monospace"><?= htmlspecialchars($nas['protocol'] ?? 'RadSec RFC 6614 (mTLS :2083)') ?></span>
                </div>

                <div class="list-group-item bg-transparent text-light border-secondary d-flex justify-content-between align-items-center py-3">
                    <span class="text-muted">شهادة التشفير (Common Name):</span>
                    <span class="font-monospace text-warning fs-7"><?= htmlspecialchars($nas['common_name'] ?? 'agent-SASMAN') ?></span>
                </div>

                <div class="list-group-item bg-transparent text-light border-secondary d-flex justify-content-between align-items-center py-3">
                    <span class="text-muted">زمن الاستجابة (Latency):</span>
                    <span class="fw-bold text-success"><?= htmlspecialchars($nas['latency_ms'] ?? 1) ?> ms</span>
                </div>

                <div class="list-group-item bg-transparent text-light border-secondary d-flex justify-content-between align-items-center py-3">
                    <span class="text-muted">عنوان الوصول المباشر لـ Winbox:</span>
                    <?php if (!empty($nas['winbox_address'])): ?>
                    <div class="d-flex align-items-center gap-2">
                        <code class="bg-dark px-2 py-1 rounded text-info fw-bold"><?= htmlspecialchars($nas['winbox_address']) ?></code>
                        <button class="btn btn-sm btn-outline-info" onclick="navigator.clipboard.writeText('<?= htmlspecialchars($nas['winbox_address']) ?>'); alert('تم نسخ عنوان Winbox!');"><i class="fa-solid fa-copy"></i></button>
                    </div>
                    <?php else: ?>
                    <span class="text-muted">المنفذ الافتراضي (127.0.0.1:8291)</span>
                    <?php endif; ?>
                </div>
            </div>
        </div>
    </div>

    <!-- Provisioning Script Card -->
    <div class="col-12 col-lg-5">
        <div class="card glass-card border-0 shadow-sm rounded-4 p-4 h-100">
            <h5 class="fw-bold text-light mb-2"><i class="fa-solid fa-terminal me-2 text-warning"></i> سكربت التثبيت السريع للمايكروتك</h5>
            <p class="text-muted fs-8 mb-3">انسخ هذا الأمر وضعه في Terminal المايكروتك لربط الراوتر بالشهادات تلقائياً:</p>
            
            <div class="bg-dark p-3 rounded-3 border border-secondary mb-3 position-relative">
                <textarea class="form-control bg-transparent border-0 text-info font-monospace fs-8 p-0" rows="5" readonly id="scriptBox"><?= htmlspecialchars($provision_code) ?></textarea>
            </div>

            <button class="btn btn-outline-warning w-100 py-2 fw-bold" onclick="navigator.clipboard.writeText(document.getElementById('scriptBox').value); alert('تم نسخ سكربت المايكروتك بنجاح!');">
                <i class="fa-solid fa-copy me-2"></i> نسخ السكربت بالكامل
            </button>
        </div>
    </div>
</div>

<!-- Configured Routers Table -->
<div class="card glass-card border-0 shadow-sm rounded-4 p-4">
    <h5 class="fw-bold text-light mb-3"><i class="fa-solid fa-server me-2 text-primary"></i> راوترات المايكروتك المعرفة (Configured NAS Devices)</h5>
    <div class="table-responsive">
        <table class="table table-dark table-hover align-middle mb-0">
            <thead class="table-secondary">
                <tr>
                    <th>#</th>
                    <th>عنوان IP (NAS Name)</th>
                    <th>اسم الراوتر</th>
                    <th>السر المشترك (Secret)</th>
                    <th>النوع</th>
                    <th>الوصف</th>
                    <th class="text-center">الشهادات والعمليات</th>
                </tr>
            </thead>
            <tbody>
                <?php if (empty($routers)): ?>
                <tr>
                    <td colspan="7" class="text-center py-5 text-muted">لا توجد رواترات مايكروتك مضافة حالياً</td>
                </tr>
                <?php else: ?>
                <?php foreach ($routers as $i => $r): ?>
                <?php
                    $ip = $r['ip'] ?? $r['nasname'] ?? '';
                    $name = $r['name'] ?? $r['shortname'] ?? '—';
                    $secret = $r['secret'] ?? '—';
                    $radsec = $r['radsec_status'] ?? 'none';
                    $cn = $r['common_name'] ?? '';
                    $id = $r['id'] ?? $i;
                ?>
                <tr>
                    <td class="text-muted fs-8"><?= $i + 1 ?></td>
                    <td class="fw-bold text-info font-monospace fs-6">
                        <i class="fa-solid fa-network-wired me-1 text-primary"></i> <?= htmlspecialchars($ip) ?>
                    </td>
                    <td><strong class="text-light"><?= htmlspecialchars($name) ?></strong></td>
                    <td><code class="bg-dark px-2 py-1 rounded text-warning font-monospace"><?= htmlspecialchars($secret) ?></code></td>
                    <td>
                        <?php if ($radsec === 'online'): ?>
                        <span class="badge bg-success-subtle text-success border border-success px-2 py-1"><i class="fa-solid fa-shield-check me-1"></i> RadSec mTLS متصل</span>
                        <?php elseif ($radsec === 'configured'): ?>
                        <span class="badge bg-info-subtle text-info border border-info px-2 py-1">شهادة مفعلة</span>
                        <?php else: ?>
                        <span class="badge bg-secondary">UDP تقليدي (1812)</span>
                        <?php endif; ?>
                    </td>
                    <td class="text-muted fs-8 font-monospace"><?= htmlspecialchars($cn ?: '—') ?></td>
                    <td class="text-center">
                        <div class="btn-group btn-group-sm">
                            <form action="/nas/<?= urlencode($id) ?>/generate-cert" method="POST" class="d-inline">
                                <button type="submit" class="btn btn-outline-warning" title="توليد وتحديث شهادة RadSec mTLS"><i class="fa-solid fa-certificate"></i></button>
                            </form>
                            <a href="/nas/<?= urlencode($ip) ?>/delete" class="btn btn-outline-danger" onclick="return confirm('حذف راوتر المايكروتك [<?= htmlspecialchars($ip) ?>]؟');" title="حذف">
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

<!-- Add NAS Modal -->
<div class="modal fade" id="addNasModal" tabindex="-1" aria-labelledby="addNasModalLabel" aria-hidden="true">
    <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content glass-card border-secondary text-light">
            <div class="modal-header border-secondary">
                <h5 class="modal-title fw-bold" id="addNasModalLabel"><i class="fa-solid fa-plus me-2 text-primary"></i> إضافة راوتر مايكروتك جديد</h5>
                <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" aria-label="Close"></button>
            </div>
            <form action="/nas" method="POST">
                <div class="modal-body">
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">عنوان IP للراوتر (NAS IP) *</label>
                        <input type="text" name="nasname" class="form-control bg-dark border-secondary text-light font-monospace" required placeholder="192.168.10.1 أو 127.0.0.1">
                    </div>
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">اسم الراوتر (Shortname) *</label>
                        <input type="text" name="shortname" class="form-control bg-dark border-secondary text-light" required placeholder="Main-CCR-2004">
                    </div>
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">السر المشترك (RADIUS Secret) *</label>
                        <input type="text" name="secret" class="form-control bg-dark border-secondary text-light font-monospace" required value="sasman123" placeholder="sasman123">
                    </div>
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">نوع الراوتر</label>
                        <select name="type" class="form-select bg-dark border-secondary text-light">
                            <option value="mikrotik" selected>MikroTik RouterOS</option>
                            <option value="cisco">Cisco</option>
                            <option value="other">Other / Linux</option>
                        </select>
                    </div>
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">ملاحظات أو الوصف</label>
                        <input type="text" name="description" class="form-control bg-dark border-secondary text-light" placeholder="راوتر البرج الرئيسي">
                    </div>
                </div>
                <div class="modal-footer border-secondary">
                    <button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal">إلغاء</button>
                    <button type="submit" class="btn btn-primary px-4 fw-bold">حفظ الراوتر</button>
                </div>
            </form>
        </div>
    </div>
</div>
