<?php
$totalResellerBalance = 0;
$agentCount = 0;
foreach ($admins as $a) {
    if (($a['role'] ?? '') !== 'superadmin') {
        $agentCount++;
        $totalResellerBalance += (float)($a['balance'] ?? 0);
    }
}
?>

<div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
    <div>
        <h4 class="fw-bold text-light mb-1"><i class="fa-solid fa-user-gear me-2 text-warning"></i> إدارة الموزعين ومصفوفة الصلاحيات (RBAC Studio)</h4>
        <p class="text-muted mb-0 fs-7">إنشاء حسابات الموزعين والمحلات، شحن المحافظ، والتحكم الدقيق بصلاحيات كل وكيل</p>
    </div>
    <div class="d-flex gap-2">
        <button class="btn btn-warning rounded-pill px-4 fw-bold shadow-sm" data-bs-toggle="modal" data-bs-target="#rechargeModal">
            <i class="fa-solid fa-wallet me-2"></i> شحن رصيد وكيل
        </button>
        <button class="btn btn-primary rounded-pill px-4 fw-bold shadow-sm" data-bs-toggle="modal" data-bs-target="#addAdminModal">
            <i class="fa-solid fa-user-plus me-2"></i> إضافة موزع جديد
        </button>
    </div>
</div>

<!-- KPI Cards -->
<div class="row g-3 mb-4">
    <div class="col-12 col-md-4">
        <div class="card glass-card border-warning border-opacity-50 shadow-sm rounded-4 p-3 h-100">
            <div class="d-flex align-items-center justify-content-between">
                <div>
                    <span class="text-muted fs-7 fw-semibold d-block mb-1">إجمالي أرصدة محافظ الوكلاء</span>
                    <h3 class="fw-bold text-warning mb-0"><?= number_format($totalResellerBalance, 0) ?> د.ع</h3>
                </div>
                <div class="stat-icon bg-warning-subtle text-warning rounded-3 p-3">
                    <i class="fa-solid fa-vault fs-3"></i>
                </div>
            </div>
        </div>
    </div>

    <div class="col-12 col-md-4">
        <div class="card glass-card border-info border-opacity-50 shadow-sm rounded-4 p-3 h-100">
            <div class="d-flex align-items-center justify-content-between">
                <div>
                    <span class="text-muted fs-7 fw-semibold d-block mb-1">عدد الوكلاء والموزعين</span>
                    <h3 class="fw-bold text-info mb-0"><?= $agentCount ?> وكيل</h3>
                </div>
                <div class="stat-icon bg-info-subtle text-info rounded-3 p-3">
                    <i class="fa-solid fa-users-gear fs-3"></i>
                </div>
            </div>
        </div>
    </div>

    <div class="col-12 col-md-4">
        <div class="card glass-card border-0 shadow-sm rounded-4 p-3 h-100">
            <div class="d-flex align-items-center justify-content-between">
                <div>
                    <span class="text-muted fs-7 fw-semibold d-block mb-1">إجمالي الحسابات الإدارية</span>
                    <h3 class="fw-bold text-light mb-0"><?= count($admins) ?> حساب</h3>
                </div>
                <div class="stat-icon bg-secondary rounded-3 p-3">
                    <i class="fa-solid fa-shield-halved fs-3 text-light"></i>
                </div>
            </div>
        </div>
    </div>
</div>

<!-- Tabs -->
<ul class="nav nav-pills mb-3 gap-2" id="adminTabs" role="tablist">
    <li class="nav-item" role="presentation">
        <button class="nav-link active fw-bold rounded-pill px-4" id="admins-list-tab" data-bs-toggle="pill" data-bs-target="#admins-list-pane" type="button" role="tab">
            <i class="fa-solid fa-users me-2"></i> قائمة الوكلاء والموزعين
        </button>
    </li>
    <li class="nav-item" role="presentation">
        <button class="nav-link fw-bold rounded-pill px-4" id="admins-trans-tab" data-bs-toggle="pill" data-bs-target="#admins-trans-pane" type="button" role="tab">
            <i class="fa-solid fa-clock-rotate-left me-2"></i> سجل حركات شحن محافظ الوكلاء
        </button>
    </li>
</ul>

<div class="tab-content" id="adminTabsContent">
    <!-- Tab 1: Admins Table -->
    <div class="tab-pane fade show active" id="admins-list-pane" role="tabpanel">
        <div class="card glass-card border-0 shadow-sm rounded-4 p-4">
            <div class="table-responsive">
                <table class="table table-dark table-hover align-middle mb-0">
                    <thead class="table-secondary">
                        <tr>
                            <th>#</th>
                            <th>اسم المستخدم</th>
                            <th>اسم الوكيل / المحل</th>
                            <th>الدور</th>
                            <th>ملخص الصلاحيات الحبيبية (RBAC)</th>
                            <th>رصيد المحفظة</th>
                            <th>تاريخ التسجيل</th>
                            <th class="text-center">الإجراءات</th>
                        </tr>
                    </thead>
                    <tbody>
                        <?php if (empty($admins)): ?>
                        <tr>
                            <td colspan="8" class="text-center py-5 text-muted">
                                <i class="fa-solid fa-user-slash fs-1 d-block mb-2 text-secondary"></i>
                                لا يوجد وكلاء مسجلون حالياً
                            </td>
                        </tr>
                        <?php else: ?>
                        <?php foreach ($admins as $i => $a): ?>
                        <?php
                            $aId = $a['id'] ?? $i;
                            $uName = $a['username'] ?? '';
                            $aName = $a['name'] ?? '—';
                            $role = $a['role'] ?? 'agent';
                            $bal = (float)($a['balance'] ?? 0);
                            $isSuper = ($role === 'superadmin');
                            
                            $perms = [
                                'can_create_users' => !empty($a['can_create_users']),
                                'can_edit_users' => !empty($a['can_edit_users']),
                                'can_delete_users' => !empty($a['can_delete_users']),
                                'can_toggle_users' => !empty($a['can_toggle_users']),
                                'can_disconnect_users' => !empty($a['can_disconnect_users']),
                                'can_renew_users' => !empty($a['can_renew_users']),
                                'can_generate_vouchers' => !empty($a['can_generate_vouchers']),
                                'can_delete_vouchers' => !empty($a['can_delete_vouchers']),
                                'can_print_vouchers' => !empty($a['can_print_vouchers']),
                                'can_manage_profiles' => !empty($a['can_manage_profiles']),
                                'can_manage_nas' => !empty($a['can_manage_nas']),
                                'can_manage_devices' => !empty($a['can_manage_devices']),
                                'can_manage_transactions' => !empty($a['can_manage_transactions']),
                                'can_manage_subagents' => !empty($a['can_manage_subagents']),
                                'can_view_logs' => !empty($a['can_view_logs']),
                                'can_clear_logs' => !empty($a['can_clear_logs']),
                                'can_manage_whatsapp' => !empty($a['can_manage_whatsapp']),
                                'can_manage_streams' => !empty($a['can_manage_streams']),
                            ];
                        ?>
                        <tr>
                            <td class="text-muted fs-8"><?= $i + 1 ?></td>
                            <td class="fw-bold text-info font-monospace fs-6">
                                <i class="fa-solid fa-user-shield me-1 <?= $isSuper ? 'text-danger' : 'text-warning' ?>"></i>
                                @<?= htmlspecialchars($uName) ?>
                            </td>
                            <td><?= htmlspecialchars($aName) ?></td>
                            <td>
                                <?php if ($isSuper): ?>
                                <span class="badge bg-danger-subtle text-danger border border-danger px-2 py-1"><i class="fa-solid fa-crown me-1"></i> مدير عام (Superadmin)</span>
                                <?php else: ?>
                                <span class="badge bg-warning-subtle text-warning border border-warning px-2 py-1"><i class="fa-solid fa-user-tag me-1"></i> وكيل فرعي (Agent)</span>
                                <?php endif; ?>
                            </td>
                            <td>
                                <div class="d-flex gap-1 flex-wrap align-items-center">
                                    <?php if ($isSuper): ?>
                                    <span class="badge bg-success-subtle text-success border border-success fs-8">صلاحيات كاملة 100%</span>
                                    <?php else: ?>
                                        <?php if (!empty($perms['can_create_users'])): ?><span class="badge bg-primary-subtle text-primary border border-primary fs-9">إضافة مشتركين</span><?php endif; ?>
                                        <?php if (!empty($perms['can_delete_users'])): ?><span class="badge bg-danger-subtle text-danger border border-danger fs-9">حذف مشتركين</span><?php endif; ?>
                                        <?php if (!empty($perms['can_generate_vouchers'])): ?><span class="badge bg-warning-subtle text-warning border border-warning fs-9">توليد كروت</span><?php endif; ?>
                                        <?php if (!empty($perms['can_manage_profiles'])): ?><span class="badge bg-info-subtle text-info border border-info fs-9">باقات</span><?php endif; ?>
                                        <?php if (!empty($perms['can_manage_nas'])): ?><span class="badge bg-cyan-subtle text-info border border-info fs-9">مايكروتك</span><?php endif; ?>
                                        <?php if (!empty($perms['can_view_logs'])): ?><span class="badge bg-secondary border border-secondary fs-9">سجل الرقابة</span><?php endif; ?>
                                    <?php endif; ?>
                                </div>
                            </td>
                            <td class="fw-bold text-success fs-6"><?= number_format($bal, 0) ?> د.ع</td>
                            <td class="fs-7 text-muted"><?= htmlspecialchars($a['created_at'] ?? '—') ?></td>
                            <td class="text-center">
                                <?php if (!$isSuper): ?>
                                <div class="btn-group btn-group-sm">
                                    <button type="button" class="btn btn-outline-primary" onclick='openPermissionsModal(<?= json_encode($a) ?>)' title="تعديل الصلاحيات الحبيبية">
                                        <i class="fa-solid fa-sliders me-1"></i> الصلاحيات
                                    </button>
                                    <button type="button" class="btn btn-outline-warning" onclick="openAgentRecharge(<?= $aId ?>, '<?= htmlspecialchars($uName, ENT_QUOTES) ?>', <?= $bal ?>)" title="شحن رصيد">
                                        <i class="fa-solid fa-plus me-1"></i> شحن
                                    </button>
                                    <button type="button" class="btn btn-outline-info" onclick="openAgentWithdraw(<?= $aId ?>, '<?= htmlspecialchars($uName, ENT_QUOTES) ?>', <?= $bal ?>)" title="سحب رصيد">
                                        <i class="fa-solid fa-minus me-1"></i> سحب
                                    </button>
                                    <a href="/admins/<?= urlencode($aId) ?>/delete" class="btn btn-outline-danger" onclick="return confirm('هل أنت متأكد من حذف حساب الوكيل @<?= htmlspecialchars($uName, ENT_QUOTES) ?>؟');" title="حذف">
                                        <i class="fa-solid fa-trash"></i>
                                    </a>
                                </div>
                                <?php else: ?>
                                <span class="badge bg-secondary">حساب محمي</span>
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

    <!-- Tab 2: Admin Transactions -->
    <div class="tab-pane fade" id="admins-trans-pane" role="tabpanel">
        <div class="card glass-card border-0 shadow-sm rounded-4 p-4">
            <h5 class="fw-bold text-light mb-3"><i class="fa-solid fa-clock-rotate-left me-2 text-warning"></i> سجل حركات شحن وسحب محافظ الوكلاء</h5>
            <div class="table-responsive">
                <table class="table table-dark table-hover align-middle mb-0">
                    <thead class="table-secondary">
                        <tr>
                            <th>#</th>
                            <th>الوكيل المستفيد</th>
                            <th>المنفذ للعملية</th>
                            <th>نوع الحركة</th>
                            <th>المبلغ</th>
                            <th>الملاحظات</th>
                            <th>التاريخ والوقت</th>
                        </tr>
                    </thead>
                    <tbody>
                        <?php if (empty($transactions)): ?>
                        <tr>
                            <td colspan="7" class="text-center py-5 text-muted">لا توجد حركات شحن محافظ مسجلة حتى الآن</td>
                        </tr>
                        <?php else: ?>
                        <?php foreach ($transactions as $i => $t): ?>
                        <tr>
                            <td class="text-muted fs-8"><?= $i + 1 ?></td>
                            <td class="fw-bold text-info font-monospace">@<?= htmlspecialchars($t['admin_username'] ?? '—') ?></td>
                            <td class="text-light fs-7">@<?= htmlspecialchars($t['performer_name'] ?? $t['performed_by_username'] ?? 'admin') ?></td>
                            <td>
                                <?php if (($t['transaction_type'] ?? '') === 'recharge'): ?>
                                <span class="badge bg-success-subtle text-success border border-success"><i class="fa-solid fa-plus me-1"></i> شحن رصيد</span>
                                <?php else: ?>
                                <span class="badge bg-danger-subtle text-danger border border-danger"><i class="fa-solid fa-minus me-1"></i> سحب رصيد</span>
                                <?php endif; ?>
                            </td>
                            <td class="fw-bold text-warning fs-6"><?= number_format($t['amount'] ?? 0, 0) ?> د.ع</td>
                            <td class="fs-8 text-light"><?= htmlspecialchars($t['notes'] ?? '—') ?></td>
                            <td class="fs-7 text-muted"><?= htmlspecialchars($t['created_at'] ?? '—') ?></td>
                        </tr>
                        <?php endforeach; ?>
                        <?php endif; ?>
                    </tbody>
                </table>
            </div>
        </div>
    </div>
</div>

<!-- Add Admin Modal -->
<div class="modal fade" id="addAdminModal" tabindex="-1" aria-labelledby="addAdminModalLabel" aria-hidden="true">
    <div class="modal-dialog modal-dialog-centered modal-lg">
        <div class="modal-content glass-card border-secondary text-light">
            <div class="modal-header border-secondary">
                <h5 class="modal-title fw-bold" id="addAdminModalLabel"><i class="fa-solid fa-user-plus me-2 text-primary"></i> إضافة موزع / وكيل فرعي جديد</h5>
                <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" aria-label="Close"></button>
            </div>
            <form action="/admins" method="POST">
                <div class="modal-body">
                    <div class="row g-3 mb-3">
                        <div class="col-md-6">
                            <label class="form-label fs-7 fw-semibold">اسم المستخدم (Login Username) *</label>
                            <input type="text" name="username" class="form-control bg-dark border-secondary text-light" required placeholder="agent1">
                        </div>
                        <div class="col-md-6">
                            <label class="form-label fs-7 fw-semibold">كلمة المرور *</label>
                            <input type="text" name="password" class="form-control bg-dark border-secondary text-light" required placeholder="••••••••">
                        </div>
                        <div class="col-md-6">
                            <label class="form-label fs-7 fw-semibold">اسم الوكيل / اسم المحل</label>
                            <input type="text" name="name" class="form-control bg-dark border-secondary text-light" placeholder="مركز الأمل للاتصالات">
                        </div>
                        <div class="col-md-6">
                            <label class="form-label fs-7 fw-semibold">البريد الإلكتروني (اختياري)</label>
                            <input type="email" name="email" class="form-control bg-dark border-secondary text-light" placeholder="agent@network.com">
                        </div>
                    </div>

                    <div class="p-3 bg-dark rounded-4 border border-secondary">
                        <div class="d-flex align-items-center justify-content-between mb-2">
                            <label class="form-label fs-7 fw-bold text-warning mb-0"><i class="fa-solid fa-sliders me-1"></i> تحديد الصلاحيات الافتراضية للوكيل:</label>
                            <div class="btn-group btn-group-sm">
                                <button type="button" class="btn btn-outline-info py-0 px-2 fs-9" onclick="applyPreset('add', 'reseller')">موزع كروت ومبيعات</button>
                                <button type="button" class="btn btn-outline-success py-0 px-2 fs-9" onclick="applyPreset('add', 'manager')">مدير فرع كامل</button>
                            </div>
                        </div>
                        
                        <div class="row g-2 fs-8 text-light">
                            <div class="col-md-6"><div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_create_users" id="add_can_create_users" value="1" checked><label class="form-check-label" for="add_can_create_users">إضافة مشترك جديد</label></div></div>
                            <div class="col-md-6"><div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_edit_users" id="add_can_edit_users" value="1" checked><label class="form-check-label" for="add_can_edit_users">تعديل بيانات المشترك</label></div></div>
                            <div class="col-md-6"><div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_renew_users" id="add_can_renew_users" value="1" checked><label class="form-check-label" for="add_can_renew_users">تجديد وتمديد اشتراك</label></div></div>
                            <div class="col-md-6"><div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_toggle_users" id="add_can_toggle_users" value="1" checked><label class="form-check-label" for="add_can_toggle_users">تعطيل وتفعيل الحسابات</label></div></div>
                            <div class="col-md-6"><div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_disconnect_users" id="add_can_disconnect_users" value="1" checked><label class="form-check-label" for="add_can_disconnect_users">فصل الجلسة من المايكروتك</label></div></div>
                            <div class="col-md-6"><div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_delete_users" id="add_can_delete_users" value="1"><label class="form-check-label text-danger" for="add_can_delete_users">حذف المشترك نهائياً</label></div></div>
                            <div class="col-md-6"><div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_generate_vouchers" id="add_can_generate_vouchers" value="1" checked><label class="form-check-label" for="add_can_generate_vouchers">توليد كروت الشحن</label></div></div>
                            <div class="col-md-6"><div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_print_vouchers" id="add_can_print_vouchers" value="1" checked><label class="form-check-label" for="add_can_print_vouchers">طباعة وتصدير الكروت</label></div></div>
                            <div class="col-md-6"><div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_manage_transactions" id="add_can_manage_transactions" value="1" checked><label class="form-check-label" for="add_can_manage_transactions">تسجيل الديون والمدفوعات</label></div></div>
                            <div class="col-md-6"><div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_view_logs" id="add_can_view_logs" value="1" checked><label class="form-check-label" for="add_can_view_logs">الاطلاع على سجل العمليات</label></div></div>
                            <div class="col-md-6"><div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_manage_profiles" id="add_can_manage_profiles" value="1"><label class="form-check-label" for="add_can_manage_profiles">إنشاء وتعديل باقات السرعة</label></div></div>
                            <div class="col-md-6"><div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_manage_nas" id="add_can_manage_nas" value="1"><label class="form-check-label" for="add_can_manage_nas">إدارة راوترات المايكروتك</label></div></div>
                        </div>
                    </div>
                </div>
                <div class="modal-footer border-secondary">
                    <button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal">إلغاء</button>
                    <button type="submit" class="btn btn-primary px-4 fw-bold">حفظ وإنشاء الوكيل</button>
                </div>
            </form>
        </div>
    </div>
</div>

<!-- Edit Permissions Modal (Permission Matrix Studio) -->
<div class="modal fade" id="permissionsModal" tabindex="-1" aria-labelledby="permissionsModalLabel" aria-hidden="true">
    <div class="modal-dialog modal-dialog-centered modal-lg">
        <div class="modal-content glass-card border-primary text-light">
            <div class="modal-header border-secondary">
                <h5 class="modal-title fw-bold" id="permissionsModalLabel">
                    <i class="fa-solid fa-sliders me-2 text-primary"></i> مصفوفة الصلاحيات الحبيبية: <span id="permModalAdminUser" class="text-warning font-monospace">@agent</span>
                </h5>
                <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" aria-label="Close"></button>
            </div>
            <form id="editPermissionsForm" action="/admins/permissions" method="POST">
                <input type="hidden" name="admin_id" id="permModalAdminId">
                <div class="modal-body">
                    <!-- Quick Presets -->
                    <div class="p-3 mb-3 bg-dark bg-opacity-75 rounded-4 border border-secondary d-flex align-items-center justify-content-between flex-wrap gap-2">
                        <div>
                            <span class="fw-bold fs-7 text-light d-block"><i class="fa-solid fa-wand-magic-sparkles me-1 text-warning"></i> قوالب صلاحيات سريعة:</span>
                            <small class="text-muted fs-8">تطبيق مجموعة الصلاحيات بنقرة زر واحدة</small>
                        </div>
                        <div class="d-flex gap-2">
                            <button type="button" class="btn btn-sm btn-outline-info rounded-pill" onclick="applyPreset('edit', 'reseller')">
                                <i class="fa-solid fa-ticket me-1"></i> موزع كروت ومبيعات
                            </button>
                            <button type="button" class="btn btn-sm btn-outline-success rounded-pill" onclick="applyPreset('edit', 'manager')">
                                <i class="fa-solid fa-crown me-1"></i> مدير فرع متكامل
                            </button>
                            <button type="button" class="btn btn-sm btn-outline-secondary rounded-pill" onclick="applyPreset('edit', 'readonly')">
                                <i class="fa-solid fa-eye me-1"></i> مشاهدة ومتابعة فقط
                            </button>
                        </div>
                    </div>

                    <!-- Category Matrix Cards -->
                    <div class="row g-3">
                        <!-- Users Permissions -->
                        <div class="col-md-6">
                            <div class="p-3 bg-dark rounded-4 border border-primary border-opacity-25 h-100">
                                <h6 class="fw-bold text-primary mb-3"><i class="fa-solid fa-users me-2"></i> إدارة المشتركين والحسابات</h6>
                                <div class="d-flex flex-column gap-2 fs-8">
                                    <div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_create_users" id="edit_can_create_users" value="1"><label class="form-check-label" for="edit_can_create_users">إضافة مشترك جديد</label></div>
                                    <div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_edit_users" id="edit_can_edit_users" value="1"><label class="form-check-label" for="edit_can_edit_users">تعديل بيانات وكلمة المرور</label></div>
                                    <div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_renew_users" id="edit_can_renew_users" value="1"><label class="form-check-label" for="edit_can_renew_users">تجديد وتمديد الاشتراكات</label></div>
                                    <div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_toggle_users" id="edit_can_toggle_users" value="1"><label class="form-check-label" for="edit_can_toggle_users">تعطيل / تفعيل الحساب</label></div>
                                    <div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_disconnect_users" id="edit_can_disconnect_users" value="1"><label class="form-check-label" for="edit_can_disconnect_users">فصل الجلسة من المايكروتك</label></div>
                                    <div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_delete_users" id="edit_can_delete_users" value="1"><label class="form-check-label text-danger fw-bold" for="edit_can_delete_users">حذف المشتركين نهائياً ⚠️</label></div>
                                </div>
                            </div>
                        </div>

                        <!-- Vouchers Permissions -->
                        <div class="col-md-6">
                            <div class="p-3 bg-dark rounded-4 border border-warning border-opacity-25 h-100">
                                <h6 class="fw-bold text-warning mb-3"><i class="fa-solid fa-ticket me-2"></i> كروت الشحن والطباعة</h6>
                                <div class="d-flex flex-column gap-2 fs-8">
                                    <div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_generate_vouchers" id="edit_can_generate_vouchers" value="1"><label class="form-check-label" for="edit_can_generate_vouchers">توليد حزم كروت جديدة</label></div>
                                    <div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_print_vouchers" id="edit_can_print_vouchers" value="1"><label class="form-check-label" for="edit_can_print_vouchers">طباعة وتصدير الكروت (A4 / حراري)</label></div>
                                    <div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_delete_vouchers" id="edit_can_delete_vouchers" value="1"><label class="form-check-label text-danger fw-bold" for="edit_can_delete_vouchers">حذف وتفريغ الكروت ⚠️</label></div>
                                </div>
                            </div>
                        </div>

                        <!-- Profiles & NAS Permissions -->
                        <div class="col-md-6">
                            <div class="p-3 bg-dark rounded-4 border border-info border-opacity-25 h-100">
                                <h6 class="fw-bold text-info mb-3"><i class="fa-solid fa-network-wired me-2"></i> الباقات وأجهزة الشبكة</h6>
                                <div class="d-flex flex-column gap-2 fs-8">
                                    <div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_manage_profiles" id="edit_can_manage_profiles" value="1"><label class="form-check-label" for="edit_can_manage_profiles">إنشاء وتعديل باقات السرعة</label></div>
                                    <div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_manage_nas" id="edit_can_manage_nas" value="1"><label class="form-check-label" for="edit_can_manage_nas">إدارة راوترات المايكروتك و Winbox</label></div>
                                    <div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_manage_devices" id="edit_can_manage_devices" value="1"><label class="form-check-label" for="edit_can_manage_devices">إدارة الصحونات وأجهزة CPE</label></div>
                                </div>
                            </div>
                        </div>

                        <!-- Finance, Logs & Services -->
                        <div class="col-md-6">
                            <div class="p-3 bg-dark rounded-4 border border-success border-opacity-25 h-100">
                                <h6 class="fw-bold text-success mb-3"><i class="fa-solid fa-shield-halved me-2"></i> المالية، الرقابة والخدمات</h6>
                                <div class="d-flex flex-column gap-2 fs-8">
                                    <div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_manage_transactions" id="edit_can_manage_transactions" value="1"><label class="form-check-label" for="edit_can_manage_transactions">تسجيل الديون وحركات التسديد</label></div>
                                    <div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_manage_subagents" id="edit_can_manage_subagents" value="1"><label class="form-check-label" for="edit_can_manage_subagents">إنشاء وشحن وكلاء فرعيين تحته</label></div>
                                    <div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_view_logs" id="edit_can_view_logs" value="1"><label class="form-check-label" for="edit_can_view_logs">الاطلاع على سجل العمليات والرقابة</label></div>
                                    <div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_manage_whatsapp" id="edit_can_manage_whatsapp" value="1"><label class="form-check-label" for="edit_can_manage_whatsapp">إرسال إشعارات الواتساب</label></div>
                                    <div class="form-check form-switch"><input class="form-check-input" type="checkbox" name="can_manage_streams" id="edit_can_manage_streams" value="1"><label class="form-check-label" for="edit_can_manage_streams">إدارة قنوات البث المباشر IPTV</label></div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
                <div class="modal-footer border-secondary">
                    <button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal">إلغاء</button>
                    <button type="submit" class="btn btn-primary px-4 fw-bold"><i class="fa-solid fa-floppy-disk me-1"></i> حفظ مصفوفة الصلاحيات</button>
                </div>
            </form>
        </div>
    </div>
</div>

<!-- Recharge Modal -->
<div class="modal fade" id="rechargeModal" tabindex="-1" aria-labelledby="rechargeModalLabel" aria-hidden="true">
    <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content glass-card border-warning text-light">
            <div class="modal-header border-secondary">
                <h5 class="modal-title fw-bold text-warning" id="rechargeModalLabel"><i class="fa-solid fa-wallet me-2"></i> شحن رصيد وكيل</h5>
                <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" aria-label="Close"></button>
            </div>
            <form action="/admins/recharge" method="POST">
                <div class="modal-body">
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">اختر الوكيل / الموزع *</label>
                        <select name="admin_id" id="rechargeAdminSelect" class="form-select bg-dark border-secondary text-light" required>
                            <option value="">-- اختر الوكيل --</option>
                            <?php foreach ($admins as $a): ?>
                            <?php if (($a['role'] ?? '') !== 'superadmin'): ?>
                            <option value="<?= htmlspecialchars($a['id']) ?>"><?= htmlspecialchars($a['name'] ?: $a['username']) ?> (@<?= htmlspecialchars($a['username']) ?>) - الرصيد الحالي: <?= number_format($a['balance'] ?? 0, 0) ?> د.ع</option>
                            <?php endif; ?>
                            <?php endforeach; ?>
                        </select>
                    </div>
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">المبلغ المراد شحنه (د.ع) *</label>
                        <input type="number" name="amount" class="form-control bg-dark border-secondary text-light fs-5 fw-bold" required value="100000" step="5000">
                    </div>
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">ملاحظات العملية</label>
                        <input type="text" name="notes" class="form-control bg-dark border-secondary text-light" placeholder="تسديد نقدي / دفعة رصيد">
                    </div>
                </div>
                <div class="modal-footer border-secondary">
                    <button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal">إلغاء</button>
                    <button type="submit" class="btn btn-warning px-4 fw-bold">تأكيد الشحن</button>
                </div>
            </form>
        </div>
    </div>
</div>

<!-- Withdraw Modal -->
<div class="modal fade" id="withdrawModal" tabindex="-1" aria-labelledby="withdrawModalLabel" aria-hidden="true">
    <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content glass-card border-info text-light">
            <div class="modal-header border-secondary">
                <h5 class="modal-title fw-bold text-info" id="withdrawModalLabel"><i class="fa-solid fa-hand-holding-dollar me-2"></i> سحب رصيد من وكيل</h5>
                <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" aria-label="Close"></button>
            </div>
            <form action="/admins/withdraw" method="POST">
                <div class="modal-body">
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">اختر الوكيل / الموزع *</label>
                        <select name="admin_id" id="withdrawAdminSelect" class="form-select bg-dark border-secondary text-light" required>
                            <option value="">-- اختر الوكيل --</option>
                            <?php foreach ($admins as $a): ?>
                            <?php if (($a['role'] ?? '') !== 'superadmin'): ?>
                            <option value="<?= htmlspecialchars($a['id']) ?>"><?= htmlspecialchars($a['name'] ?: $a['username']) ?> (@<?= htmlspecialchars($a['username']) ?>) - الرصيد: <?= number_format($a['balance'] ?? 0, 0) ?> د.ع</option>
                            <?php endif; ?>
                            <?php endforeach; ?>
                        </select>
                    </div>
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">المبلغ المراد سحبه (د.ع) *</label>
                        <input type="number" name="amount" class="form-control bg-dark border-secondary text-light fs-5 fw-bold" required value="25000" step="5000">
                    </div>
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">ملاحظات أو سبب السحب</label>
                        <input type="text" name="notes" class="form-control bg-dark border-secondary text-light" placeholder="استرجاع دفعة / تعديل رصيد">
                    </div>
                </div>
                <div class="modal-footer border-secondary">
                    <button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal">إلغاء</button>
                    <button type="submit" class="btn btn-info px-4 fw-bold">تأكيد سحب الرصيد</button>
                </div>
            </form>
        </div>
    </div>
</div>

<script>
function openAgentRecharge(adminId, username, balance) {
    document.getElementById('rechargeAdminSelect').value = adminId;
    new bootstrap.Modal(document.getElementById('rechargeModal')).show();
}

function openAgentWithdraw(adminId, username, balance) {
    document.getElementById('withdrawAdminSelect').value = adminId;
    new bootstrap.Modal(document.getElementById('withdrawModal')).show();
}

function openPermissionsModal(admin) {
    document.getElementById('permModalAdminId').value = admin.id;
    document.getElementById('permModalAdminUser').innerText = '@' + (admin.username || '');
    document.getElementById('editPermissionsForm').action = '/admins/' + admin.id + '/permissions';

    const fields = [
        'can_create_users', 'can_edit_users', 'can_delete_users', 'can_toggle_users', 'can_disconnect_users', 'can_renew_users',
        'can_generate_vouchers', 'can_delete_vouchers', 'can_print_vouchers',
        'can_manage_profiles', 'can_manage_nas', 'can_manage_devices',
        'can_manage_transactions', 'can_manage_subagents', 'can_view_logs', 'can_manage_whatsapp', 'can_manage_streams'
    ];

    fields.forEach(f => {
        const el = document.getElementById('edit_' + f);
        if (el) {
            el.checked = !!admin[f];
        }
    });

    new bootstrap.Modal(document.getElementById('permissionsModal')).show();
}

function applyPreset(prefix, type) {
    const setChecked = (field, val) => {
        const el = document.getElementById(prefix + '_' + field);
        if (el) el.checked = val;
    };

    if (type === 'reseller') {
        // Sales & Recharge Only
        setChecked('can_create_users', true);
        setChecked('can_edit_users', true);
        setChecked('can_renew_users', true);
        setChecked('can_toggle_users', true);
        setChecked('can_disconnect_users', true);
        setChecked('can_delete_users', false);
        setChecked('can_generate_vouchers', true);
        setChecked('can_print_vouchers', true);
        setChecked('can_delete_vouchers', false);
        setChecked('can_manage_profiles', false);
        setChecked('can_manage_nas', false);
        setChecked('can_manage_devices', false);
        setChecked('can_manage_transactions', true);
        setChecked('can_manage_subagents', false);
        setChecked('can_view_logs', true);
        setChecked('can_manage_whatsapp', false);
        setChecked('can_manage_streams', false);
    } else if (type === 'manager') {
        // Branch Manager Full Privileges
        setChecked('can_create_users', true);
        setChecked('can_edit_users', true);
        setChecked('can_renew_users', true);
        setChecked('can_toggle_users', true);
        setChecked('can_disconnect_users', true);
        setChecked('can_delete_users', true);
        setChecked('can_generate_vouchers', true);
        setChecked('can_print_vouchers', true);
        setChecked('can_delete_vouchers', true);
        setChecked('can_manage_profiles', true);
        setChecked('can_manage_nas', true);
        setChecked('can_manage_devices', true);
        setChecked('can_manage_transactions', true);
        setChecked('can_manage_subagents', true);
        setChecked('can_view_logs', true);
        setChecked('can_manage_whatsapp', true);
        setChecked('can_manage_streams', true);
    } else if (type === 'readonly') {
        // Read Only
        setChecked('can_create_users', false);
        setChecked('can_edit_users', false);
        setChecked('can_renew_users', false);
        setChecked('can_toggle_users', false);
        setChecked('can_disconnect_users', false);
        setChecked('can_delete_users', false);
        setChecked('can_generate_vouchers', false);
        setChecked('can_print_vouchers', true);
        setChecked('can_delete_vouchers', false);
        setChecked('can_manage_profiles', false);
        setChecked('can_manage_nas', false);
        setChecked('can_manage_devices', false);
        setChecked('can_manage_transactions', false);
        setChecked('can_manage_subagents', false);
        setChecked('can_view_logs', true);
        setChecked('can_manage_whatsapp', false);
        setChecked('can_manage_streams', false);
    }
}
</script>
