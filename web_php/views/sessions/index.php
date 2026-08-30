<div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
    <div>
        <h4 class="fw-bold text-light mb-1"><i class="fa-solid fa-tower-broadcast me-2 text-success"></i> المشتركون المتصلون حالياً (Live Sessions)</h4>
        <p class="text-muted mb-0 fs-7">مراقبة الجلسات الحية على سيرفر PPPoE و Hotspot مع فحص الـ IP و MAC وفصل الجلسة</p>
    </div>
    <a href="/sessions" class="btn btn-outline-success rounded-pill px-4 fw-bold">
        <i class="fa-solid fa-arrows-rotate me-2"></i> تحديث الجلسات الحية
    </a>
</div>

<!-- Sessions Table -->
<div class="card glass-card border-0 shadow-sm rounded-4 p-4">
    <div class="table-responsive">
        <table class="table table-dark table-hover align-middle mb-0">
            <thead class="table-secondary">
                <tr>
                    <th>#</th>
                    <th>اسم المشترك</th>
                    <th>عنوان IP</th>
                    <th>عنوان MAC</th>
                    <th>وقت الاتصال (Uptime)</th>
                    <th>الاستهلاك الحي</th>
                    <th class="text-center">فصل الجلسة</th>
                </tr>
            </thead>
            <tbody>
                <?php if (empty($sessions)): ?>
                <tr>
                    <td colspan="7" class="text-center py-5 text-muted">
                        <i class="fa-solid fa-signal-stream fs-1 d-block mb-2 text-secondary"></i>
                        لا توجد جلسات متصلة حالياً على راوتر المايكروتك
                    </td>
                </tr>
                <?php else: ?>
                <?php foreach ($sessions as $i => $s): ?>
                <tr>
                    <td class="text-muted fs-8"><?= $i + 1 ?></td>
                    <td class="fw-bold text-info"><?= htmlspecialchars($s['username'] ?? '') ?></td>
                    <td class="font-monospace text-light"><?= htmlspecialchars($s['ip'] ?? $s['framed_ip'] ?? '—') ?></td>
                    <td class="font-monospace text-muted fs-8"><?= htmlspecialchars($s['mac'] ?? $s['calling_station_id'] ?? '—') ?></td>
                    <td class="fs-7 text-warning"><?= htmlspecialchars($s['uptime'] ?? $s['session_time'] ?? 'متصل') ?></td>
                    <td class="fs-8 text-muted">
                        <i class="fa-solid fa-arrow-down text-success"></i> <?= htmlspecialchars($s['download'] ?? '0 B') ?> /
                        <i class="fa-solid fa-arrow-up text-primary"></i> <?= htmlspecialchars($s['upload'] ?? '0 B') ?>
                    </td>
                    <td class="text-center">
                        <a href="/sessions/<?= urlencode($s['username']) ?>/disconnect" onclick="return confirm('فصل اتصال هذا المشترك من المايكروتك فوراً؟');" class="btn btn-sm btn-outline-danger px-3 rounded-pill">
                            <i class="fa-solid fa-plug-circle-xmark me-1"></i> طرد الجلسة
                        </a>
                    </td>
                </tr>
                <?php endforeach; ?>
                <?php endif; ?>
            </tbody>
        </table>
    </div>
</div>
