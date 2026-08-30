<?php
function formatBytesUsers($bytes) {
    if ($bytes <= 0) return '0 B';
    $units = ['B', 'KB', 'MB', 'GB', 'TB'];
    $bytes = max($bytes, 0);
    $pow = floor(($bytes ? log($bytes) : 0) / log(1024));
    $pow = min($pow, count($units) - 1);
    $bytes /= pow(1024, $pow);
    return round($bytes, 2) . ' ' . $units[$pow];
}
?>

<!-- Header & Title -->
<div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-3">
    <div>
        <h4 class="fw-bold text-light mb-1"><i class="fa-solid fa-users me-2 text-primary"></i> إدارة المشتركين والحسابات</h4>
        <p class="text-muted mb-0 fs-7">عرض، إضافة، تعديل، تجديد، ومراقبة استهلاك وجلسات المشتركين عبر راديوس والمايكروتك</p>
    </div>
    <div class="d-flex gap-2">
        <?php if (!empty($perms['can_create_users'])): ?>
        <button class="btn btn-primary rounded-pill px-4 fw-bold shadow" data-bs-toggle="modal" data-bs-target="#addUserModal">
            <i class="fa-solid fa-user-plus me-2"></i> إضافة مشترك جديد
        </button>
        <?php endif; ?>
    </div>
</div>

<!-- Pro KPI Summary Chips Bar -->
<div class="row g-2 mb-4">
    <div class="col-6 col-md-4 col-xl-2">
        <div class="p-3 rounded-4 border border-secondary border-opacity-50 text-center h-100 kpi-chip" style="background: rgba(75, 101, 132, 0.15);" onclick="setQuickFilter('all')">
            <span class="text-muted fs-8 d-block"><i class="fa-solid fa-users me-1 text-info"></i> إجمالي المشتركين</span>
            <span class="fs-4 fw-bold font-monospace text-light"><?= number_format($total_users ?? count($users)) ?></span>
        </div>
    </div>
    <div class="col-6 col-md-4 col-xl-2">
        <div class="p-3 rounded-4 border border-success border-opacity-50 text-center h-100 kpi-chip" style="background: rgba(33, 140, 116, 0.15);" onclick="setQuickFilter('active')">
            <span class="text-success fs-8 d-block"><i class="fa-solid fa-face-smile me-1"></i> النشطين</span>
            <span class="fs-4 fw-bold font-monospace text-success"><?= number_format($active_users ?? 0) ?></span>
        </div>
    </div>
    <div class="col-6 col-md-4 col-xl-2">
        <div class="p-3 rounded-4 border border-info border-opacity-50 text-center h-100 kpi-chip" style="background: rgba(52, 152, 219, 0.15);" onclick="setQuickFilter('online')">
            <span class="text-info fs-8 d-block"><i class="fa-solid fa-bolt me-1"></i> المتصلين الآن</span>
            <span class="fs-4 fw-bold font-monospace text-info"><?= number_format($online_users ?? 0) ?></span>
        </div>
    </div>
    <div class="col-6 col-md-4 col-xl-2">
        <div class="p-3 rounded-4 border border-danger border-opacity-50 text-center h-100 kpi-chip" style="background: rgba(252, 92, 101, 0.15);" onclick="setQuickFilter('expired')">
            <span class="text-danger fs-8 d-block"><i class="fa-solid fa-face-frown me-1"></i> المنتهين</span>
            <span class="fs-4 fw-bold font-monospace text-danger"><?= number_format($expired_users ?? 0) ?></span>
        </div>
    </div>
    <div class="col-6 col-md-4 col-xl-2">
        <div class="p-3 rounded-4 border border-warning border-opacity-50 text-center h-100 kpi-chip" style="background: rgba(241, 196, 15, 0.15);" onclick="setQuickFilter('about_to_expire')">
            <span class="text-warning fs-8 d-block"><i class="fa-solid fa-calendar-days me-1"></i> قريب الانتهاء</span>
            <span class="fs-4 fw-bold font-monospace text-warning"><?= number_format($about_to_expire ?? 0) ?></span>
        </div>
    </div>
    <div class="col-6 col-md-4 col-xl-2">
        <div class="p-3 rounded-4 border border-primary border-opacity-50 text-center h-100 kpi-chip" style="background: rgba(63, 81, 181, 0.15);" onclick="window.location.href='/transactions'">
            <span class="text-primary-emphasis fs-8 d-block"><i class="fa-solid fa-coins me-1 text-primary"></i> إجمالي الديون</span>
            <span class="fs-6 fw-bold font-monospace text-light text-truncate d-block mt-1"><?= number_format($total_debt ?? 0, 0) ?> د.ع</span>
        </div>
    </div>
</div>

<!-- Interactive Search & Filter Toolbar -->
<div class="card glass-card border-0 shadow-sm rounded-4 p-3 mb-4">
    <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <!-- Quick Filter Pills -->
        <div class="d-flex flex-wrap gap-1" id="filterPills">
            <button class="btn btn-sm btn-outline-light rounded-pill px-3 fw-bold active-filter" onclick="setFilter('all', this)">
                <i class="fa-solid fa-list me-1"></i> الكل (<span id="count-all"><?= count($users) ?></span>)
            </button>
            <button class="btn btn-sm btn-outline-success rounded-pill px-3 fw-bold" onclick="setFilter('active', this)">
                <i class="fa-solid fa-circle-check me-1"></i> النشطين
            </button>
            <button class="btn btn-sm btn-outline-info rounded-pill px-3 fw-bold" onclick="setFilter('online', this)">
                <i class="fa-solid fa-bolt me-1"></i> المتصلين الآن
            </button>
            <button class="btn btn-sm btn-outline-danger rounded-pill px-3 fw-bold" onclick="setFilter('expired', this)">
                <i class="fa-solid fa-circle-xmark me-1"></i> المنتهين
            </button>
            <button class="btn btn-sm btn-outline-warning rounded-pill px-3 fw-bold" onclick="setFilter('about_to_expire', this)">
                <i class="fa-solid fa-triangle-exclamation me-1"></i> قريب الانتهاء
            </button>
        </div>

        <!-- Search Box with Live Badge -->
        <div class="d-flex align-items-center gap-2">
            <div class="position-relative" style="min-width: 280px; width: 100%;">
                <span class="position-absolute top-50 start-0 translate-middle-y ms-3 text-muted">
                    <i class="fa-solid fa-magnifying-glass"></i>
                </span>
                <input type="text" id="userSearchInput" class="form-control form-control-sm bg-dark border-secondary text-light rounded-pill ps-5 pe-5 py-2" placeholder="ابحث بالاسم، اليوزر، الهاتف، أو الـ IP..." onkeyup="filterUsersTable()">
                <span class="position-absolute top-50 end-0 translate-middle-y me-2 badge bg-primary rounded-pill font-monospace" id="usersCountBadge"><?= count($users) ?></span>
            </div>
        </div>
    </div>
</div>

<!-- Pro Users Table Card -->
<div class="card glass-card border-0 shadow-lg rounded-4 p-0 overflow-hidden">
    <div class="table-responsive">
        <table class="table table-dark table-hover align-middle mb-0 users-table-pro" id="usersTable">
            <thead>
                <tr>
                    <th class="ps-4">#</th>
                    <th>👤 المشترك</th>
                    <th>🔑 كلمة المرور</th>
                    <th>📦 الباقة</th>
                    <th>📅 تاريخ الانتهاء</th>
                    <th>💰 الرصيد / الدين</th>
                    <th>⚡ الجلسة والمدة</th>
                    <th>📊 البيانات (تنزيل/رفع)</th>
                    <th>📡 IP والماك (MAC)</th>
                    <th class="text-center pe-4">⚙️ الإجراءات</th>
                </tr>
            </thead>
            <tbody>
                <?php if (empty($users)): ?>
                <tr>
                    <td colspan="10" class="text-center py-5 text-muted">
                        <i class="fa-solid fa-users-slash fs-1 d-block mb-3 text-secondary opacity-50"></i>
                        <h5>لا يوجد مشتركون حالياً</h5>
                        <p class="fs-8 text-muted">قم بإضافة مشترك جديد أو استيراد البيانات من Excel أو SAS4</p>
                    </td>
                </tr>
                <?php else: ?>
                <?php foreach ($users as $i => $u): ?>
                <?php 
                    $uName = $u['user'] ?? $u['username'] ?? '';
                    $uPass = $u['pass'] ?? $u['password'] ?? '—';
                    $uFullName = $u['full_name'] ?? '';
                    $uPhone = $u['phone'] ?? '';
                    $uProfile = $u['profile'] ?? 'Standard';
                    $uExpiry = $u['expires_at'] ?? $u['expiration'] ?? '—';
                    $uBalance = (float)($u['balance'] ?? 0);
                    $isExpired = !empty($u['expired']);
                    $isEnabled = !isset($u['enabled']) || !empty($u['enabled']);
                    
                    // Session Info
                    $session = $u['session'] ?? [];
                    $isOnline = !empty($session['online']) || !empty($u['online']);
                    $framedIP = $session['framed_ip_address'] ?? $session['ip'] ?? $u['framed_ip_address'] ?? '—';
                    $macAddr = $session['calling_station_id'] ?? $session['mac'] ?? $u['mac'] ?? '—';
                    $uptime = $session['uptime'] ?? (!empty($session['session_seconds']) ? gmdate("H:i:s", $session['session_seconds']) : '—');
                    
                    // Usage Stats
                    $inOctets = (int)($session['download_bytes'] ?? $session['acctinputoctets'] ?? $u['acctinputoctets'] ?? 0);
                    $outOctets = (int)($session['upload_bytes'] ?? $session['acctoutputoctets'] ?? $u['acctoutputoctets'] ?? 0);

                    // Check if about to expire within 3 days
                    $isAboutToExpire = false;
                    $daysLeft = null;
                    if ($uExpiry && $uExpiry !== '—') {
                        $expTs = strtotime($uExpiry);
                        $now = time();
                        $diff = $expTs - $now;
                        $daysLeft = ceil($diff / 86400);
                        if ($expTs > $now && $expTs <= ($now + 3 * 86400)) {
                            $isAboutToExpire = true;
                        }
                    }
                ?>
                <tr class="user-row" 
                    data-username="<?= htmlspecialchars(strtolower($uName)) ?>"
                    data-fullname="<?= htmlspecialchars(strtolower($uFullName)) ?>"
                    data-phone="<?= htmlspecialchars($uPhone) ?>"
                    data-ip="<?= htmlspecialchars($framedIP) ?>"
                    data-status="<?= $isExpired ? 'expired' : ($isAboutToExpire ? 'about_to_expire' : 'active') ?>"
                    data-online="<?= $isOnline ? '1' : '0' ?>"
                    data-aboutexpire="<?= $isAboutToExpire ? '1' : '0' ?>">
                    
                    <!-- Index -->
                    <td class="ps-4 text-muted fs-8 font-monospace"><?= $i + 1 ?></td>
                    
                    <!-- 1. Subscriber -->
                    <td>
                        <div class="d-flex align-items-center gap-3">
                            <div class="user-avatar-circle">
                                <?= mb_strtoupper(mb_substr($uName, 0, 1)) ?>
                            </div>
                            <div>
                                <div class="d-flex align-items-center gap-2">
                                    <a href="javascript:void(0)" class="fw-bold text-info font-monospace fs-6 text-decoration-none user-name-link" onclick="openUserDetailsModal('<?= htmlspecialchars($uName, ENT_QUOTES) ?>', '<?= htmlspecialchars($uFullName, ENT_QUOTES) ?>', '<?= htmlspecialchars($uProfile, ENT_QUOTES) ?>', '<?= htmlspecialchars($uExpiry, ENT_QUOTES) ?>', '<?= htmlspecialchars($uPhone, ENT_QUOTES) ?>', '<?= $uBalance ?>')">
                                        @<?= htmlspecialchars($uName) ?>
                                    </a>
                                    <?php if (!$isEnabled): ?>
                                    <span class="badge bg-danger-subtle text-danger border border-danger fs-9 px-2">معطل</span>
                                    <?php endif; ?>
                                </div>
                                <?php if ($uFullName): ?>
                                <span class="text-white-50 d-block fs-8"><?= htmlspecialchars($uFullName) ?></span>
                                <?php endif; ?>
                                <?php if ($uPhone): ?>
                                <div class="d-flex align-items-center gap-1 mt-1">
                                    <span class="text-muted font-monospace fs-9"><?= htmlspecialchars($uPhone) ?></span>
                                    <a href="https://wa.me/<?= preg_replace('/[^0-9]/', '', $uPhone) ?>" target="_blank" class="text-success fs-9 ms-1" title="مراسلة عبر واتساب">
                                        <i class="fa-brands fa-whatsapp"></i>
                                    </a>
                                </div>
                                <?php endif; ?>
                            </div>
                        </div>
                    </td>

                    <!-- 2. Password -->
                    <td>
                        <div class="d-flex align-items-center gap-1">
                            <code class="pro-pass-code font-monospace"><?= htmlspecialchars($uPass) ?></code>
                            <button class="btn btn-sm btn-link text-muted p-1 copy-btn" onclick="copyPassword('<?= htmlspecialchars($uPass, ENT_QUOTES) ?>', this)" title="نسخ كلمة المرور">
                                <i class="fa-solid fa-copy fs-8"></i>
                            </button>
                        </div>
                    </td>

                    <!-- 3. Profile -->
                    <td>
                        <span class="badge profile-pill font-monospace"><i class="fa-solid fa-gauge-high me-1 text-primary"></i><?= htmlspecialchars($uProfile) ?></span>
                    </td>

                    <!-- 4. Expiry -->
                    <td>
                        <div class="fs-8 text-light font-monospace mb-1"><?= htmlspecialchars($uExpiry) ?></div>
                        <?php if ($isExpired): ?>
                        <span class="badge bg-danger-subtle text-danger border border-danger fs-9 px-2"><i class="fa-solid fa-circle-xmark me-1"></i> منتهي</span>
                        <?php elseif ($isAboutToExpire): ?>
                        <span class="badge bg-warning-subtle text-warning border border-warning fs-9 px-2"><i class="fa-solid fa-triangle-exclamation me-1"></i> ينتهي خلال <?= $daysLeft ?> يوم</span>
                        <?php else: ?>
                        <span class="badge bg-success-subtle text-success border border-success fs-9 px-2"><i class="fa-solid fa-circle-check me-1"></i> نشط (باقي <?= $daysLeft ?> يوم)</span>
                        <?php endif; ?>
                    </td>

                    <!-- 5. Balance / Debt -->
                    <td>
                        <?php if ($uBalance > 0): ?>
                        <span class="badge debt-badge font-monospace"><i class="fa-solid fa-clock-rotate-left me-1"></i> مطلوب <?= number_format($uBalance, 0) ?> د.ع</span>
                        <?php else: ?>
                        <span class="badge paid-badge font-monospace"><i class="fa-solid fa-check me-1"></i> خالص (0)</span>
                        <?php endif; ?>
                    </td>

                    <!-- 6. Session & Duration -->
                    <td>
                        <?php if ($isOnline): ?>
                        <div class="d-flex align-items-center gap-1 mb-1">
                            <span class="online-pulse-dot"></span>
                            <strong class="text-success fs-8">متصل الآن</strong>
                        </div>
                        <small class="text-muted d-block fs-9 font-monospace"><i class="fa-solid fa-clock me-1"></i><?= htmlspecialchars($uptime) ?></small>
                        <?php else: ?>
                        <span class="badge bg-dark text-muted border border-secondary fs-9 px-2">غير متصل</span>
                        <?php endif; ?>
                    </td>

                    <!-- 7. Data Usage -->
                    <td>
                        <div class="d-flex flex-column gap-1 fs-9 font-monospace">
                            <span class="text-success"><i class="fa-solid fa-arrow-down me-1"></i><?= formatBytesUsers($inOctets) ?></span>
                            <span class="text-primary"><i class="fa-solid fa-arrow-up me-1"></i><?= formatBytesUsers($outOctets) ?></span>
                        </div>
                    </td>

                    <!-- 8. IP & MAC -->
                    <td>
                        <div class="font-monospace fs-8 text-info"><?= htmlspecialchars($framedIP) ?></div>
                        <small class="font-monospace fs-9 text-muted d-block"><?= htmlspecialchars($macAddr) ?></small>
                    </td>

                    <!-- 9. Actions -->
                    <td class="text-center pe-4">
                        <div class="btn-group btn-group-sm action-btn-group shadow-sm">
                            <!-- Renew Modal Button -->
                            <?php if (!empty($perms['can_renew_users'])): ?>
                            <button type="button" class="btn btn-action-renew" onclick="openRenewModal('<?= htmlspecialchars($uName, ENT_QUOTES) ?>', '<?= htmlspecialchars($uProfile, ENT_QUOTES) ?>', '<?= htmlspecialchars($uExpiry, ENT_QUOTES) ?>')" title="تجديد الاشتراك">
                                <i class="fa-solid fa-rotate-right me-1"></i> تجديد
                            </button>
                            <?php endif; ?>

                            <!-- Edit Modal Button -->
                            <?php if (!empty($perms['can_edit_users'])): ?>
                            <button type="button" class="btn btn-action-edit" onclick="openEditModal('<?= htmlspecialchars($uName, ENT_QUOTES) ?>', '<?= htmlspecialchars($uPass, ENT_QUOTES) ?>', '<?= htmlspecialchars($uProfile, ENT_QUOTES) ?>', '<?= htmlspecialchars($uFullName, ENT_QUOTES) ?>', '<?= htmlspecialchars($uPhone, ENT_QUOTES) ?>')" title="تعديل">
                                <i class="fa-solid fa-pen-to-square"></i>
                            </button>
                            <?php endif; ?>

                            <!-- User Card Modal Button (Always accessible) -->
                            <button type="button" class="btn btn-action-card" onclick="openUserDetailsModal('<?= htmlspecialchars($uName, ENT_QUOTES) ?>', '<?= htmlspecialchars($uFullName, ENT_QUOTES) ?>', '<?= htmlspecialchars($uProfile, ENT_QUOTES) ?>', '<?= htmlspecialchars($uExpiry, ENT_QUOTES) ?>', '<?= htmlspecialchars($uPhone, ENT_QUOTES) ?>', '<?= $uBalance ?>')" title="بطاقة المشترك">
                                <i class="fa-solid fa-id-card"></i>
                            </button>

                            <!-- Disconnect Session -->
                            <?php if ($isOnline && !empty($perms['can_disconnect_users'])): ?>
                            <a href="/users/<?= urlencode($uName) ?>/disconnect" class="btn btn-action-kick" title="فصل الجلسة من المايكروتك">
                                <i class="fa-solid fa-plug-circle-xmark"></i>
                            </a>
                            <?php endif; ?>

                            <!-- Toggle Status -->
                            <?php if (!empty($perms['can_toggle_users'])): ?>
                                <?php if ($isEnabled): ?>
                                <a href="/users/<?= urlencode($uName) ?>/toggle-status" class="btn btn-action-disable" title="تعطيل / إيقاف الحساب" onclick="return confirm('هل تريد بالتأكيد إيقاف/تعطيل حساب المشترك [<?= htmlspecialchars($uName, ENT_QUOTES) ?>]؟');">
                                    <i class="fa-solid fa-pause"></i>
                                </a>
                                <?php else: ?>
                                <a href="/users/<?= urlencode($uName) ?>/toggle-status" class="btn btn-action-enable" title="تفعيل / تشغيل الحساب">
                                    <i class="fa-solid fa-play"></i>
                                </a>
                                <?php endif; ?>
                            <?php endif; ?>

                            <!-- Delete User -->
                            <?php if (!empty($perms['can_delete_users'])): ?>
                            <a href="/users/<?= urlencode($uName) ?>/delete" class="btn btn-action-delete" onclick="return confirm('هل أنت متأكد من حذف المشترك [<?= htmlspecialchars($uName, ENT_QUOTES) ?>]؟');" title="حذف">
                                <i class="fa-solid fa-trash"></i>
                            </a>
                            <?php endif; ?>
                        </div>
                    </td>
                </tr>
                <?php endforeach; ?>
                <?php endif; ?>
            </tbody>
        </table>
    </div>
</div>

<!-- 1. RENEW MODAL -->
<div class="modal fade" id="renewUserModal" tabindex="-1" aria-labelledby="renewModalLabel" aria-hidden="true">
    <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content glass-card border-success text-light">
            <div class="modal-header border-secondary">
                <h5 class="modal-title fw-bold text-success" id="renewModalLabel">
                    <i class="fa-solid fa-rotate-right me-2"></i> تجديد اشتراك مشترك
                </h5>
                <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" aria-label="Close"></button>
            </div>
            <form action="" method="POST" id="renewForm">
                <div class="modal-body">
                    <div class="bg-dark p-3 rounded-4 border border-secondary mb-3">
                        <span class="text-muted fs-8 d-block mb-1">المشترك المراد تجديده:</span>
                        <h5 class="fw-bold text-info mb-0 font-monospace" id="renewModalUsername">—</h5>
                    </div>

                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">اختر باقة التجديد *</label>
                        <select name="profile" id="renewProfileSelect" class="form-select bg-dark border-secondary text-light" required onchange="updateRenewPrice()">
                            <?php foreach ($profiles as $p): ?>
                            <option value="<?= htmlspecialchars($p['name']) ?>" 
                                    data-price="<?= htmlspecialchars($p['price'] ?? 0) ?>" 
                                    data-days="<?= htmlspecialchars($p['validity_days'] ?? 30) ?>">
                                <?= htmlspecialchars($p['name']) ?> (<?= number_format($p['price'] ?? 0, 0) ?> د.ع / <?= htmlspecialchars($p['validity_days'] ?? 30) ?> يوم)
                            </option>
                            <?php endforeach; ?>
                        </select>
                    </div>

                    <div class="row g-2 mb-3">
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">المبلغ المطلوب (د.ع)</label>
                            <input type="number" name="price" id="renewPriceInput" class="form-control bg-dark border-secondary text-light font-monospace" required>
                        </div>
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">مدة التمديد (أيام)</label>
                            <input type="number" name="validity_days" id="renewDaysInput" class="form-control bg-dark border-secondary text-light font-monospace" value="30" required>
                        </div>
                    </div>

                    <!-- Payment Status -->
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold d-block">طريقة استلام المبلغ *</label>
                        <div class="bg-dark p-3 rounded-4 border border-secondary">
                            <div class="form-check mb-2">
                                <input class="form-check-input" type="radio" name="payment_status" id="payCash" value="paid" checked>
                                <label class="form-check-label text-success fw-bold fs-7" for="payCash">
                                    <i class="fa-solid fa-money-bill-wave me-1"></i> واصل نقداً (تم استلام المبلغ وقبضه)
                                </label>
                            </div>
                            <div class="form-check">
                                <input class="form-check-input" type="radio" name="payment_status" id="payDebt" value="unpaid">
                                <label class="form-check-label text-danger fw-bold fs-7" for="payDebt">
                                    <i class="fa-solid fa-clock-rotate-left me-1"></i> غير واصل / تسجيل دين على المشترك
                                </label>
                            </div>
                        </div>
                    </div>

                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">ملاحظات العملية (اختياري)</label>
                        <input type="text" name="notes" class="form-control bg-dark border-secondary text-light" placeholder="مثال: تجديد شهري يدوي">
                    </div>
                </div>
                <div class="modal-footer border-secondary">
                    <button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal">إلغاء</button>
                    <button type="submit" class="btn btn-success px-4 fw-bold">
                        <i class="fa-solid fa-check me-1"></i> تأكيد وتجديد الاشتراك
                    </button>
                </div>
            </form>
        </div>
    </div>
</div>

<!-- 2. ADD USER MODAL -->
<div class="modal fade" id="addUserModal" tabindex="-1" aria-labelledby="addUserModalLabel" aria-hidden="true">
    <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content glass-card border-secondary text-light">
            <div class="modal-header border-secondary">
                <h5 class="modal-title fw-bold" id="addUserModalLabel">
                    <i class="fa-solid fa-user-plus me-2 text-primary"></i> إضافة مشترك جديد
                </h5>
                <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" aria-label="Close"></button>
            </div>
            <form action="/users" method="POST">
                <div class="modal-body">
                    <div class="row g-2 mb-3">
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">اسم المستخدم (User) *</label>
                            <input type="text" name="user" class="form-control bg-dark border-secondary text-light font-monospace" required placeholder="ahmed123">
                        </div>
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">كلمة المرور (Password) *</label>
                            <input type="text" name="pass" class="form-control bg-dark border-secondary text-light font-monospace" required placeholder="123456" value="123456">
                        </div>
                    </div>
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">الباقة *</label>
                        <select name="profile" class="form-select bg-dark border-secondary text-light" required>
                            <option value="">-- اختر الباقة --</option>
                            <?php foreach ($profiles as $p): ?>
                            <option value="<?= htmlspecialchars($p['name']) ?>"><?= htmlspecialchars($p['name']) ?> (<?= number_format($p['price'] ?? 0, 0) ?> د.ع)</option>
                            <?php endforeach; ?>
                        </select>
                    </div>
                    <div class="row g-2 mb-3">
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">الاسم الكامل</label>
                            <input type="text" name="full_name" class="form-control bg-dark border-secondary text-light" placeholder="أحمد علي">
                        </div>
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">رقم الهاتف</label>
                            <input type="text" name="phone" class="form-control bg-dark border-secondary text-light font-monospace" placeholder="07701234567">
                        </div>
                    </div>
                </div>
                <div class="modal-footer border-secondary">
                    <button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal">إلغاء</button>
                    <button type="submit" class="btn btn-primary px-4 fw-bold">حفظ المشترك</button>
                </div>
            </form>
        </div>
    </div>
</div>

<!-- 3. EDIT USER MODAL -->
<div class="modal fade" id="editUserModal" tabindex="-1" aria-labelledby="editUserModalLabel" aria-hidden="true">
    <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content glass-card border-primary text-light">
            <div class="modal-header border-secondary">
                <h5 class="modal-title fw-bold text-primary" id="editUserModalLabel">
                    <i class="fa-solid fa-user-pen me-2"></i> تعديل بيانات المشترك
                </h5>
                <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" aria-label="Close"></button>
            </div>
            <form action="/users" method="POST">
                <input type="hidden" name="old_user" id="editOldUser">
                <div class="modal-body">
                    <div class="row g-2 mb-3">
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">اسم المستخدم (User) *</label>
                            <input type="text" name="user" id="editUser" class="form-control bg-dark border-secondary text-light font-monospace" required>
                        </div>
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">كلمة المرور *</label>
                            <input type="text" name="pass" id="editPass" class="form-control bg-dark border-secondary text-light font-monospace" required>
                        </div>
                    </div>
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">الباقة *</label>
                        <select name="profile" id="editProfile" class="form-select bg-dark border-secondary text-light" required>
                            <?php foreach ($profiles as $p): ?>
                            <option value="<?= htmlspecialchars($p['name']) ?>"><?= htmlspecialchars($p['name']) ?></option>
                            <?php endforeach; ?>
                        </select>
                    </div>
                    <div class="row g-2 mb-3">
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">الاسم الكامل</label>
                            <input type="text" name="full_name" id="editFullName" class="form-control bg-dark border-secondary text-light">
                        </div>
                        <div class="col-6">
                            <label class="form-label fs-7 fw-semibold">رقم الهاتف</label>
                            <input type="text" name="phone" id="editPhone" class="form-control bg-dark border-secondary text-light font-monospace">
                        </div>
                    </div>
                </div>
                <div class="modal-footer border-secondary">
                    <button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal">إلغاء</button>
                    <button type="submit" class="btn btn-primary px-4 fw-bold">تحديث البيانات</button>
                </div>
            </form>
        </div>
    </div>
</div>

<!-- 4. USER DETAILS & SESSIONS INSPECTOR MODAL -->
<div class="modal fade" id="userDetailsModal" tabindex="-1" aria-hidden="true">
    <div class="modal-dialog modal-dialog-centered modal-lg">
        <div class="modal-content glass-card border-info text-light shadow-lg">
            <div class="modal-header border-secondary pb-3">
                <div class="d-flex align-items-center gap-3">
                    <div class="user-avatar-circle" id="detailsModalAvatar">
                        U
                    </div>
                    <div>
                        <div class="d-flex align-items-center gap-2">
                            <h5 class="modal-title fw-bold text-info font-monospace mb-0" id="detailsModalUser">@username</h5>
                            <span id="detailsModalStatusBadge" class="badge bg-secondary fs-9 px-2">جاري التحميل...</span>
                        </div>
                        <small class="text-white-50 fs-8" id="detailsModalFullName">الاسم الكامل</small>
                    </div>
                </div>
                <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" aria-label="Close"></button>
            </div>

            <!-- Tabbed Navigation -->
            <div class="modal-body p-4">
                <ul class="nav nav-pills nav-fill bg-dark p-1 rounded-4 border border-secondary border-opacity-50 mb-4" id="detailsTab" role="tablist">
                    <li class="nav-item" role="presentation">
                        <button class="nav-link active rounded-pill fw-bold fs-8" id="tab-info-btn" data-bs-toggle="pill" data-bs-target="#tab-info" type="button" role="tab">
                            <i class="fa-solid fa-id-card me-1"></i> بيانات المشترك
                        </button>
                    </li>
                    <li class="nav-item" role="presentation">
                        <button class="nav-link rounded-pill fw-bold fs-8" id="tab-session-btn" data-bs-toggle="pill" data-bs-target="#tab-session" type="button" role="tab">
                            <i class="fa-solid fa-bolt me-1 text-warning"></i> الجلسة الحالية
                        </button>
                    </li>
                    <li class="nav-item" role="presentation">
                        <button class="nav-link rounded-pill fw-bold fs-8" id="tab-history-btn" data-bs-toggle="pill" data-bs-target="#tab-history" type="button" role="tab">
                            <i class="fa-solid fa-clock-rotate-left me-1 text-info"></i> سجل الجلسات
                        </button>
                    </li>
                    <li class="nav-item" role="presentation">
                        <button class="nav-link rounded-pill fw-bold fs-8" id="tab-trans-btn" data-bs-toggle="pill" data-bs-target="#tab-trans" type="button" role="tab">
                            <i class="fa-solid fa-book-bookmark me-1 text-success"></i> السجل المالي
                        </button>
                    </li>
                </ul>

                <div class="tab-content" id="detailsTabContent">
                    <!-- Tab 1: Basic Info & Identity -->
                    <div class="tab-pane fade show active" id="tab-info" role="tabpanel">
                        <div class="row g-3">
                            <div class="col-12 col-md-7">
                                <div class="p-3 rounded-4 bg-dark bg-opacity-75 border border-secondary h-100">
                                    <h6 class="fw-bold text-info mb-3 border-bottom border-secondary pb-2"><i class="fa-solid fa-circle-info me-2"></i> معلومات الحساب والاشتراك</h6>
                                    <div class="d-flex justify-content-between align-items-center py-2 border-bottom border-secondary border-opacity-50 fs-8">
                                        <span class="text-muted">اسم المستخدم:</span>
                                        <strong class="text-light font-monospace" id="detUser">—</strong>
                                    </div>
                                    <div class="d-flex justify-content-between align-items-center py-2 border-bottom border-secondary border-opacity-50 fs-8">
                                        <span class="text-muted">كلمة المرور:</span>
                                        <div class="d-flex align-items-center gap-1">
                                            <code class="pro-pass-code" id="detPass">—</code>
                                            <button class="btn btn-sm btn-link text-muted p-0" onclick="copyPassword(document.getElementById('detPass').innerText, this)"><i class="fa-solid fa-copy fs-9"></i></button>
                                        </div>
                                    </div>
                                    <div class="d-flex justify-content-between align-items-center py-2 border-bottom border-secondary border-opacity-50 fs-8">
                                        <span class="text-muted">الباقة المخصصة:</span>
                                        <span class="badge bg-primary-subtle text-primary border border-primary px-2" id="detProfile">—</span>
                                    </div>
                                    <div class="d-flex justify-content-between align-items-center py-2 border-bottom border-secondary border-opacity-50 fs-8">
                                        <span class="text-muted">تاريخ الانتهاء:</span>
                                        <strong class="text-warning font-monospace" id="detExpiry">—</strong>
                                    </div>
                                    <div class="d-flex justify-content-between align-items-center py-2 border-bottom border-secondary border-opacity-50 fs-8">
                                        <span class="text-muted">رقم الهاتف:</span>
                                        <div class="d-flex align-items-center gap-2">
                                            <span class="text-light font-monospace" id="detPhone">—</span>
                                            <a href="#" id="detPhoneWa" target="_blank" class="text-success fs-8"><i class="fa-brands fa-whatsapp"></i></a>
                                        </div>
                                    </div>
                                    <div class="d-flex justify-content-between align-items-center py-2 fs-8">
                                        <span class="text-muted">الرصيد / الدين:</span>
                                        <span id="detBalanceBadge" class="badge bg-success-subtle text-success border border-success font-monospace">خالص (0)</span>
                                    </div>
                                </div>
                            </div>
                            <div class="col-12 col-md-5">
                                <div class="p-3 rounded-4 bg-dark bg-opacity-75 border border-secondary text-center h-100 d-flex flex-column align-items-center justify-content-center">
                                    <div class="bg-white p-2 rounded-3 shadow mb-2">
                                        <img id="detQR" src="" alt="QR" class="img-fluid" style="max-width: 130px; height: auto;">
                                    </div>
                                    <strong class="text-light fs-8 d-block mb-1">بوابة المشترك السريعة</strong>
                                    <small class="text-muted fs-9">امسح الكود لتسجيل الدخول الفوري للبوابة</small>
                                </div>
                            </div>
                        </div>
                    </div>

                    <!-- Tab 2: Live Current Session -->
                    <div class="tab-pane fade" id="tab-session" role="tabpanel">
                        <div id="liveSessionContainer">
                            <!-- Populated dynamically -->
                        </div>
                    </div>

                    <!-- Tab 3: Session History (Radacct) -->
                    <div class="tab-pane fade" id="tab-history" role="tabpanel">
                        <div class="table-responsive rounded-4 border border-secondary border-opacity-50" style="max-height: 320px; overflow-y: auto;">
                            <table class="table table-dark table-hover align-middle mb-0 fs-8">
                                <thead class="table-secondary sticky-top">
                                    <tr>
                                        <th>بدء الجلسة</th>
                                        <th>نهاية الجلسة</th>
                                        <th>عنوان IP</th>
                                        <th>تنزيل ⬇️</th>
                                        <th>رفع ⬆️</th>
                                        <th>المدة</th>
                                        <th>الماك MAC</th>
                                    </tr>
                                </thead>
                                <tbody id="sessionHistoryTableBody">
                                    <tr><td colspan="7" class="text-center py-4 text-muted">جاري تحميل سجل الجلسات...</td></tr>
                                </tbody>
                            </table>
                        </div>
                    </div>

                    <!-- Tab 4: Financial Transactions -->
                    <div class="tab-pane fade" id="tab-trans" role="tabpanel">
                        <div class="d-flex justify-content-between align-items-center mb-3">
                            <span class="text-muted fs-8">سجل الديون والمدفوعات والتجديدات</span>
                            <a href="/transactions" class="btn btn-sm btn-outline-primary rounded-pill px-3 fs-9">
                                <i class="fa-solid fa-book-bookmark me-1"></i> فتح السجل المالي العام
                            </a>
                        </div>
                        <div class="table-responsive rounded-4 border border-secondary border-opacity-50" style="max-height: 320px; overflow-y: auto;">
                            <table class="table table-dark table-hover align-middle mb-0 fs-8">
                                <thead class="table-secondary sticky-top">
                                    <tr>
                                        <th>النوع</th>
                                        <th>المبلغ</th>
                                        <th>ملاحظات وبيان العملية</th>
                                        <th>التاريخ</th>
                                    </tr>
                                </thead>
                                <tbody id="transactionsTableBody">
                                    <tr><td colspan="4" class="text-center py-4 text-muted">جاري تحميل السجل المالي...</td></tr>
                                </tbody>
                            </table>
                        </div>
                    </div>
                </div>
            </div>

            <div class="modal-footer border-secondary">
                <button type="button" class="btn btn-secondary rounded-pill px-4" data-bs-dismiss="modal">إغلاق</button>
            </div>
        </div>
    </div>
</div>

<style>
/* ============================================================
   PRO USERS TABLE STYLING
============================================================ */
.kpi-chip {
    transition: transform 0.2s ease, box-shadow 0.2s ease;
    cursor: pointer;
}
.kpi-chip:hover {
    transform: translateY(-3px);
    box-shadow: 0 6px 16px rgba(0, 0, 0, 0.25);
}

.active-filter {
    background-color: #3b82f6 !important;
    color: #ffffff !important;
    border-color: #3b82f6 !important;
    box-shadow: 0 0 12px rgba(59, 130, 246, 0.5);
}

.users-table-pro {
    font-size: 0.9rem;
}
.users-table-pro thead th {
    background: linear-gradient(180deg, #1e293b 0%, #0f172a 100%);
    color: #cbd5e1;
    font-weight: 700;
    padding: 14px 12px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.1);
    white-space: nowrap;
}
.users-table-pro tbody tr {
    transition: background 0.2s ease, transform 0.2s ease;
    border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}
.users-table-pro tbody tr:hover {
    background: rgba(255, 255, 255, 0.04) !important;
}

/* User Avatar */
.user-avatar-circle {
    width: 38px;
    height: 38px;
    border-radius: 12px;
    background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%);
    color: #ffffff;
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: 800;
    font-size: 1rem;
    box-shadow: 0 4px 10px rgba(59, 130, 246, 0.3);
    border: 1px solid rgba(255, 255, 255, 0.2);
}

.user-name-link {
    transition: color 0.2s ease;
}
.user-name-link:hover {
    color: #60a5fa !important;
    text-shadow: 0 0 8px rgba(96, 165, 250, 0.5);
}

/* Password Code */
.pro-pass-code {
    background: rgba(0, 0, 0, 0.4);
    border: 1px solid rgba(255, 255, 255, 0.1);
    color: #fbbf24;
    padding: 3px 8px;
    border-radius: 6px;
    font-size: 0.8rem;
}

/* Profile Pill */
.profile-pill {
    background: rgba(59, 130, 246, 0.15);
    color: #60a5fa;
    border: 1px solid rgba(59, 130, 246, 0.3);
    padding: 5px 10px;
    border-radius: 8px;
    font-size: 0.8rem;
}

/* Debt & Paid Badges */
.debt-badge {
    background: rgba(239, 68, 68, 0.15);
    color: #f87171;
    border: 1px solid rgba(239, 68, 68, 0.3);
    padding: 5px 10px;
    border-radius: 8px;
    font-size: 0.8rem;
}
.paid-badge {
    background: rgba(16, 185, 129, 0.15);
    color: #34d399;
    border: 1px solid rgba(16, 185, 129, 0.3);
    padding: 5px 10px;
    border-radius: 8px;
    font-size: 0.8rem;
}

/* Online Pulse Dot */
.online-pulse-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background-color: #10b981;
    box-shadow: 0 0 8px #10b981;
    animation: pulseDot 1.5s infinite;
}
@keyframes pulseDot {
    0% { transform: scale(0.9); opacity: 0.7; }
    50% { transform: scale(1.3); opacity: 1; box-shadow: 0 0 12px #10b981; }
    100% { transform: scale(0.9); opacity: 0.7; }
}

/* Action Buttons Group */
.action-btn-group .btn {
    padding: 5px 9px;
    font-size: 0.8rem;
    border: 1px solid rgba(255, 255, 255, 0.1);
    background: rgba(15, 23, 42, 0.6);
    color: #cbd5e1;
    transition: all 0.2s ease;
}
.btn-action-renew:hover { background: #10b981 !important; color: #ffffff !important; border-color: #10b981 !important; }
.btn-action-edit:hover { background: #3b82f6 !important; color: #ffffff !important; border-color: #3b82f6 !important; }
.btn-action-card:hover { background: #8b5cf6 !important; color: #ffffff !important; border-color: #8b5cf6 !important; }
.btn-action-kick:hover { background: #06b6d4 !important; color: #ffffff !important; border-color: #06b6d4 !important; }
.btn-action-disable { color: #f59e0b !important; }
.btn-action-disable:hover { background: #f59e0b !important; color: #ffffff !important; border-color: #f59e0b !important; box-shadow: 0 0 10px rgba(245, 158, 11, 0.6) !important; }
.btn-action-enable { color: #10b981 !important; }
.btn-action-enable:hover { background: #10b981 !important; color: #ffffff !important; border-color: #10b981 !important; box-shadow: 0 0 10px rgba(16, 185, 129, 0.6) !important; }
.btn-action-delete:hover { background: #ef4444 !important; color: #ffffff !important; border-color: #ef4444 !important; }
</style>

<script>
let currentFilter = 'all';

function setFilter(filter, btn) {
    currentFilter = filter;
    document.querySelectorAll('#filterPills button').forEach(b => b.classList.remove('active-filter'));
    if (btn) btn.classList.add('active-filter');
    filterUsersTable();
}

function setQuickFilter(filter) {
    const pills = document.querySelectorAll('#filterPills button');
    pills.forEach(p => {
        if (p.getAttribute('onclick') && p.getAttribute('onclick').includes("'" + filter + "'")) {
            setFilter(filter, p);
        }
    });
}

function filterUsersTable() {
    const search = (document.getElementById('userSearchInput').value || '').toLowerCase().trim();
    const rows = document.querySelectorAll('.user-row');
    let visibleCount = 0;

    rows.forEach(row => {
        const u = row.getAttribute('data-username') || '';
        const f = row.getAttribute('data-fullname') || '';
        const p = row.getAttribute('data-phone') || '';
        const ip = row.getAttribute('data-ip') || '';
        const status = row.getAttribute('data-status') || '';
        const isOnline = row.getAttribute('data-online') === '1';
        const isAboutExp = row.getAttribute('data-aboutexpire') === '1';

        let matchesFilter = true;
        if (currentFilter === 'active') {
            matchesFilter = (status === 'active');
        } else if (currentFilter === 'expired') {
            matchesFilter = (status === 'expired');
        } else if (currentFilter === 'online') {
            matchesFilter = isOnline;
        } else if (currentFilter === 'about_to_expire') {
            matchesFilter = isAboutExp;
        }

        let matchesSearch = true;
        if (search) {
            matchesSearch = u.includes(search) || f.includes(search) || p.includes(search) || ip.includes(search);
        }

        if (matchesFilter && matchesSearch) {
            row.style.display = '';
            visibleCount++;
        } else {
            row.style.display = 'none';
        }
    });

    const badge = document.getElementById('usersCountBadge');
    if (badge) badge.innerText = visibleCount;
}

function copyPassword(pass, btn) {
    navigator.clipboard.writeText(pass);
    const icon = btn.querySelector('i');
    if (icon) {
        icon.className = 'fa-solid fa-check text-success fs-8';
        setTimeout(() => {
            icon.className = 'fa-solid fa-copy fs-8';
        }, 1500);
    }
}

function openRenewModal(username, currentProfile, expiry) {
    document.getElementById('renewModalUsername').innerText = '@' + username;
    document.getElementById('renewForm').action = '/users/' + encodeURIComponent(username) + '/renew';
    
    const sel = document.getElementById('renewProfileSelect');
    if (sel) {
        for (let i = 0; i < sel.options.length; i++) {
            if (sel.options[i].value === currentProfile) {
                sel.selectedIndex = i;
                break;
            }
        }
    }
    updateRenewPrice();
    new bootstrap.Modal(document.getElementById('renewUserModal')).show();
}

function updateRenewPrice() {
    const sel = document.getElementById('renewProfileSelect');
    if (!sel || sel.selectedIndex < 0) return;
    const opt = sel.options[sel.selectedIndex];
    const price = opt.getAttribute('data-price') || '0';
    const days = opt.getAttribute('data-days') || '30';
    
    document.getElementById('renewPriceInput').value = price;
    document.getElementById('renewDaysInput').value = days;
}

function openEditModal(user, pass, profile, fullName, phone) {
    document.getElementById('editOldUser').value = user;
    document.getElementById('editUser').value = user;
    document.getElementById('editPass').value = pass;
    document.getElementById('editFullName').value = fullName;
    document.getElementById('editPhone').value = phone;
    
    const sel = document.getElementById('editProfile');
    if (sel) {
        for (let i = 0; i < sel.options.length; i++) {
            if (sel.options[i].value === profile) {
                sel.selectedIndex = i;
                break;
            }
        }
    }
    new bootstrap.Modal(document.getElementById('editUserModal')).show();
}

async function openUserDetailsModal(user, fullName, profile, expiry, phone, balance) {
    // 1. Initial Quick Fill
    document.getElementById('detailsModalAvatar').innerText = user.charAt(0).toUpperCase();
    document.getElementById('detailsModalUser').innerText = '@' + user;
    document.getElementById('detailsModalFullName').innerText = fullName || 'مشترك في الشبكة';
    document.getElementById('detailsModalStatusBadge').className = 'badge bg-secondary fs-9 px-2';
    document.getElementById('detailsModalStatusBadge').innerText = 'جاري التحميل...';
    
    document.getElementById('detUser').innerText = '@' + user;
    document.getElementById('detPass').innerText = '...';
    document.getElementById('detProfile').innerText = profile || 'افتراضي';
    document.getElementById('detExpiry').innerText = expiry || '—';
    document.getElementById('detPhone').innerText = phone || '—';
    document.getElementById('detPhoneWa').href = phone ? 'https://wa.me/' + phone.replace(/[^0-9]/g, '') : '#';
    
    const balVal = Number(balance || 0);
    const balBadge = document.getElementById('detBalanceBadge');
    if (balVal > 0) {
        balBadge.className = 'badge bg-danger-subtle text-danger border border-danger font-monospace';
        balBadge.innerText = 'مطلوب ' + balVal.toLocaleString() + ' د.ع';
    } else {
        balBadge.className = 'badge bg-success-subtle text-success border border-success font-monospace';
        balBadge.innerText = 'خالص (0)';
    }

    document.getElementById('detQR').src = 'https://api.qrserver.com/v1/create-qr-code/?size=130x130&data=' + encodeURIComponent(window.location.origin + '/portal?username=' + user);
    
    // Switch to first tab
    const firstTabBtn = document.getElementById('tab-info-btn');
    if (firstTabBtn) bootstrap.Tab.getOrCreateInstance(firstTabBtn).show();

    // Show modal immediately
    const modalEl = document.getElementById('userDetailsModal');
    const bsModal = bootstrap.Modal.getOrCreateInstance(modalEl);
    bsModal.show();

    // 2. Fetch Full Details Asynchronously
    try {
        const resp = await fetch('/users/' + encodeURIComponent(user) + '/details');
        if (!resp.ok) return;
        const data = await resp.json();
        
        // Update Password
        if (data.pass) document.getElementById('detPass').innerText = data.pass;
        
        // Update Status Badge
        const s = data.session || {};
        const isOnline = s.online || s.status === 'online';
        const isExp = data.expired;
        
        const statusBadge = document.getElementById('detailsModalStatusBadge');
        if (isOnline) {
            statusBadge.className = 'badge bg-success-subtle text-success border border-success fs-9 px-2';
            statusBadge.innerHTML = '<i class="fa-solid fa-bolt me-1"></i> متصل الآن';
        } else if (isExp) {
            statusBadge.className = 'badge bg-danger-subtle text-danger border border-danger fs-9 px-2';
            statusBadge.innerHTML = '<i class="fa-solid fa-circle-xmark me-1"></i> منتهي';
        } else {
            statusBadge.className = 'badge bg-secondary-subtle text-secondary border border-secondary fs-9 px-2';
            statusBadge.innerHTML = '<i class="fa-solid fa-circle-check me-1"></i> نشط (غير متصل)';
        }

        // Render Live Session Tab
        const liveContainer = document.getElementById('liveSessionContainer');
        if (isOnline || s.ip || s.session_seconds) {
            liveContainer.innerHTML = `
                <div class="p-3 rounded-4 bg-dark bg-opacity-75 border border-secondary mb-3">
                    <div class="d-flex justify-content-between align-items-center mb-3">
                        <div class="d-flex align-items-center gap-2">
                            <span class="online-pulse-dot"></span>
                            <strong class="text-success fs-7">جلسة متصلة ونشطة في المايكروتك</strong>
                        </div>
                        <a href="/users/${encodeURIComponent(user)}/disconnect" class="btn btn-sm btn-outline-danger rounded-pill px-3" onclick="return confirm('هل تريد فصل جلسة المشترك الآن؟');">
                            <i class="fa-solid fa-plug-circle-xmark me-1"></i> فصل الجلسة الآن
                        </a>
                    </div>
                    <div class="row g-2 text-center mb-3">
                        <div class="col-6 col-md-3">
                            <div class="p-2 rounded-3 bg-dark border border-secondary border-opacity-50">
                                <small class="text-muted fs-9 d-block">عنوان IP</small>
                                <strong class="text-info font-monospace fs-8">${s.ip || '—'}</strong>
                            </div>
                        </div>
                        <div class="col-6 col-md-3">
                            <div class="p-2 rounded-3 bg-dark border border-secondary border-opacity-50">
                                <small class="text-muted fs-9 d-block">الماك أدرس (MAC)</small>
                                <strong class="text-light font-monospace fs-9">${s.calling_station || '—'}</strong>
                            </div>
                        </div>
                        <div class="col-6 col-md-3">
                            <div class="p-2 rounded-3 bg-dark border border-secondary border-opacity-50">
                                <small class="text-muted fs-9 d-block">مدة الاتصال (Uptime)</small>
                                <strong class="text-success font-monospace fs-8">⏱️ ${s.uptime || formatSeconds(s.session_seconds || 0)}</strong>
                            </div>
                        </div>
                        <div class="col-6 col-md-3">
                            <div class="p-2 rounded-3 bg-dark border border-secondary border-opacity-50">
                                <small class="text-muted fs-9 d-block">البيانات (⬇️/⬆️)</small>
                                <strong class="text-warning font-monospace fs-9">${s.download || '0 B'} / ${s.upload || '0 B'}</strong>
                            </div>
                        </div>
                    </div>
                    <div class="d-flex flex-wrap justify-content-between text-muted fs-9 pt-2 border-top border-secondary border-opacity-50">
                        <span>سيرفر المايكروتك (NAS IP): <code class="text-info">${s.nas_ip || '127.0.0.1'}</code></span>
                        <span>معرف الجلسة (Acct-Session-Id): <code class="text-light font-monospace">${s.session_id || '—'}</code></span>
                    </div>
                </div>
            `;
        } else {
            liveContainer.innerHTML = `
                <div class="text-center py-5 text-muted">
                    <i class="fa-solid fa-plug-circle-xmark fs-1 d-block mb-3 text-secondary opacity-50"></i>
                    <h6>لا توجد جلسة نشطة حالياً</h6>
                    <small class="text-muted">المشترك غير متصل بالراوتر في الوقت الحالي</small>
                </div>
            `;
        }

        // Render Sessions History Tab
        const histBody = document.getElementById('sessionHistoryTableBody');
        const sessHistory = data.session_history || [];
        if (sessHistory.length > 0) {
            histBody.innerHTML = sessHistory.map(h => `
                <tr>
                    <td class="text-light font-monospace fs-9">${h.started_at || '—'}</td>
                    <td class="text-muted font-monospace fs-9">${h.stopped_at || '<span class="badge bg-success fs-9">متصلة الآن</span>'}</td>
                    <td><code class="text-info">${h.ip || '—'}</code></td>
                    <td class="text-success font-monospace">${h.download || '0 B'}</td>
                    <td class="text-primary font-monospace">${h.upload || '0 B'}</td>
                    <td class="text-light font-monospace">${h.session_time ? formatSeconds(h.session_time) : '—'}</td>
                    <td class="text-muted font-monospace fs-9">${h.calling_station || '—'}</td>
                </tr>
            `).join('');
        } else {
            histBody.innerHTML = `<tr><td colspan="7" class="text-center py-4 text-muted">لا يوجد سجل جلسات سابقة مسجل</td></tr>`;
        }

        // Render Financial Transactions Tab
        const transBody = document.getElementById('transactionsTableBody');
        const transactions = data.transactions || [];
        if (transactions.length > 0) {
            transBody.innerHTML = transactions.map(t => `
                <tr>
                    <td>
                        <span class="badge ${t.type === 'payment' ? 'bg-success-subtle text-success border border-success' : 'bg-danger-subtle text-danger border border-danger'} px-2 py-1">
                            ${t.type === 'payment' ? '<i class="fa-solid fa-check me-1"></i> تسديد / دفع' : '<i class="fa-solid fa-clock-rotate-left me-1"></i> قيد دين'}
                        </span>
                    </td>
                    <td class="font-monospace fw-bold ${t.type === 'payment' ? 'text-success' : 'text-danger'}">${Number(t.amount || 0).toLocaleString()} د.ع</td>
                    <td class="text-light fs-8">${t.notes || '—'}</td>
                    <td class="text-muted font-monospace fs-9">${t.created_at ? new Date(t.created_at).toLocaleString('ar-IQ') : '—'}</td>
                </tr>
            `).join('');
        } else {
            transBody.innerHTML = `<tr><td colspan="4" class="text-center py-4 text-muted">لا توجد حركات مالية مسجلة</td></tr>`;
        }

    } catch (e) {
        console.error('Failed to load user details:', e);
    }
}

function formatSeconds(secs) {
    if (!secs || secs <= 0) return '0ث';
    const h = Math.floor(secs / 3600);
    const m = Math.floor((secs % 3600) / 60);
    const s = secs % 60;
    if (h > 0) return `${h}س ${m}د`;
    if (m > 0) return `${m}د ${s}ث`;
    return `${s}ث`;
}
</script>
