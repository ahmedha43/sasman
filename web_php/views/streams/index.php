<div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
    <div>
        <h4 class="fw-bold text-light mb-1"><i class="fa-solid fa-tv me-2 text-danger"></i> قنوات البث المباشر المحلي (IPTV Live Streams)</h4>
        <p class="text-muted mb-0 fs-7">إدارة قنوات البث المباشر المحلية، مباريات كرة القدم، وإتاحتها لمشتركي الشبكة في البوابة</p>
    </div>
    <button class="btn btn-danger rounded-pill px-4 fw-bold shadow-sm" data-bs-toggle="modal" data-bs-target="#addStreamModal">
        <i class="fa-solid fa-plus me-2"></i> إضافة قناة بث مباشر
    </button>
</div>

<!-- Streams Grid -->
<div class="row g-3 mb-4">
    <?php if (empty($streams)): ?>
    <div class="col-12">
        <div class="card glass-card border-0 shadow-sm rounded-4 p-5 text-center text-muted">
            <i class="fa-solid fa-tv fs-1 d-block mb-3 text-secondary"></i>
            <h5 class="fw-bold text-light mb-1">لا توجد قنوات بث مضافة حالياً</h5>
            <p class="fs-7 text-muted">أضف قنوات البث المباشر المحلية ليتمكن المشتركون من مشاهدتها مجاناً داخل الشبكة</p>
            <div>
                <button class="btn btn-outline-danger rounded-pill px-4" data-bs-toggle="modal" data-bs-target="#addStreamModal">
                    <i class="fa-solid fa-plus me-1"></i> إضافة أول قناة الآن
                </button>
            </div>
        </div>
    </div>
    <?php else: ?>
    <?php foreach ($streams as $s): ?>
    <?php
        $sId = $s['id'] ?? '';
        $sName = $s['name'] ?? 'قناة بث مباشر';
        $source = $s['source'] ?? $s['url'] ?? '';
        $status = $s['status'] ?? 'active';
        $isActive = ($status === 'active');
    ?>
    <div class="col-12 col-md-6 col-xl-4">
        <div class="card glass-card border-0 shadow-sm rounded-4 p-4 h-100 position-relative">
            <div class="d-flex align-items-start justify-content-between mb-3">
                <div class="d-flex align-items-center gap-3">
                    <div class="bg-danger-subtle text-danger p-3 rounded-3">
                        <i class="fa-solid fa-play fs-4"></i>
                    </div>
                    <div>
                        <h5 class="fw-bold text-light mb-0"><?= htmlspecialchars($sName) ?></h5>
                        <small class="text-muted font-monospace fs-8">ID: <?= htmlspecialchars($sId) ?></small>
                    </div>
                </div>
                <?php if ($isActive): ?>
                <span class="badge bg-success-subtle text-success border border-success px-2 py-1"><i class="fa-solid fa-circle-dot me-1"></i> مباشر</span>
                <?php else: ?>
                <span class="badge bg-secondary px-2 py-1">متوقفة</span>
                <?php endif; ?>
            </div>

            <div class="bg-dark p-2 rounded border border-secondary mb-3">
                <small class="text-muted fs-8 d-block mb-1">رابط المصدر:</small>
                <div class="font-monospace fs-8 text-info text-truncate"><?= htmlspecialchars($source) ?></div>
            </div>

            <div class="d-flex justify-content-between align-items-center mt-auto pt-2 border-top border-secondary">
                <button type="button" class="btn btn-sm btn-outline-info rounded-pill px-3" onclick="playStream('<?= htmlspecialchars($sName, ENT_QUOTES) ?>', '<?= htmlspecialchars($source, ENT_QUOTES) ?>')">
                    <i class="fa-solid fa-circle-play me-1"></i> تشغيل ومعاينة
                </button>
                <a href="/streams/<?= urlencode($sId) ?>/delete" class="btn btn-sm btn-outline-danger" onclick="return confirm('حذف قناة [<?= htmlspecialchars($sName, ENT_QUOTES) ?>]؟');" title="حذف">
                    <i class="fa-solid fa-trash"></i>
                </a>
            </div>
        </div>
    </div>
    <?php endforeach; ?>
    <?php endif; ?>
</div>

<!-- Add Stream Modal -->
<div class="modal fade" id="addStreamModal" tabindex="-1" aria-labelledby="addStreamModalLabel" aria-hidden="true">
    <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content glass-card border-danger text-light">
            <div class="modal-header border-secondary">
                <h5 class="modal-title fw-bold text-danger" id="addStreamModalLabel"><i class="fa-solid fa-tv me-2"></i> إضافة قناة بث مباشر</h5>
                <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" aria-label="Close"></button>
            </div>
            <form action="/streams" method="POST">
                <div class="modal-body">
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">اسم القناة *</label>
                        <input type="text" name="name" class="form-control bg-dark border-secondary text-light" required placeholder="beIN Sports 1 HD">
                    </div>
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">معرف القناة (Channel ID - اختياري بالإنجليزية)</label>
                        <input type="text" name="id" class="form-control bg-dark border-secondary text-light font-monospace" placeholder="bein1">
                    </div>
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">رابط مصدر البث (HLS / m3u8 / HTTP Stream) *</label>
                        <input type="text" name="source" class="form-control bg-dark border-secondary text-light font-monospace" required placeholder="http://192.168.10.50:8080/live/bein1.m3u8">
                    </div>
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">الحالة</label>
                        <select name="status" class="form-select bg-dark border-secondary text-light">
                            <option value="active" selected>نشطة ومتاحة في البوابة</option>
                            <option value="inactive">معطلة مؤقتاً</option>
                        </select>
                    </div>
                </div>
                <div class="modal-footer border-secondary">
                    <button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal">إلغاء</button>
                    <button type="submit" class="btn btn-danger px-4 fw-bold">حفظ القناة</button>
                </div>
            </form>
        </div>
    </div>
</div>

<!-- Player Modal -->
<div class="modal fade" id="playerModal" tabindex="-1" aria-hidden="true">
    <div class="modal-dialog modal-dialog-centered modal-lg">
        <div class="modal-content glass-card border-secondary text-light">
            <div class="modal-header border-secondary">
                <h5 class="modal-title fw-bold" id="playerTitle"><i class="fa-solid fa-tv me-2 text-danger"></i> معاينة البث المباشر</h5>
                <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" aria-label="Close" onclick="stopPlayer()"></button>
            </div>
            <div class="modal-body p-0 bg-black text-center" style="min-height: 380px;">
                <video id="liveVideo" controls autoplay class="w-100" style="max-height: 480px;"></video>
            </div>
        </div>
    </div>
</div>

<script>
function playStream(name, url) {
    document.getElementById('playerTitle').innerText = name;
    const video = document.getElementById('liveVideo');
    video.src = url;
    new bootstrap.Modal(document.getElementById('playerModal')).show();
}

function stopPlayer() {
    const video = document.getElementById('liveVideo');
    video.pause();
    video.src = '';
}
</script>
