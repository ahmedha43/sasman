<!-- Premium Hero Banner -->
<div class="dashboard-hero-card rounded-4 p-4 mb-4 text-center position-relative overflow-hidden shadow-sm" style="background: linear-gradient(135deg, #0f172a 0%, #1e293b 100%); border: 1px solid rgba(255, 255, 255, 0.08);">
    <!-- Subtle tech lines & constellation overlay in background -->
    <div style="position: absolute; inset: 0; opacity: 0.05; pointer-events: none;">
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 200" preserveAspectRatio="none" style="width: 100%; height: 100%;">
            <line x1="50" y1="30" x2="180" y2="150" stroke="#ffffff" stroke-width="1.2"/>
            <line x1="180" y1="150" x2="350" y2="60" stroke="#ffffff" stroke-width="1.2"/>
            <line x1="350" y1="60" x2="520" y2="140" stroke="#ffffff" stroke-width="1.2"/>
            <line x1="520" y1="140" x2="700" y2="40" stroke="#ffffff" stroke-width="1.2"/>
            <circle cx="50" cy="30" r="4.5" fill="#ffffff"/>
            <circle cx="180" cy="150" r="6" fill="#ffffff"/>
            <circle cx="350" cy="60" r="5" fill="#ffffff"/>
            <circle cx="520" cy="140" r="6.5" fill="#ffffff"/>
            <circle cx="700" cy="40" r="5" fill="#ffffff"/>
        </svg>
    </div>

    <div class="position-relative z-2">
        <div class="d-flex flex-column flex-md-row align-items-center justify-content-between gap-3 text-md-start mb-3">
            <div class="d-flex align-items-center gap-3">
                <div class="bg-primary bg-opacity-25 text-primary border border-primary p-3 rounded-4 shadow-sm">
                    <i class="fa-solid fa-server fs-2"></i>
                </div>
                <div>
                    <h3 class="fw-bold text-light mb-1 font-monospace" style="letter-spacing: 0.5px;">SASMAN MIKROTIK RADIUS</h3>
                    <p class="text-white-50 mb-0 fs-7">منظومة إدارة شبكات المايكروتك وراديوس السحابي (Cloud & Local Multi-Tenant Core)</p>
                </div>
            </div>
            <div class="text-md-end">
                <span class="badge bg-success-subtle text-success border border-success px-3 py-2 fs-7 mb-2 d-inline-block">
                    <i class="fa-solid fa-circle-dot me-1 fa-fade"></i> النظام متصل ونشط
                </span>
                <div class="text-white-50 fs-8 font-monospace" id="liveClock"><?= date('Y-m-d H:i:s') ?></div>
            </div>
        </div>

        <!-- Quick Action Buttons -->
        <div class="d-flex flex-wrap justify-content-center justify-content-md-start gap-2 pt-3 border-top border-white border-opacity-10">
            <a href="/users" class="btn btn-primary btn-sm rounded-pill px-3 fw-bold shadow-sm">
                <i class="fa-solid fa-user-plus me-1"></i> إضافة مشترك
            </a>
            <a href="/users" class="btn btn-success btn-sm rounded-pill px-3 fw-bold shadow-sm">
                <i class="fa-solid fa-bolt me-1"></i> تجديد اشتراك
            </a>
            <a href="/vouchers" class="btn btn-warning btn-sm rounded-pill px-3 fw-bold shadow-sm">
                <i class="fa-solid fa-ticket me-1"></i> توليد كروت
            </a>
            <a href="/transactions" class="btn btn-outline-light btn-sm rounded-pill px-3 fw-bold">
                <i class="fa-solid fa-book-bookmark me-1"></i> السجل المالي والديون
            </a>
            <a href="/nas" class="btn btn-outline-info btn-sm rounded-pill px-3 fw-bold">
                <i class="fa-solid fa-network-wired me-1"></i> راوترات المايكروتك
            </a>
        </div>
    </div>
</div>

<!-- Pro Futuristic SAS Metric Cards Grid -->
<div class="sas-pro-grid mb-4">
    <!-- Card 1: Total Users (#4b6584) -->
    <div class="sas-card-pro card-total" onclick="window.location.href='/users'">
        <div class="card-glass-glow"></div>
        <div class="card-content-top">
            <div class="d-flex align-items-center justify-content-between mb-2">
                <div class="card-badge-icon">
                    <i class="fa-solid fa-users"></i>
                </div>
                <div class="card-arrow-link">
                    <i class="fa-solid fa-arrow-up-left"></i>
                </div>
            </div>
            <h4 class="card-title">إجمالي المشتركين</h4>
            <div class="card-sub-label">المستخدمين المسجلين في النظام</div>
        </div>

        <div class="card-content-bottom">
            <div class="card-number-val"><?= number_format($total_users) ?></div>
            <div class="card-footer-pill">
                <i class="fa-solid fa-database me-1 fs-9"></i> قاعدة البيانات نشطة
            </div>
        </div>
        <i class="fa-solid fa-users watermark-icon-pro"></i>
    </div>

    <!-- Card 2: Active Users (#218c74) -->
    <div class="sas-card-pro card-active" onclick="window.location.href='/users?filter=active'">
        <div class="card-glass-glow"></div>
        <div class="card-content-top">
            <div class="d-flex align-items-center justify-content-between mb-2">
                <div class="card-badge-icon">
                    <i class="fa-solid fa-face-smile"></i>
                </div>
                <div class="card-arrow-link">
                    <i class="fa-solid fa-arrow-up-left"></i>
                </div>
            </div>
            <h4 class="card-title">المشتركين النشطين</h4>
            <div class="card-sub-label">الحسابات الفعالة حالياً</div>
        </div>

        <div class="card-content-bottom">
            <div class="card-number-val"><?= number_format($active_users) ?></div>
            <div class="card-footer-pill">
                <span class="active-pulse-dot"></span> <?= $active_percent ?>% نسبة التفعيل
            </div>
        </div>
        <i class="fa-solid fa-face-smile watermark-icon-pro"></i>
    </div>

    <!-- Card 3: Online Users (#3498db) -->
    <div class="sas-card-pro card-online" onclick="window.location.href='/sessions'">
        <div class="card-glass-glow"></div>
        <div class="card-content-top">
            <div class="d-flex align-items-center justify-content-between mb-2">
                <div class="card-badge-icon">
                    <i class="fa-solid fa-tower-broadcast"></i>
                </div>
                <div class="card-arrow-link">
                    <i class="fa-solid fa-arrow-up-left"></i>
                </div>
            </div>
            <h4 class="card-title">المتصلين الآن</h4>
            <div class="card-sub-label">الجلسات الحية في المايكروتك</div>
        </div>

        <div class="card-content-bottom">
            <div class="card-number-val"><?= number_format($online_users) ?></div>
            <div class="card-footer-pill">
                <i class="fa-solid fa-bolt me-1 fs-9"></i> PPPoE & Hotspot Live
            </div>
        </div>
        <i class="fa-solid fa-lightbulb watermark-icon-pro"></i>
    </div>

    <!-- Card 4: Expired Users (#fc5c65) -->
    <div class="sas-card-pro card-expired" onclick="window.location.href='/users?filter=expired'">
        <div class="card-glass-glow"></div>
        <div class="card-content-top">
            <div class="d-flex align-items-center justify-content-between mb-2">
                <div class="card-badge-icon">
                    <i class="fa-solid fa-face-frown"></i>
                </div>
                <div class="card-arrow-link">
                    <i class="fa-solid fa-arrow-up-left"></i>
                </div>
            </div>
            <h4 class="card-title">المشتركين المنتهين</h4>
            <div class="card-sub-label">الحسابات المنتهية / المتوقفة</div>
        </div>

        <div class="card-content-bottom">
            <div class="card-number-val"><?= number_format($expired_users) ?></div>
            <div class="card-footer-pill">
                <i class="fa-solid fa-clock-rotate-left me-1 fs-9"></i> بحاجة للتجديد
            </div>
        </div>
        <i class="fa-solid fa-face-frown watermark-icon-pro"></i>
    </div>

    <!-- Card 5: About To Expire (#f1c40f) -->
    <div class="sas-card-pro card-warning" onclick="window.location.href='/users?filter=about_to_expire'">
        <div class="card-glass-glow"></div>
        <div class="card-content-top">
            <div class="d-flex align-items-center justify-content-between mb-2">
                <div class="card-badge-icon">
                    <i class="fa-solid fa-calendar-days"></i>
                </div>
                <div class="card-arrow-link">
                    <i class="fa-solid fa-arrow-up-left"></i>
                </div>
            </div>
            <h4 class="card-title">قريبين من الانتهاء</h4>
            <div class="card-sub-label">تنتهي اشتراكاتهم خلال 3 أيام</div>
        </div>

        <div class="card-content-bottom">
            <div class="card-number-val"><?= number_format($about_to_expire) ?></div>
            <div class="card-footer-pill">
                <i class="fa-solid fa-bell me-1 fs-9"></i> جاهز للتنبيه بالواتساب
            </div>
        </div>
        <i class="fa-solid fa-calendar-days watermark-icon-pro"></i>
    </div>

    <!-- Card 6: Wallet Balance (#3f51b5) -->
    <div class="sas-card-pro card-wallet" onclick="window.location.href='/transactions'">
        <div class="card-glass-glow"></div>
        <div class="card-content-top">
            <div class="d-flex align-items-center justify-content-between mb-2">
                <div class="card-badge-icon">
                    <i class="fa-solid fa-wallet"></i>
                </div>
                <div class="card-arrow-link">
                    <i class="fa-solid fa-arrow-up-left"></i>
                </div>
            </div>
            <h4 class="card-title">الرصيد الإداري</h4>
            <div class="card-sub-label">رصيد المحفظة الإدارية الحالية</div>
        </div>

        <div class="card-content-bottom">
            <div class="card-number-val text-truncate" style="font-size: 2.1rem;"><?= number_format($admin_balance, 0) ?> <span class="fs-6 fw-bold">د.ع</span></div>
            <div class="card-footer-pill">
                <i class="fa-solid fa-coins me-1 fs-9"></i> الرصيد المالي المتاح
            </div>
        </div>
        <i class="fa-solid fa-wallet watermark-icon-pro"></i>
    </div>
</div>

<!-- MikroTik Live Status Bar -->
<div class="card glass-card border-0 shadow-sm rounded-4 p-4 mb-4">
    <div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3">
        <div class="d-flex align-items-center gap-3">
            <div class="stat-icon bg-info-subtle text-info rounded-3 p-3">
                <i class="fa-solid fa-network-wired fs-3"></i>
            </div>
            <div>
                <h6 class="fw-bold text-light mb-1">اتصال راوتر المايكروتك وبروتوكول التشفير</h6>
                <div class="d-flex flex-wrap align-items-center gap-2 fs-8">
                    <span class="text-muted">البروتوكول: <code class="text-info"><?= htmlspecialchars($nas_status['protocol'] ?? 'RadSec RFC 6614 (mTLS :2083)') ?></code></span>
                    <span class="text-secondary">•</span>
                    <span class="text-muted">الاستجابة: <strong class="text-success"><?= htmlspecialchars($nas_status['latency_ms'] ?? 1) ?> ms</strong></span>
                    <?php if (!empty($nas_status['winbox_address'])): ?>
                    <span class="text-secondary">•</span>
                    <span class="text-muted">Winbox: <code class="text-warning"><?= htmlspecialchars($nas_status['winbox_address']) ?></code></span>
                    <?php endif; ?>
                </div>
            </div>
        </div>
        <div>
            <?php if (!empty($nas_status['connected'])): ?>
            <span class="badge bg-success px-4 py-2 fs-7 rounded-pill"><i class="fa-solid fa-circle-check me-1"></i> الراوتر متصل ومستقر</span>
            <?php else: ?>
            <span class="badge bg-danger px-4 py-2 fs-7 rounded-pill"><i class="fa-solid fa-triangle-exclamation me-1"></i> تعذر الاتصال بالراوتر</span>
            <?php endif; ?>
        </div>
    </div>
</div>

<!-- Bottom Section: Recent Users & Active Sessions Tables -->
<div class="row g-4">
    <!-- Recent Subscribers -->
    <div class="col-12 col-lg-7">
        <div class="card glass-card border-0 shadow-sm rounded-4 p-4 h-100">
            <div class="d-flex justify-content-between align-items-center mb-3">
                <h5 class="fw-bold text-light mb-0"><i class="fa-solid fa-users me-2 text-primary"></i> أحدث المشتركين</h5>
                <a href="/users" class="btn btn-sm btn-outline-primary rounded-pill px-3">عرض الكل</a>
            </div>
            <div class="table-responsive">
                <table class="table table-dark table-hover align-middle mb-0">
                    <thead class="table-secondary">
                        <tr>
                            <th>المشترك</th>
                            <th>الباقة</th>
                            <th>الصلاحية</th>
                            <th>الحالة</th>
                        </tr>
                    </thead>
                    <tbody>
                        <?php if (empty($recent_users)): ?>
                        <tr><td colspan="4" class="text-center py-4 text-muted">لا يوجد مشتركون حالياً</td></tr>
                        <?php else: ?>
                        <?php foreach ($recent_users as $u): ?>
                        <?php
                            $uName = $u['user'] ?? $u['username'] ?? '';
                            $isExp = !empty($u['expired']);
                            $isEnabled = !isset($u['enabled']) || !empty($u['enabled']);
                            $isActive = !$isExp && $isEnabled;
                        ?>
                        <tr>
                            <td>
                                <strong class="text-light font-monospace fs-7">@<?= htmlspecialchars($uName) ?></strong>
                                <?php if (!empty($u['full_name'])): ?>
                                <small class="text-muted d-block fs-8"><?= htmlspecialchars($u['full_name']) ?></small>
                                <?php endif; ?>
                            </td>
                            <td><span class="badge bg-secondary"><?= htmlspecialchars($u['profile'] ?? $u['profile_name'] ?? 'افتراضي') ?></span></td>
                            <td class="fs-8 text-light font-monospace"><?= htmlspecialchars($u['expires_at'] ?? $u['expiration'] ?? '—') ?></td>
                            <td>
                                <?php if ($isActive): ?>
                                <span class="badge bg-success-subtle text-success border border-success px-2 py-1"><i class="fa-solid fa-circle-check me-1"></i> نشط</span>
                                <?php else: ?>
                                <span class="badge bg-danger-subtle text-danger border border-danger px-2 py-1"><i class="fa-solid fa-circle-xmark me-1"></i> منتهي</span>
                                <?php endif; ?>
                            </td>
                        </tr>
                        <?php endforeach; ?>
                        <?php endif; ?>
                    </tbody>
                </table>
            </div>
        </div>
    </div>

    <!-- Active Live Sessions -->
    <div class="col-12 col-lg-5">
        <div class="card glass-card border-0 shadow-sm rounded-4 p-4 h-100">
            <div class="d-flex justify-content-between align-items-center mb-3">
                <h5 class="fw-bold text-light mb-0"><i class="fa-solid fa-bolt me-2 text-warning"></i> الجلسات المتصلة لحظياً</h5>
                <a href="/sessions" class="btn btn-sm btn-outline-warning rounded-pill px-3">عرض الجلسات</a>
            </div>
            <div class="table-responsive">
                <table class="table table-dark table-hover align-middle mb-0">
                    <thead class="table-secondary">
                        <tr>
                            <th>المستخدم</th>
                            <th>IP</th>
                            <th>المدة</th>
                        </tr>
                    </thead>
                    <tbody>
                        <?php if (empty($sessions)): ?>
                        <tr><td colspan="3" class="text-center py-4 text-muted">لا توجد جلسات نشطة حالياً</td></tr>
                        <?php else: ?>
                        <?php foreach ($sessions as $s): ?>
                        <tr>
                            <td><span class="font-monospace text-info fw-bold">@<?= htmlspecialchars($s['username'] ?? '') ?></span></td>
                            <td><code class="text-light fs-8"><?= htmlspecialchars($s['ip'] ?? $s['framed_ip_address'] ?? '—') ?></code></td>
                            <td class="text-muted fs-8"><?= htmlspecialchars($s['uptime'] ?? '—') ?></td>
                        </tr>
                        <?php endforeach; ?>
                        <?php endif; ?>
                    </tbody>
                </table>
            </div>
        </div>
    </div>
</div>

<style>
/* ============================================================
   PRO FUTURISTIC SAS METRIC CARDS SYSTEM
============================================================ */
.sas-pro-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: 22px;
}

.sas-card-pro {
    position: relative;
    border-radius: 20px;
    padding: 24px;
    color: #ffffff !important;
    overflow: hidden;
    min-height: 185px;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.35);
    border: 1px solid rgba(255, 255, 255, 0.16);
    transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
    cursor: pointer;
    text-decoration: none;
}

.sas-card-pro:hover {
    transform: translateY(-6px) scale(1.015);
    box-shadow: 0 20px 40px -10px rgba(0, 0, 0, 0.5) !important;
    border-color: rgba(255, 255, 255, 0.35);
}

/* Internal Glass Glow Top-Left */
.card-glass-glow {
    position: absolute;
    top: -50%;
    right: -20%;
    width: 200px;
    height: 200px;
    background: radial-gradient(circle, rgba(255, 255, 255, 0.15) 0%, rgba(255, 255, 255, 0) 70%);
    pointer-events: none;
    border-radius: 50%;
}

/* 6 Iconic Gradient Colors */
.card-total {
    background: linear-gradient(135deg, #37474f 0%, #455a64 50%, #546e7a 100%);
}
.card-active {
    background: linear-gradient(135deg, #137752 0%, #218c74 50%, #2ecc71 100%);
}
.card-online {
    background: linear-gradient(135deg, #1f618d 0%, #2980b9 50%, #3498db 100%);
}
.card-expired {
    background: linear-gradient(135deg, #c0392b 0%, #e74c3c 50%, #fc5c65 100%);
}
.card-warning {
    background: linear-gradient(135deg, #d35400 0%, #e67e22 50%, #f1c40f 100%);
}
.card-wallet {
    background: linear-gradient(135deg, #283593 0%, #3949ab 50%, #3f51b5 100%);
}

/* Card Header & Badge */
.card-badge-icon {
    width: 40px;
    height: 40px;
    border-radius: 12px;
    background: rgba(255, 255, 255, 0.18);
    backdrop-filter: blur(10px);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.15rem;
    color: #ffffff;
    box-shadow: 0 4px 10px rgba(0, 0, 0, 0.15);
    border: 1px solid rgba(255, 255, 255, 0.2);
}

.card-arrow-link {
    width: 28px;
    height: 28px;
    border-radius: 50%;
    background: rgba(255, 255, 255, 0.1);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.75rem;
    color: rgba(255, 255, 255, 0.7);
    transition: transform 0.25s ease, background 0.25s ease;
}

.sas-card-pro:hover .card-arrow-link {
    transform: translate(-3px, -3px);
    background: rgba(255, 255, 255, 0.3);
    color: #ffffff;
}

.card-title {
    font-size: 1.15rem;
    font-weight: 800;
    margin: 0;
    letter-spacing: -0.2px;
    color: #ffffff;
}

.card-sub-label {
    font-size: 0.8rem;
    color: rgba(255, 255, 255, 0.8);
    margin-top: 3px;
    font-weight: 500;
}

/* Card Value & Footer */
.card-content-bottom {
    margin-top: 18px;
    position: relative;
    z-index: 2;
}

.card-number-val {
    font-size: 2.7rem;
    font-weight: 900;
    line-height: 1;
    color: #ffffff;
    font-family: 'Consolas', 'SF Mono', monospace;
    letter-spacing: -1px;
    text-shadow: 0 2px 8px rgba(0, 0, 0, 0.25);
}

.card-footer-pill {
    display: inline-flex;
    align-items: center;
    margin-top: 10px;
    padding: 4px 12px;
    border-radius: 30px;
    background: rgba(0, 0, 0, 0.18);
    backdrop-filter: blur(8px);
    border: 1px solid rgba(255, 255, 255, 0.12);
    font-size: 0.75rem;
    font-weight: 600;
    color: rgba(255, 255, 255, 0.9);
}

/* Neon Pulse Dot for Active Users */
.active-pulse-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background-color: #00ff88;
    margin-left: 6px;
    box-shadow: 0 0 8px #00ff88;
    animation: pulseGlow 1.8s infinite;
}

@keyframes pulseGlow {
    0% { transform: scale(0.9); opacity: 0.7; }
    50% { transform: scale(1.3); opacity: 1; box-shadow: 0 0 12px #00ff88; }
    100% { transform: scale(0.9); opacity: 0.7; }
}

/* Watermark Icon */
.watermark-icon-pro {
    position: absolute;
    bottom: -15px;
    left: -15px;
    font-size: 6rem;
    opacity: 0.12;
    color: #ffffff;
    pointer-events: none;
    transition: transform 0.35s cubic-bezier(0.16, 1, 0.3, 1), opacity 0.35s ease;
}

.sas-card-pro:hover .watermark-icon-pro {
    transform: scale(1.18) rotate(-6deg);
    opacity: 0.22;
}
</style>

<script>
setInterval(() => {
    const clockEl = document.getElementById('liveClock');
    if (clockEl) {
        const d = new Date();
        clockEl.innerText = d.getFullYear() + '-' + 
            String(d.getMonth()+1).padStart(2, '0') + '-' + 
            String(d.getDate()).padStart(2, '0') + ' ' + 
            String(d.getHours()).padStart(2, '0') + ':' + 
            String(d.getMinutes()).padStart(2, '0') + ':' + 
            String(d.getSeconds()).padStart(2, '0');
    }
}, 1000);
</script>
