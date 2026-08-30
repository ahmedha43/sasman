<header class="navbar navbar-expand-lg border-bottom border-dark-subtle px-4 py-3 glass-header">
    <div class="container-fluid p-0 d-flex justify-content-between align-items-center">
        <div class="d-flex align-items-center gap-3">
            <button class="btn btn-outline-secondary d-md-none" type="button" id="sidebarToggle">
                <i class="fa-solid fa-bars"></i>
            </button>
            <h5 class="mb-0 fw-bold text-light"><?= htmlspecialchars($title ?? 'SASMAN') ?></h5>
        </div>

        <div class="d-flex align-items-center gap-2">
            <!-- Account Role Badge -->
            <?php if (!empty($is_superadmin)): ?>
            <span class="badge bg-primary-subtle text-primary border border-primary px-3 py-2 fs-8 d-none d-sm-inline-flex align-items-center gap-1">
                <i class="fa-solid fa-crown text-warning"></i> مدير النظام (Superadmin)
            </span>
            <?php else: ?>
            <span class="badge bg-warning-subtle text-warning border border-warning px-3 py-2 fs-8 d-none d-sm-inline-flex align-items-center gap-1">
                <i class="fa-solid fa-user-tag"></i> وكيل فرعي: <strong>@<?= htmlspecialchars($current_user['username'] ?? '') ?></strong>
            </span>
            <?php endif; ?>

            <!-- Agent Balance Pill -->
            <?php if (isset($current_user['balance']) && (!empty($is_agent) || (isset($current_user['role']) && $current_user['role'] !== 'superadmin'))): ?>
            <div class="bg-dark px-3 py-2 rounded-pill border border-warning border-opacity-50 text-warning fs-8 font-monospace shadow-sm">
                <i class="fa-solid fa-wallet me-1"></i> <?= number_format((float)$current_user['balance'], 0) ?> <small class="text-white-50 fs-9">د.ع</small>
            </div>
            <?php endif; ?>

            <!-- Portal Link -->
            <a href="/portal" target="_blank" class="btn btn-sm btn-outline-info rounded-pill px-3 fs-8">
                <i class="fa-solid fa-arrow-up-right-from-square me-1"></i> بوابة المشتركين
            </a>

            <!-- Logout Link -->
            <a href="/logout" class="btn btn-sm btn-outline-danger rounded-pill px-3 fs-8 d-none d-sm-inline-block">
                <i class="fa-solid fa-power-off me-1"></i> خروج
            </a>
        </div>
    </div>
</header>
