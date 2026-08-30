<?php
$currentUri = parse_url($_SERVER['REQUEST_URI'] ?? '/', PHP_URL_PATH);
function isActive($path, $currentUri) {
    return ($path === '/' && $currentUri === '/') || ($path !== '/' && str_starts_with($currentUri, $path)) ? 'active' : '';
}
$isSuper = !empty($is_superadmin);
$isAg = !empty($is_agent);
$canProf = !empty($can_manage_profiles);
$canNasDev = !empty($can_manage_nas);
?>
<aside class="sidebar glass-sidebar border-end border-dark-subtle d-flex flex-column flex-shrink-0 p-3 text-white" style="width: 280px; min-height: 100vh;">
    <a href="/" class="d-flex align-items-center mb-3 mb-md-0 me-md-auto text-white text-decoration-none px-2">
        <div class="logo-box bg-gradient-primary rounded-3 p-2 me-2 shadow-sm">
            <i class="fa-solid fa-server fs-4 text-white"></i>
        </div>
        <div>
            <span class="fs-5 fw-bold d-block text-gradient">SASMAN</span>
            <small class="text-white-50 fs-8">RADIUS & Network Cloud</small>
        </div>
    </a>

    <!-- Sub-Agent Balance Widget in Sidebar -->
    <?php if ($isAg || (isset($current_user['role']) && $current_user['role'] !== 'superadmin')): ?>
    <div class="p-3 my-2 rounded-4 bg-dark bg-opacity-75 border border-warning border-opacity-50 text-center shadow-sm">
        <div class="d-flex align-items-center justify-content-between mb-1">
            <span class="badge bg-warning-subtle text-warning border border-warning px-2 py-0 fs-9"><i class="fa-solid fa-user-tag me-1"></i> حساب وكيل</span>
            <small class="text-white-50 fs-9">محفظتك</small>
        </div>
        <h5 class="fw-bold text-warning font-monospace mb-0"><?= number_format((float)($current_user['balance'] ?? 0), 0) ?> <small class="fs-9 text-white-50">د.ع</small></h5>
    </div>
    <?php endif; ?>

    <hr class="border-secondary border-opacity-50 my-2">
    
    <ul class="nav nav-pills flex-column mb-auto gap-1">
        <li class="nav-item">
            <a href="/" class="nav-link text-white <?= isActive('/', $currentUri) ?>">
                <i class="fa-solid fa-chart-pie me-2 text-info"></i>
                <span>لوحة المؤشرات</span>
            </a>
        </li>
        <li>
            <a href="/users" class="nav-link text-white <?= isActive('/users', $currentUri) ?>">
                <i class="fa-solid fa-users me-2 text-primary"></i>
                <span>إدارة المشتركين</span>
            </a>
        </li>
        <li>
            <a href="/sessions" class="nav-link text-white <?= isActive('/sessions', $currentUri) ?>">
                <i class="fa-solid fa-tower-broadcast me-2 text-success"></i>
                <span>المتصلون الآن (Live)</span>
            </a>
        </li>
        <li>
            <a href="/vouchers" class="nav-link text-white <?= isActive('/vouchers', $currentUri) ?>">
                <i class="fa-solid fa-ticket me-2 text-warning"></i>
                <span>كروت الشحن والطباعة</span>
            </a>
        </li>
        <li>
            <a href="/profiles" class="nav-link text-white <?= isActive('/profiles', $currentUri) ?>">
                <i class="fa-solid fa-gauge-high me-2 text-info"></i>
                <span>باقات السرعة</span>
                <?php if (!$canProf): ?><span class="badge bg-secondary border border-secondary float-start fs-9 mt-1 opacity-75">عرض</span><?php endif; ?>
            </a>
        </li>

        <li class="nav-header text-uppercase text-muted fs-8 px-3 mt-2 mb-1">الشبكة والمايكروتك</li>
        <li>
            <a href="/nas" class="nav-link text-white <?= isActive('/nas', $currentUri) ?>">
                <i class="fa-solid fa-network-wired me-2 text-cyan"></i>
                <span>المايكروتك و Winbox</span>
                <?php if (!$canNasDev): ?><span class="badge bg-secondary border border-secondary float-start fs-9 mt-1 opacity-75">عرض</span><?php endif; ?>
            </a>
        </li>
        <li>
            <a href="/devices" class="nav-link text-white <?= isActive('/devices', $currentUri) ?>">
                <i class="fa-solid fa-satellite-dish me-2 text-danger"></i>
                <span>الصحونات وأجهزة CPE</span>
            </a>
        </li>

        <li class="nav-header text-uppercase text-muted fs-8 px-3 mt-2 mb-1">المالية والموزعون</li>
        <li>
            <a href="/admins" class="nav-link text-white <?= isActive('/admins', $currentUri) ?>">
                <i class="fa-solid fa-user-gear me-2 text-warning"></i>
                <span>الموزعون والوكلاء</span>
            </a>
        </li>
        <li>
            <a href="/transactions" class="nav-link text-white <?= isActive('/transactions', $currentUri) ?>">
                <i class="fa-solid fa-money-bill-transfer me-2 text-success"></i>
                <span>السجل المالي والأرباح</span>
            </a>
        </li>

        <li class="nav-header text-uppercase text-muted fs-8 px-3 mt-2 mb-1">التواصل والخدمات</li>
        <li>
            <a href="/whatsapp" class="nav-link text-white <?= isActive('/whatsapp', $currentUri) ?>">
                <i class="fa-brands fa-whatsapp me-2 text-success"></i>
                <span>إشعارات الواتساب</span>
            </a>
        </li>
        <li>
            <a href="/streams" class="nav-link text-white <?= isActive('/streams', $currentUri) ?>">
                <i class="fa-solid fa-tv me-2 text-purple"></i>
                <span>قنوات البث (IPTV)</span>
            </a>
        </li>
        <?php if (!empty($perms['can_view_logs']) || $isSuper): ?>
        <li>
            <a href="/logs" class="nav-link text-white <?= isActive('/logs', $currentUri) ?>">
                <i class="fa-solid fa-list-check me-2 text-muted"></i>
                <span>سجل العمليات (Audit)</span>
            </a>
        </li>
        <?php endif; ?>
        
        <?php if ($isSuper): ?>
        <li>
            <a href="/settings" class="nav-link text-white <?= isActive('/settings', $currentUri) ?>">
                <i class="fa-solid fa-gears me-2 text-warning"></i>
                <span>إعدادات النظام والنسخ</span>
            </a>
        </li>
        <?php endif; ?>
    </ul>
    
    <hr class="border-secondary border-opacity-50 my-2">
    <div class="d-flex align-items-center justify-content-between px-2 pt-1">
        <div class="d-flex align-items-center gap-2">
            <div class="user-avatar-circle" style="width: 32px; height: 32px; font-size: 0.85rem; background: linear-gradient(135deg, #0ea5e9 0%, #2563eb 100%);">
                <?= mb_strtoupper(mb_substr($current_user['username'] ?? 'A', 0, 1)) ?>
            </div>
            <div>
                <span class="d-block fs-8 fw-bold text-truncate" style="max-width: 110px;">@<?= htmlspecialchars($current_user['username'] ?? 'admin') ?></span>
                <span class="badge <?= $isSuper ? 'bg-primary-subtle text-primary' : 'bg-warning-subtle text-warning' ?> fs-9 py-0"><?= $isSuper ? 'مدير عام' : 'وكيل فرعي' ?></span>
            </div>
        </div>
        <a href="/logout" class="btn btn-sm btn-outline-danger rounded-circle p-2" title="تسجيل الخروج">
            <i class="fa-solid fa-power-off fs-8"></i>
        </a>
    </div>
</aside>
