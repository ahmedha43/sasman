<?php if (!empty($flash_success)): ?>
<div class="alert alert-success alert-dismissible fade show border border-success d-flex align-items-center gap-2 mb-4 shadow-sm" role="alert">
    <i class="fa-solid fa-circle-check fs-5"></i>
    <div><?= htmlspecialchars($flash_success) ?></div>
    <button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button>
</div>
<?php endif; ?>

<?php if (!empty($flash_error)): ?>
<div class="alert alert-danger alert-dismissible fade show border border-danger d-flex align-items-center gap-2 mb-4 shadow-sm" role="alert">
    <i class="fa-solid fa-triangle-exclamation fs-5"></i>
    <div><?= htmlspecialchars($flash_error) ?></div>
    <button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button>
</div>
<?php endif; ?>
