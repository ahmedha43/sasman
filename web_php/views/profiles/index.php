<!-- Header & Actions -->
<div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-3">
    <div>
        <h4 class="fw-bold text-light mb-1"><i class="fa-solid fa-gauge-high me-2 text-info"></i> باقات السرعة والاشتراكات (Profiles Studio)</h4>
        <p class="text-muted mb-0 fs-7">تعريف وتعديل باقات السرعة (Rate Limits: Download/Upload) وتحديد أسعار المشتركين والوكلاء والصلاحيات</p>
    </div>
    <div class="d-flex gap-2">
        <button class="btn btn-success rounded-pill px-4 fw-bold shadow" data-bs-toggle="modal" data-bs-target="#addProfileModal">
            <i class="fa-solid fa-plus me-2"></i> إضافة باقة سرعة جديدة
        </button>
    </div>
</div>

<!-- KPI Summary Chips Toolbar -->
<div class="row g-2 mb-4">
    <div class="col-6 col-md-3">
        <div class="p-3 rounded-4 border border-info border-opacity-50 text-center h-100 profile-kpi-chip" style="background: rgba(52, 152, 219, 0.15);">
            <span class="text-info fs-8 d-block"><i class="fa-solid fa-layer-group me-1"></i> إجمالي الباقات المعرفة</span>
            <span class="fs-4 fw-bold font-monospace text-light"><?= number_format($total_profiles ?? count($profiles)) ?></span>
        </div>
    </div>
    <div class="col-6 col-md-3">
        <div class="p-3 rounded-4 border border-success border-opacity-50 text-center h-100 profile-kpi-chip" style="background: rgba(33, 140, 116, 0.15);">
            <span class="text-success fs-8 d-block"><i class="fa-solid fa-bolt me-1"></i> متوسط سرعة التحميل</span>
            <span class="fs-4 fw-bold font-monospace text-success"><?= number_format($avg_download ?? 30) ?> <span class="fs-7">Mbps</span></span>
        </div>
    </div>
    <div class="col-6 col-md-3">
        <div class="p-3 rounded-4 border border-warning border-opacity-50 text-center h-100 profile-kpi-chip" style="background: rgba(241, 196, 15, 0.15);">
            <span class="text-warning fs-8 d-block"><i class="fa-solid fa-coins me-1"></i> متوسط سعر الباقة</span>
            <span class="fs-4 fw-bold font-monospace text-warning"><?= number_format($avg_price ?? 35000, 0) ?> <span class="fs-8">د.ع</span></span>
        </div>
    </div>
    <div class="col-6 col-md-3">
        <div class="p-3 rounded-4 border border-secondary border-opacity-50 text-center h-100 profile-kpi-chip" style="background: rgba(100, 116, 139, 0.15);">
            <span class="text-white-50 fs-8 d-block"><i class="fa-solid fa-calendar-days me-1 text-primary"></i> متوسط الصلاحية</span>
            <span class="fs-4 fw-bold font-monospace text-light"><?= number_format($avg_validity ?? 30) ?> <span class="fs-7">يوماً</span></span>
        </div>
    </div>
</div>

<!-- Search Bar for Profiles -->
<div class="card glass-card border-0 shadow-sm rounded-4 p-3 mb-4">
    <div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3">
        <div class="position-relative" style="max-width: 380px; width: 100%;">
            <span class="position-absolute top-50 start-0 translate-middle-y ms-3 text-muted">
                <i class="fa-solid fa-magnifying-glass"></i>
            </span>
            <input type="text" id="profileSearchInput" class="form-control form-control-sm bg-dark border-secondary text-light rounded-pill ps-5 pe-3 py-2" placeholder="ابحث باسم الباقة أو السرعة..." onkeyup="filterProfiles()">
        </div>
        <div class="text-muted fs-8">
            يتم تطبيق باقات السرعة تلقائياً في سيرفر المايكروتك عبر كواليس RadSec و RADIUS Attribute <code>MikroTik-Rate-Limit</code>.
        </div>
    </div>
</div>

<!-- Profiles Pro Grid -->
<div class="row g-3 mb-4" id="profilesGrid">
    <?php if (empty($profiles)): ?>
    <div class="col-12 text-center py-5 text-muted">
        <i class="fa-solid fa-gauge-simple-high fs-1 d-block mb-3 text-secondary opacity-50"></i>
        <h5>لا توجد باقات معرفة حالياً</h5>
        <p class="fs-8 text-muted">قم بإنشاء باقة جديدة لتحديد سرعات المشتركين وأسعارهم</p>
    </div>
    <?php else: ?>
    <?php foreach ($profiles as $p): ?>
    <?php
        $limitStr = $p['limit'] ?? $p['rate_limit'] ?? '';
        $uploadVal = 10;
        $downloadVal = 30;
        if ($limitStr) {
            $parts = explode('/', $limitStr);
            if (count($parts) === 2) {
                $uploadVal = (int)preg_replace('/[^0-9]/', '', $parts[0]);
                $downloadVal = (int)preg_replace('/[^0-9]/', '', $parts[1]);
            }
        }
        $speedDisplay = $limitStr ?: "{$uploadVal}M/{$downloadVal}M";
        $customerPrice = (float)($p['price'] ?? 0);
        $agentPrice = (float)($p['agent_price'] ?? $customerPrice);
        $profit = max(0, $customerPrice - $agentPrice);
    ?>
    <div class="col-12 col-md-6 col-xl-4 profile-card-col" data-profilename="<?= htmlspecialchars(strtolower($p['name'])) ?>" data-speed="<?= htmlspecialchars(strtolower($speedDisplay)) ?>">
        <div class="card glass-card border-0 shadow-lg rounded-4 p-4 h-100 position-relative profile-card-pro overflow-hidden">
            <!-- Ambient Glow Accent -->
            <div class="profile-card-glow"></div>

            <!-- Card Header -->
            <div class="d-flex justify-content-between align-items-start mb-3 position-relative z-2">
                <div class="d-flex align-items-center gap-3">
                    <div class="profile-icon-badge">
                        <i class="fa-solid fa-gauge-high"></i>
                    </div>
                    <div>
                        <h5 class="fw-bold text-light mb-0 font-monospace"><?= htmlspecialchars($p['name']) ?></h5>
                        <small class="text-white-50 fs-9">باقة اشتراك المايكروتك</small>
                    </div>
                </div>
                <div class="btn-group btn-group-sm">
                    <button type="button" class="btn btn-outline-info rounded-pill px-3 me-1" onclick='openEditProfile(<?= json_encode($p, JSON_HEX_TAG | JSON_HEX_APOS | JSON_HEX_QUOT | JSON_HEX_AMP) ?>)' title="تعديل الباقة">
                        <i class="fa-solid fa-pen-to-square me-1"></i> تعديل
                    </button>
                    <a href="/profiles/<?= urlencode($p['name']) ?>/delete" onclick="return confirm('هل تريد بالتأكيد حذف باقة السرعة [<?= htmlspecialchars($p['name'], ENT_QUOTES) ?>]؟');" class="btn btn-outline-danger rounded-pill px-2" title="حذف">
                        <i class="fa-solid fa-trash"></i>
                    </a>
                </div>
            </div>

            <!-- Rate Limit Speed Meter Box -->
            <div class="p-3 rounded-4 bg-dark bg-opacity-75 border border-secondary border-opacity-50 mb-3 position-relative z-2">
                <div class="d-flex justify-content-between align-items-center mb-2">
                    <span class="text-muted fs-8 fw-semibold">معدل السرعة (Rate Limit):</span>
                    <span class="badge bg-primary-subtle text-primary border border-primary px-3 py-1 font-monospace fs-7 fw-bold"><?= htmlspecialchars($speedDisplay) ?></span>
                </div>
                <div class="row g-2 text-center">
                    <div class="col-6">
                        <div class="p-2 rounded-3 bg-dark border border-secondary border-opacity-25">
                            <small class="text-muted fs-9 d-block"><i class="fa-solid fa-arrow-down text-success me-1"></i> تنزيل (Download)</small>
                            <strong class="text-success fs-5 font-monospace"><?= $downloadVal ?> <span class="fs-8">Mbps</span></strong>
                        </div>
                    </div>
                    <div class="col-6">
                        <div class="p-2 rounded-3 bg-dark border border-secondary border-opacity-25">
                            <small class="text-muted fs-9 d-block"><i class="fa-solid fa-arrow-up text-primary me-1"></i> رفع (Upload)</small>
                            <strong class="text-primary fs-5 font-monospace"><?= $uploadVal ?> <span class="fs-8">Mbps</span></strong>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Pricing & Profit Matrix -->
            <div class="row g-2 text-center mb-3 position-relative z-2">
                <div class="col-6">
                    <div class="bg-dark bg-opacity-50 p-2 rounded-3 border border-secondary border-opacity-50">
                        <small class="text-muted fs-9 d-block">سعر المشترك (البيع)</small>
                        <strong class="text-success fs-6 font-monospace"><?= number_format($customerPrice, 0) ?> د.ع</strong>
                    </div>
                </div>
                <div class="col-6">
                    <div class="bg-dark bg-opacity-50 p-2 rounded-3 border border-secondary border-opacity-50">
                        <small class="text-muted fs-9 d-block">سعر الوكيل (الجملة)</small>
                        <strong class="text-info fs-6 font-monospace"><?= number_format($agentPrice, 0) ?> د.ع</strong>
                    </div>
                </div>
                <?php if ($profit > 0): ?>
                <div class="col-12">
                    <div class="p-1 rounded-pill bg-success bg-opacity-10 border border-success border-opacity-25 text-success fs-9 fw-semibold">
                        <i class="fa-solid fa-sack-dollar me-1"></i> ربح الوكيل في الباقة: <strong>+<?= number_format($profit, 0) ?> د.ع</strong>
                    </div>
                </div>
                <?php endif; ?>
            </div>

            <!-- Network Specs Footer -->
            <div class="d-flex justify-content-between text-muted fs-8 pt-3 border-top border-secondary border-opacity-50 position-relative z-2">
                <span><i class="fa-solid fa-calendar-days me-1 text-warning"></i> الصلاحية: <strong class="text-light"><?= htmlspecialchars($p['validity_days'] ?? 30) ?></strong> يوماً</span>
                <?php if (!empty($p['pool'])): ?>
                <span><i class="fa-solid fa-network-wired me-1 text-info"></i> Pool: <code class="text-info"><?= htmlspecialchars($p['pool']) ?></code></span>
                <?php else: ?>
                <span><i class="fa-solid fa-arrows-split-up-and-left me-1 text-secondary"></i> مجمع افتراضي</span>
                <?php endif; ?>
            </div>

            <!-- Quick Filter Users Link -->
            <div class="mt-3 pt-2 text-center position-relative z-2">
                <a href="/users?search=<?= urlencode($p['name']) ?>" class="text-decoration-none fs-8 text-primary-emphasis fw-semibold">
                    <i class="fa-solid fa-users me-1"></i> عرض المشتركين المشتركين بهذه الباقة <i class="fa-solid fa-arrow-left fs-9 ms-1"></i>
                </a>
            </div>

            <!-- Watermark Speed Icon -->
            <i class="fa-solid fa-bolt watermark-profile-icon"></i>
        </div>
    </div>
    <?php endforeach; ?>
    <?php endif; ?>
</div>

<!-- 1. ADD PROFILE MODAL -->
<div class="modal fade" id="addProfileModal" tabindex="-1" aria-labelledby="addProfileModalLabel" aria-hidden="true">
    <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content glass-card border-success text-light">
            <div class="modal-header border-secondary">
                <h5 class="modal-title fw-bold text-success" id="addProfileModalLabel">
                    <i class="fa-solid fa-gauge-high me-2"></i> إنشاء باقة سرعة جديدة
                </h5>
                <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" aria-label="Close"></button>
            </div>
            <form action="/profiles" method="POST">
                <div class="modal-body">
                    <!-- Speed Presets Chips -->
                    <div class="mb-3">
                        <label class="form-label fs-8 text-muted fw-semibold d-block">قوالب سرعة جاهزة سريعة التعبئة (Presets):</label>
                        <div class="d-flex flex-wrap gap-1">
                            <button type="button" class="btn btn-sm btn-outline-info rounded-pill px-3 fs-9" onclick="setSpeedPreset('Speed-10M', 10, 5, 25000, 20000)">10 Mbps</button>
                            <button type="button" class="btn btn-sm btn-outline-info rounded-pill px-3 fs-9" onclick="setSpeedPreset('Speed-20M', 20, 10, 30000, 25000)">20 Mbps</button>
                            <button type="button" class="btn btn-sm btn-outline-info rounded-pill px-3 fs-9" onclick="setSpeedPreset('Speed-30M', 30, 10, 35000, 30000)">30 Mbps</button>
                            <button type="button" class="btn btn-sm btn-outline-info rounded-pill px-3 fs-9" onclick="setSpeedPreset('Speed-50M', 50, 20, 45000, 38000)">50 Mbps</button>
                            <button type="button" class="btn btn-sm btn-outline-info rounded-pill px-3 fs-9" onclick="setSpeedPreset('Turbo-100M', 100, 30, 60000, 50000)">100 Mbps</button>
                        </div>
                    </div>

                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">اسم الباقة (Profile Name) *</label>
                        <input type="text" name="name" id="add_name" class="form-control bg-dark border-secondary text-light font-monospace" required placeholder="Turbo-30M">
                    </div>
                    
                    <div class="row g-2 mb-3">
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">سرعة التحميل (Download: Mbps) *</label>
                            <input type="number" name="download" id="add_download" class="form-control bg-dark border-secondary text-light font-monospace" required placeholder="30" value="30" min="1">
                        </div>
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">سرعة الرفع (Upload: Mbps) *</label>
                            <input type="number" name="upload" id="add_upload" class="form-control bg-dark border-secondary text-light font-monospace" required placeholder="10" value="10" min="1">
                        </div>
                    </div>

                    <div class="row g-2 mb-3">
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">سعر المشترك (د.ع) *</label>
                            <input type="number" name="price" id="add_price" class="form-control bg-dark border-secondary text-light font-monospace" value="35000" step="500" required>
                        </div>
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">سعر الوكيل (د.ع)</label>
                            <input type="number" name="agent_price" id="add_agent_price" class="form-control bg-dark border-secondary text-light font-monospace" value="30000" step="500">
                        </div>
                    </div>

                    <div class="row g-2 mb-3">
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">مدة الصلاحية (بالأيام)</label>
                            <input type="number" name="validity" class="form-control bg-dark border-secondary text-light font-monospace" value="30" required>
                        </div>
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">عدد الجلسات المتزامنة</label>
                            <input type="number" name="simultaneous" class="form-control bg-dark border-secondary text-light font-monospace" value="1" min="1">
                        </div>
                    </div>

                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">مجمع العناوين (Framed-Pool - اختياري)</label>
                        <input type="text" name="pool" class="form-control bg-dark border-secondary text-light font-monospace" placeholder="مثال: dhcp_pool1">
                    </div>
                </div>
                <div class="modal-footer border-secondary">
                    <button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal">إلغاء</button>
                    <button type="submit" class="btn btn-success px-4 fw-bold">حفظ الباقة</button>
                </div>
            </form>
        </div>
    </div>
</div>

<!-- 2. EDIT PROFILE MODAL -->
<div class="modal fade" id="editProfileModal" tabindex="-1" aria-labelledby="editProfileModalLabel" aria-hidden="true">
    <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content glass-card border-info text-light">
            <div class="modal-header border-secondary">
                <h5 class="modal-title fw-bold text-info" id="editProfileModalLabel">
                    <i class="fa-solid fa-pen-to-square me-2"></i> تعديل باقة السرعة
                </h5>
                <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" aria-label="Close"></button>
            </div>
            <form action="/profiles" method="POST">
                <input type="hidden" name="original_name" id="edit_original_name">
                <div class="modal-body">
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">اسم الباقة (Profile Name) *</label>
                        <input type="text" name="name" id="edit_name" class="form-control bg-dark border-secondary text-light font-monospace" required>
                    </div>
                    
                    <div class="row g-2 mb-3">
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">سرعة التحميل (Download: Mbps) *</label>
                            <input type="number" name="download" id="edit_download" class="form-control bg-dark border-secondary text-light font-monospace" required min="1">
                        </div>
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">سرعة الرفع (Upload: Mbps) *</label>
                            <input type="number" name="upload" id="edit_upload" class="form-control bg-dark border-secondary text-light font-monospace" required min="1">
                        </div>
                    </div>

                    <div class="row g-2 mb-3">
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">سعر المشترك (د.ع) *</label>
                            <input type="number" name="price" id="edit_price" class="form-control bg-dark border-secondary text-light font-monospace" step="500" required>
                        </div>
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">سعر الوكيل (د.ع)</label>
                            <input type="number" name="agent_price" id="edit_agent_price" class="form-control bg-dark border-secondary text-light font-monospace" step="500">
                        </div>
                    </div>

                    <div class="row g-2 mb-3">
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">مدة الصلاحية (بالأيام)</label>
                            <input type="number" name="validity" id="edit_validity" class="form-control bg-dark border-secondary text-light font-monospace" required>
                        </div>
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">عدد الجلسات المتزامنة</label>
                            <input type="number" name="simultaneous" id="edit_simultaneous" class="form-control bg-dark border-secondary text-light font-monospace" value="1" min="1">
                        </div>
                    </div>

                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">مجمع العناوين (Framed-Pool - اختياري)</label>
                        <input type="text" name="pool" id="edit_pool" class="form-control bg-dark border-secondary text-light font-monospace">
                    </div>
                </div>
                <div class="modal-footer border-secondary">
                    <button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal">إلغاء</button>
                    <button type="submit" class="btn btn-info px-4 fw-bold text-dark">تحديث وحفظ التعديلات</button>
                </div>
            </form>
        </div>
    </div>
</div>

<style>
/* ============================================================
   PRO PROFILES STUDIO STYLING
============================================================ */
.profile-kpi-chip {
    transition: transform 0.2s ease, box-shadow 0.2s ease;
}
.profile-kpi-chip:hover {
    transform: translateY(-3px);
    box-shadow: 0 6px 16px rgba(0, 0, 0, 0.25);
}

.profile-card-pro {
    transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
    border: 1px solid rgba(255, 255, 255, 0.12) !important;
}
.profile-card-pro:hover {
    transform: translateY(-6px) scale(1.015);
    box-shadow: 0 20px 40px -10px rgba(0, 0, 0, 0.6) !important;
    border-color: rgba(56, 189, 248, 0.4) !important;
}

.profile-card-glow {
    position: absolute;
    top: -50%;
    right: -20%;
    width: 220px;
    height: 220px;
    background: radial-gradient(circle, rgba(56, 189, 248, 0.12) 0%, rgba(56, 189, 248, 0) 70%);
    pointer-events: none;
    border-radius: 50%;
}

.profile-icon-badge {
    width: 44px;
    height: 44px;
    border-radius: 14px;
    background: linear-gradient(135deg, rgba(56, 189, 248, 0.2) 0%, rgba(14, 165, 233, 0.1) 100%);
    border: 1px solid rgba(56, 189, 248, 0.3);
    color: #38bdf8;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.25rem;
    box-shadow: 0 4px 12px rgba(56, 189, 248, 0.15);
}

.watermark-profile-icon {
    position: absolute;
    bottom: -15px;
    left: -15px;
    font-size: 6.5rem;
    opacity: 0.05;
    color: #ffffff;
    pointer-events: none;
    transition: transform 0.35s ease, opacity 0.35s ease;
}
.profile-card-pro:hover .watermark-profile-icon {
    transform: scale(1.15) rotate(-8deg);
    opacity: 0.1;
    color: #38bdf8;
}
</style>

<script>
function setSpeedPreset(name, dl, ul, price, agentPrice) {
    document.getElementById('add_name').value = name;
    document.getElementById('add_download').value = dl;
    document.getElementById('add_upload').value = ul;
    document.getElementById('add_price').value = price;
    document.getElementById('add_agent_price').value = agentPrice;
}

function openEditProfile(profile) {
    document.getElementById('edit_original_name').value = profile.name || '';
    document.getElementById('edit_name').value = profile.name || '';
    
    let upload = 10;
    let download = 30;
    const limitStr = profile.limit || profile.rate_limit || '';
    if (limitStr) {
        const parts = limitStr.split('/');
        if (parts.length === 2) {
            upload = parseInt(parts[0].replace(/[^0-9]/g, '')) || 10;
            download = parseInt(parts[1].replace(/[^0-9]/g, '')) || 30;
        }
    }
    
    document.getElementById('edit_download').value = download;
    document.getElementById('edit_upload').value = upload;
    document.getElementById('edit_price').value = profile.price || 0;
    document.getElementById('edit_agent_price').value = profile.agent_price || profile.price || 0;
    document.getElementById('edit_validity').value = profile.validity_days || 30;
    document.getElementById('edit_simultaneous').value = profile.simultaneous || 1;
    document.getElementById('edit_pool').value = profile.pool || '';
    
    new bootstrap.Modal(document.getElementById('editProfileModal')).show();
}

function filterProfiles() {
    const q = (document.getElementById('profileSearchInput').value || '').toLowerCase().trim();
    const cols = document.querySelectorAll('.profile-card-col');
    
    cols.forEach(col => {
        const name = col.getAttribute('data-profilename') || '';
        const speed = col.getAttribute('data-speed') || '';
        if (!q || name.includes(q) || speed.includes(q)) {
            col.style.display = '';
        } else {
            col.style.display = 'none';
        }
    });
}
</script>
