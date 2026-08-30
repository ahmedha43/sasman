<?php
$isLoggedIn = !empty($username) && !empty($status) && !empty($status['username']);
$isAuthenticated = !empty($is_auth);
$isActive = ($status['status'] ?? '') === 'نشط' || ($status['status'] ?? '') === 'active' || (!empty($status['expires_at']) && strtotime($status['expires_at']) > time());

function formatBytesHuman($bytes) {
    if ($bytes <= 0) return '0 B';
    $units = ['B', 'KB', 'MB', 'GB', 'TB'];
    $bytes = max($bytes, 0);
    $pow = floor(($bytes ? log($bytes) : 0) / log(1024));
    $pow = min($pow, count($units) - 1);
    $bytes /= pow(1024, $pow);
    return round($bytes, 2) . ' ' . $units[$pow];
}

$downloadBytes = (int)($status['usage_in'] ?? $status['session']['download_bytes'] ?? 0);
$uploadBytes = (int)($status['usage_out'] ?? $status['session']['upload_bytes'] ?? 0);
$totalBytes = (int)($status['usage_total'] ?? ($downloadBytes + $uploadBytes));
$balance = (float)($status['balance'] ?? 0);
$expiryStr = $status['expires_at'] ?? $status['expiry'] ?? '—';
$daysLeft = null;
if ($expiryStr && $expiryStr !== '—') {
    $expTs = strtotime($expiryStr);
    if ($expTs > 0) {
        $daysLeft = ceil(($expTs - time()) / 86400);
    }
}
?>

<?php if (!$isLoggedIn): ?>
<!-- ============================================================
   1. UNAUTHENTICATED PORTAL: LOGIN & QUICK LOOKUP
============================================================ -->
<div class="row justify-content-center">
    <div class="col-12 col-md-8 col-lg-6">
        <div class="card glass-card border-0 shadow-2xl rounded-4 p-4 p-sm-5 text-center position-relative overflow-hidden pro-portal-card">
            <div class="portal-card-glow"></div>

            <div class="mb-4 text-center position-relative z-2">
                <div class="portal-icon-box d-inline-flex p-3 mb-3 shadow rounded-4">
                    <i class="fa-solid fa-satellite-dish fs-2 text-white"></i>
                </div>
                <h4 class="fw-bold text-light mb-1 font-monospace">بوابة خدمة المشتركين</h4>
                <p class="text-white-50 fs-8 mb-0">استعلام الرصيد، شحن الكروت، ومتابعة الصلاحية والاستهلاك</p>
            </div>

            <!-- Tab Switcher -->
            <ul class="nav nav-pills nav-fill gap-2 mb-4 bg-dark bg-opacity-75 p-1 rounded-pill border border-secondary border-opacity-50 position-relative z-2" id="portalAuthTabs" role="tablist">
                <li class="nav-item" role="presentation">
                    <button class="nav-link active rounded-pill fw-bold fs-8" data-bs-toggle="pill" data-bs-target="#login-pane" type="button">
                        <i class="fa-solid fa-key me-1"></i> تسجيل الدخول
                    </button>
                </li>
                <li class="nav-item" role="presentation">
                    <button class="nav-link rounded-pill fw-bold fs-8" data-bs-toggle="pill" data-bs-target="#lookup-pane" type="button">
                        <i class="fa-solid fa-magnifying-glass me-1"></i> استعلام سريع
                    </button>
                </li>
                <li class="nav-item" role="presentation">
                    <button class="nav-link rounded-pill fw-bold fs-8" data-bs-toggle="pill" data-bs-target="#direct-redeem-pane" type="button">
                        <i class="fa-solid fa-bolt me-1 text-warning"></i> شحن كارت
                    </button>
                </li>
            </ul>

            <div class="tab-content position-relative z-2" id="portalAuthTabsContent">
                <!-- 1. Full Login Pane -->
                <div class="tab-pane fade show active" id="login-pane" role="tabpanel">
                    <form action="/portal/login" method="POST">
                        <div class="mb-3 text-start">
                            <label class="form-label fs-8 text-light fw-semibold">اسم المستخدم (Username)</label>
                            <div class="input-group">
                                <span class="input-group-text bg-dark border-secondary text-info"><i class="fa-solid fa-user"></i></span>
                                <input type="text" name="username" class="form-control bg-dark border-secondary text-light font-monospace py-2" required placeholder="مثال: user123" autofocus>
                            </div>
                        </div>
                        <div class="mb-4 text-start">
                            <label class="form-label fs-8 text-light fw-semibold">كلمة المرور (Password)</label>
                            <div class="input-group">
                                <span class="input-group-text bg-dark border-secondary text-warning"><i class="fa-solid fa-lock"></i></span>
                                <input type="password" name="password" class="form-control bg-dark border-secondary text-light font-monospace py-2" required placeholder="••••••••">
                            </div>
                        </div>
                        <button type="submit" class="btn btn-primary w-100 py-3 rounded-pill fw-bold shadow-lg pro-portal-btn">
                            <i class="fa-solid fa-right-to-bracket me-2"></i> دخول لحسابي
                        </button>
                    </form>
                </div>

                <!-- 2. Quick Lookup Pane (Read-Only) -->
                <div class="tab-pane fade" id="lookup-pane" role="tabpanel">
                    <form action="/portal" method="GET">
                        <div class="mb-4 text-start">
                            <label class="form-label fs-8 text-light fw-semibold">اسم المشترك للبحث والاستعلام</label>
                            <div class="input-group">
                                <span class="input-group-text bg-dark border-secondary text-info"><i class="fa-solid fa-magnifying-glass"></i></span>
                                <input type="text" name="username" class="form-control bg-dark border-secondary text-light text-center fs-6 font-monospace py-2" required placeholder="username">
                            </div>
                            <small class="text-white-50 fs-9 mt-1 d-block"><i class="fa-solid fa-circle-info me-1 text-info"></i> عرض الرصيد والصلاحية والاستهلاك بشكل آمن (للقراءة فقط)</small>
                        </div>
                        <button type="submit" class="btn btn-outline-info w-100 py-3 rounded-pill fw-bold">
                            <i class="fa-solid fa-bolt me-2"></i> فحص الصلاحية والرصيد الآن
                        </button>
                    </form>
                </div>

                <!-- 3. Direct Voucher Redeem Pane -->
                <div class="tab-pane fade" id="direct-redeem-pane" role="tabpanel">
                    <form action="/portal/redeem" method="POST">
                        <div class="mb-3 text-start">
                            <label class="form-label fs-8 text-light fw-semibold">اسم المستخدم (User)</label>
                            <div class="input-group">
                                <span class="input-group-text bg-dark border-secondary text-info"><i class="fa-solid fa-user"></i></span>
                                <input type="text" name="username" class="form-control bg-dark border-secondary text-light font-monospace" required placeholder="username">
                            </div>
                        </div>
                        <div class="mb-4 text-start">
                            <label class="form-label fs-8 text-light fw-semibold">رمز كارت الشحن (PIN Code)</label>
                            <div class="input-group">
                                <span class="input-group-text bg-dark border-secondary text-warning"><i class="fa-solid fa-ticket"></i></span>
                                <input type="text" name="code" class="form-control bg-dark border-secondary text-warning font-monospace text-center fs-5 fw-bold" required placeholder="00000000">
                            </div>
                        </div>
                        <button type="submit" class="btn btn-warning text-dark w-100 py-3 rounded-pill fw-bold shadow-lg">
                            <i class="fa-solid fa-wand-magic-sparkles me-2"></i> شحن وتفعيل الاشتراك فوراً
                        </button>
                    </form>
                </div>
            </div>
        </div>
    </div>
</div>

<?php else: ?>
<!-- ============================================================
   2. SUBSCRIBER DASHBOARD (AUTHENTICATED OR QUICK LOOKUP)
============================================================ -->
<div class="row justify-content-center">
    <div class="col-12">

        <!-- Security Banner for Quick Lookup Mode -->
        <?php if (!$isAuthenticated): ?>
        <div class="alert alert-info border-0 rounded-4 shadow-sm p-3 mb-4 d-flex align-items-center justify-content-between flex-wrap gap-2" style="background: rgba(14, 165, 233, 0.15); border: 1px solid rgba(14, 165, 233, 0.3) !important;">
            <div class="d-flex align-items-center gap-2">
                <i class="fa-solid fa-shield-halved fs-4 text-info"></i>
                <div>
                    <span class="fw-bold text-light fs-8 d-block">أنت في وضع الاستعلام السريع (للقراءة فقط)</span>
                    <span class="text-white-50 fs-9">لتعديل الحساب وتغيير كلمة المرور، يرجى تسجيل الدخول بكلمة المرور.</span>
                </div>
            </div>
            <a href="/portal" class="btn btn-sm btn-info text-dark rounded-pill px-3 fw-bold fs-9">
                <i class="fa-solid fa-right-to-bracket me-1"></i> تسجيل الدخول الكامل
            </a>
        </div>
        <?php endif; ?>
        
        <!-- Hero Subscriber Header Box -->
        <div class="card glass-card border-0 shadow-lg rounded-4 p-4 mb-4 position-relative overflow-hidden" style="background: linear-gradient(135deg, #1e1b4b 0%, #0f172a 100%); border: 1px solid rgba(139, 92, 246, 0.3) !important;">
            <div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 position-relative z-2">
                <div class="d-flex align-items-center gap-3">
                    <div class="user-avatar-circle" style="width: 52px; height: 52px; font-size: 1.4rem; background: linear-gradient(135deg, #8b5cf6 0%, #6366f1 100%);">
                        <?= mb_strtoupper(mb_substr($status['username'] ?? 'U', 0, 1)) ?>
                    </div>
                    <div>
                        <div class="d-flex align-items-center gap-2">
                            <h4 class="fw-bold text-light mb-0 font-monospace">@<?= htmlspecialchars($status['username']) ?></h4>
                            <?php if ($isActive): ?>
                            <span class="badge bg-success-subtle text-success border border-success px-3 py-1 fs-9"><i class="fa-solid fa-circle-check me-1"></i> الاشتراك فعال</span>
                            <?php else: ?>
                            <span class="badge bg-danger-subtle text-danger border border-danger px-3 py-1 fs-9"><i class="fa-solid fa-circle-xmark me-1"></i> الاشتراك منتهي</span>
                            <?php endif; ?>
                        </div>
                        <span class="text-white-50 fs-8 d-block mt-1"><?= htmlspecialchars($status['full_name'] ?: 'مشترك في الشبكة') ?></span>
                    </div>
                </div>
                <div class="d-flex align-items-center gap-2">
                    <?php if ($isAuthenticated): ?>
                    <a href="/portal/logout" class="btn btn-outline-danger btn-sm rounded-pill px-4 fw-bold">
                        <i class="fa-solid fa-power-off me-1"></i> تسجيل الخروج
                    </a>
                    <?php else: ?>
                    <a href="/portal" class="btn btn-outline-light btn-sm rounded-pill px-4 fw-bold">
                        <i class="fa-solid fa-arrow-right me-1"></i> رجوع للرئيسية
                    </a>
                    <?php endif; ?>
                </div>
            </div>
        </div>

        <!-- 4 Pro KPI Metric Cards -->
        <div class="row g-3 mb-4">
            <!-- Card 1: Package -->
            <div class="col-6 col-lg-3">
                <div class="p-3 rounded-4 border border-primary border-opacity-50 text-center h-100 kpi-card-portal" style="background: rgba(59, 130, 246, 0.12);">
                    <i class="fa-solid fa-gauge-high text-primary fs-3 mb-2"></i>
                    <span class="text-muted fs-9 d-block mb-1">الباقة الحالية</span>
                    <h5 class="fw-bold text-light mb-0 font-monospace text-truncate"><?= htmlspecialchars($status['profile'] ?? 'Standard') ?></h5>
                </div>
            </div>

            <!-- Card 2: Expiry & Days Left -->
            <div class="col-6 col-lg-3">
                <div class="p-3 rounded-4 border border-warning border-opacity-50 text-center h-100 kpi-card-portal" style="background: rgba(245, 158, 11, 0.12);">
                    <i class="fa-solid fa-calendar-check text-warning fs-3 mb-2"></i>
                    <span class="text-muted fs-9 d-block mb-1">تاريخ الانتهاء</span>
                    <h6 class="fw-bold text-warning mb-0 fs-8 font-monospace"><?= htmlspecialchars($expiryStr) ?></h6>
                    <?php if ($daysLeft !== null): ?>
                    <small class="text-white-50 fs-9 d-block mt-1"><?= $daysLeft > 0 ? "باقي {$daysLeft} يوم" : "منتهي منذ " . abs($daysLeft) . " يوم" ?></small>
                    <?php endif; ?>
                </div>
            </div>

            <!-- Card 3: Data Consumption -->
            <div class="col-6 col-lg-3">
                <div class="p-3 rounded-4 border border-success border-opacity-50 text-center h-100 kpi-card-portal" style="background: rgba(16, 185, 129, 0.12);">
                    <i class="fa-solid fa-chart-pie text-success fs-3 mb-2"></i>
                    <span class="text-muted fs-9 d-block mb-1">إجمالي استهلاك البيانات</span>
                    <h5 class="fw-bold text-success mb-0 fs-7 font-monospace"><?= formatBytesHuman($totalBytes) ?></h5>
                    <small class="text-white-50 fs-9 d-block mt-1">⬇️ <?= formatBytesHuman($downloadBytes) ?> | ⬆️ <?= formatBytesHuman($uploadBytes) ?></small>
                </div>
            </div>

            <!-- Card 4: Balance / Debt -->
            <div class="col-6 col-lg-3">
                <div class="p-3 rounded-4 border border-info border-opacity-50 text-center h-100 kpi-card-portal" style="background: rgba(6, 182, 212, 0.12);">
                    <i class="fa-solid fa-wallet <?= $balance > 0 ? 'text-danger' : 'text-info' ?> fs-3 mb-2"></i>
                    <span class="text-muted fs-9 d-block mb-1">الرصيد / الدين</span>
                    <?php if ($balance > 0): ?>
                    <h6 class="fw-bold text-danger mb-0 fs-8 font-monospace">مطلوب <?= number_format($balance, 0) ?> د.ع</h6>
                    <?php else: ?>
                    <h6 class="fw-bold text-info mb-0 fs-8 font-monospace">خالص (0 د.ع)</h6>
                    <?php endif; ?>
                </div>
            </div>
        </div>

        <!-- Portal Tabs Navigation -->
        <ul class="nav nav-pills gap-2 mb-3 bg-dark bg-opacity-75 p-2 rounded-4 border border-secondary border-opacity-50" id="portalServiceTabs" role="tablist">
            <li class="nav-item" role="presentation">
                <button class="nav-link active fw-bold rounded-pill px-4 fs-8" data-bs-toggle="pill" data-bs-target="#tab-recharge" type="button">
                    <i class="fa-solid fa-ticket me-2 text-warning"></i> شحن كارت اشتراك
                </button>
            </li>
            <li class="nav-item" role="presentation">
                <button class="nav-link fw-bold rounded-pill px-4 fs-8" data-bs-toggle="pill" data-bs-target="#tab-streams" type="button">
                    <i class="fa-solid fa-tv me-2 text-danger"></i> قنوات البث والمباريات
                </button>
            </li>
            <li class="nav-item" role="presentation">
                <button class="nav-link fw-bold rounded-pill px-4 fs-8 <?= !$isAuthenticated ? 'text-white-50' : '' ?>" data-bs-toggle="pill" data-bs-target="#tab-password" type="button">
                    <i class="fa-solid <?= $isAuthenticated ? 'fa-lock text-info' : 'fa-lock text-secondary' ?> me-2"></i> تغيير كلمة المرور
                    <?php if (!$isAuthenticated): ?><span class="badge bg-secondary ms-1 fs-9">مقفل 🔒</span><?php endif; ?>
                </button>
            </li>
        </ul>

        <div class="tab-content" id="portalServiceTabsContent">
            <!-- TAB 1: RECHARGE VOUCHER -->
            <div class="tab-pane fade show active" id="tab-recharge" role="tabpanel">
                <div class="card glass-card border-0 shadow-sm rounded-4 p-4">
                    <h5 class="fw-bold text-light mb-2"><i class="fa-solid fa-wand-magic-sparkles me-2 text-warning"></i> شحن فوري برمز الكارت (PIN Code)</h5>
                    <p class="text-muted fs-8 mb-4">أدخل كود كارت الشحن المكون من أرقام لتجديد اشتراكك فوراً في المايكروتك دون انقطاع</p>

                    <form action="/portal/redeem" method="POST">
                        <input type="hidden" name="username" value="<?= htmlspecialchars($status['username']) ?>">
                        <div class="mb-4">
                            <label class="form-label fs-8 text-light fw-semibold">رمز كارت الشحن (PIN Code) *</label>
                            <input type="text" name="code" class="form-control bg-dark border-warning text-warning text-center font-monospace fs-2 fw-bold" required placeholder="00000000" style="letter-spacing: 4px;">
                        </div>
                        <button type="submit" class="btn btn-warning w-100 py-3 rounded-pill fw-bold fs-6 shadow text-dark">
                            <i class="fa-solid fa-bolt me-2"></i> شحن وتجديد الاشتراك الآن
                        </button>
                    </form>
                </div>
            </div>

            <!-- TAB 2: IPTV STREAMS -->
            <div class="tab-pane fade" id="tab-streams" role="tabpanel">
                <div class="card glass-card border-0 shadow-sm rounded-4 p-4">
                    <h5 class="fw-bold text-light mb-3"><i class="fa-solid fa-tv me-2 text-danger"></i> قنوات البث المباشر والمباريات</h5>
                    
                    <?php if (empty($streams)): ?>
                    <div class="text-center py-5 text-muted">
                        <i class="fa-solid fa-tv fs-1 d-block mb-3 text-secondary opacity-50"></i>
                        <h6>لا توجد قنوات بث متاحة حالياً</h6>
                    </div>
                    <?php else: ?>
                    <div class="row g-3">
                        <?php foreach ($streams as $s): ?>
                        <div class="col-12 col-md-6">
                            <div class="card bg-dark border-secondary rounded-4 p-3 d-flex flex-row align-items-center justify-content-between shadow-sm">
                                <div class="d-flex align-items-center gap-3">
                                    <div class="bg-danger text-white p-2 rounded-3">
                                        <i class="fa-solid fa-play"></i>
                                    </div>
                                    <span class="fw-bold text-light fs-7"><?= htmlspecialchars($s['name'] ?? 'قناة بث') ?></span>
                                </div>
                                <button type="button" class="btn btn-sm btn-outline-danger rounded-pill px-3" onclick="playPortalVideo('<?= htmlspecialchars($s['name'] ?? '', ENT_QUOTES) ?>', '<?= htmlspecialchars($s['source'] ?? $s['url'] ?? '', ENT_QUOTES) ?>')">
                                    <i class="fa-solid fa-play me-1"></i> مشاهدة
                                </button>
                            </div>
                        </div>
                        <?php endforeach; ?>
                    </div>
                    <?php endif; ?>
                </div>
            </div>

            <!-- TAB 3: CHANGE PASSWORD (STRICTLY AUTHENTICATED ONLY) -->
            <div class="tab-pane fade" id="tab-password" role="tabpanel">
                <div class="card glass-card border-0 shadow-sm rounded-4 p-4">
                    <?php if ($isAuthenticated): ?>
                    <h5 class="fw-bold text-light mb-3"><i class="fa-solid fa-lock me-2 text-info"></i> تغيير كلمة مرور حسابك</h5>
                    <form action="/portal/password" method="POST">
                        <div class="mb-3">
                            <label class="form-label fs-8 text-light fw-semibold">كلمة المرور الحالية *</label>
                            <input type="password" name="old_password" class="form-control bg-dark border-secondary text-light font-monospace py-2" required placeholder="••••••••">
                        </div>
                        <div class="mb-4">
                            <label class="form-label fs-8 text-light fw-semibold">كلمة المرور الجديدة *</label>
                            <input type="password" name="new_password" class="form-control bg-dark border-secondary text-light font-monospace py-2" required placeholder="••••••••">
                        </div>
                        <button type="submit" class="btn btn-info text-dark px-4 py-2 rounded-pill fw-bold">
                            <i class="fa-solid fa-key me-2"></i> حفظ وتحديث كلمة المرور
                        </button>
                    </form>
                    <?php else: ?>
                    <!-- Security Lock Screen for Unauthenticated Visitors -->
                    <div class="text-center py-4">
                        <div class="bg-warning bg-opacity-10 text-warning rounded-circle d-inline-flex p-3 mb-3 border border-warning border-opacity-25">
                            <i class="fa-solid fa-shield-halved fs-1"></i>
                        </div>
                        <h5 class="fw-bold text-light mb-2">خاصية محمية أمنياً 🔒</h5>
                        <p class="text-white-50 fs-8 mb-4" style="max-width: 420px; margin: 0 auto;">
                            لتغيير كلمة مرور الحساب، يلزم تسجيل الدخول الكامل بكلمة المرور لمنع التعديل غير المصرح به على بيانات المشترك.
                        </p>
                        <a href="/portal" class="btn btn-primary rounded-pill px-4 py-2 fw-bold fs-8 shadow-sm">
                            <i class="fa-solid fa-right-to-bracket me-2"></i> تسجيل الدخول لتغيير كلمة المرور
                        </a>
                    </div>
                    <?php endif; ?>
                </div>
            </div>
        </div>

    </div>
</div>

<!-- Video Player Modal -->
<div class="modal fade" id="portalVideoModal" tabindex="-1" aria-hidden="true">
    <div class="modal-dialog modal-dialog-centered modal-lg">
        <div class="modal-content glass-card border-danger text-light">
            <div class="modal-header border-secondary">
                <h5 class="modal-title fw-bold text-danger" id="portalVideoTitle"><i class="fa-solid fa-tv me-2"></i> البث المباشر</h5>
                <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" aria-label="Close" onclick="stopPortalVideo()"></button>
            </div>
            <div class="modal-body p-0 bg-black text-center" style="min-height: 380px;">
                <video id="portalVideoPlayer" controls autoplay class="w-100" style="max-height: 480px;"></video>
            </div>
        </div>
    </div>
</div>

<style>
/* ============================================================
   PRO PORTAL STYLING
============================================================ */
.pro-portal-card {
    background: rgba(15, 23, 42, 0.8) !important;
    backdrop-filter: blur(20px);
    border: 1px solid rgba(255, 255, 255, 0.12) !important;
}

.portal-card-glow {
    position: absolute;
    top: -40%;
    right: -30%;
    width: 260px;
    height: 260px;
    background: radial-gradient(circle, rgba(139, 92, 246, 0.2) 0%, rgba(139, 92, 246, 0) 70%);
    pointer-events: none;
    border-radius: 50%;
}

.portal-icon-box {
    background: linear-gradient(135deg, #8b5cf6 0%, #6366f1 100%);
    border: 1px solid rgba(255, 255, 255, 0.25);
    width: 68px;
    height: 68px;
    align-items: center;
    justify-content: center;
}

.pro-portal-btn {
    background: linear-gradient(135deg, #8b5cf6 0%, #6366f1 100%);
    border: none;
}
.pro-portal-btn:hover {
    background: linear-gradient(135deg, #a78bfa 0%, #4f46e5 100%);
    box-shadow: 0 10px 20px rgba(139, 92, 246, 0.35) !important;
}

.kpi-card-portal {
    transition: transform 0.2s ease;
}
.kpi-card-portal:hover {
    transform: translateY(-3px);
}
</style>

<script>
function playPortalVideo(name, url) {
    document.getElementById('portalVideoTitle').innerText = name;
    const video = document.getElementById('portalVideoPlayer');
    video.src = url;
    new bootstrap.Modal(document.getElementById('portalVideoModal')).show();
}

function stopPortalVideo() {
    const video = document.getElementById('portalVideoPlayer');
    video.pause();
    video.src = '';
}
</script>
<?php endif; ?>
