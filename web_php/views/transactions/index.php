<?php
$totalOutstandingDebt = 0;
$debtorCount = 0;
$totalPayments = 0;

foreach ($user_summaries as $us) {
    $bal = (float)($us['balance'] ?? 0);
    $totalPayments += (float)($us['total_paid'] ?? 0);
    if ($bal > 0) {
        $totalOutstandingDebt += $bal;
        $debtorCount++;
    }
}
?>

<div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
    <div>
        <h4 class="fw-bold text-light mb-1"><i class="fa-solid fa-money-bill-transfer me-2 text-success"></i> السجل المالي وحسابات ديون المشتركين</h4>
        <p class="text-muted mb-0 fs-7">متابعة حسابات المشتركين الفردية، كشوفات الحسابات، وتسجيل الدفعات والديون</p>
    </div>
    <button class="btn btn-success rounded-pill px-4 fw-bold shadow-sm" data-bs-toggle="modal" data-bs-target="#addTransModal">
        <i class="fa-solid fa-plus me-2"></i> تسجيل دفعة / دين مالي
    </button>
</div>

<!-- Financial Summary Cards -->
<div class="row g-3 mb-4">
    <div class="col-12 col-md-4">
        <div class="card glass-card border-danger border-opacity-50 shadow-sm rounded-4 p-3 h-100">
            <div class="d-flex align-items-center justify-content-between">
                <div>
                    <span class="text-muted fs-7 fw-semibold d-block mb-1">إجمالي الديون المطلوبة (Net Debt)</span>
                    <h3 class="fw-bold text-danger mb-0"><?= number_format($totalOutstandingDebt, 0) ?> د.ع</h3>
                </div>
                <div class="stat-icon bg-danger-subtle text-danger rounded-3 p-3">
                    <i class="fa-solid fa-hand-holding-dollar fs-3"></i>
                </div>
            </div>
        </div>
    </div>

    <div class="col-12 col-md-4">
        <div class="card glass-card border-success border-opacity-50 shadow-sm rounded-4 p-3 h-100">
            <div class="d-flex align-items-center justify-content-between">
                <div>
                    <span class="text-muted fs-7 fw-semibold d-block mb-1">إجمالي المقبوضات (Total Paid)</span>
                    <h3 class="fw-bold text-success mb-0"><?= number_format($totalPayments, 0) ?> د.ع</h3>
                </div>
                <div class="stat-icon bg-success-subtle text-success rounded-3 p-3">
                    <i class="fa-solid fa-arrow-down-left-and-arrow-up-right-to-center fs-3"></i>
                </div>
            </div>
        </div>
    </div>

    <div class="col-12 col-md-4">
        <div class="card glass-card border-0 shadow-sm rounded-4 p-3 h-100">
            <div class="d-flex align-items-center justify-content-between">
                <div>
                    <span class="text-muted fs-7 fw-semibold d-block mb-1">المشتركون المدينون</span>
                    <h3 class="fw-bold text-warning mb-0"><?= $debtorCount ?> مشترك</h3>
                </div>
                <div class="stat-icon bg-warning-subtle text-warning rounded-3 p-3">
                    <i class="fa-solid fa-users-viewfinder fs-3"></i>
                </div>
            </div>
        </div>
    </div>
</div>

<!-- Tabs Navigation -->
<ul class="nav nav-pills mb-3 gap-2" id="transTabs" role="tablist">
    <li class="nav-item" role="presentation">
        <button class="nav-link active fw-bold rounded-pill px-4" id="subscribers-ledger-tab" data-bs-toggle="pill" data-bs-target="#subscribers-ledger-pane" type="button" role="tab">
            <i class="fa-solid fa-users me-2"></i> دليل حسابات وديون المشتركين (حسب المشترك)
        </button>
    </li>
    <li class="nav-item" role="presentation">
        <button class="nav-link fw-bold rounded-pill px-4" id="all-transactions-tab" data-bs-toggle="pill" data-bs-target="#all-transactions-pane" type="button" role="tab">
            <i class="fa-solid fa-list-check me-2"></i> سجل كافة العمليات العامة
        </button>
    </li>
</ul>

<div class="tab-content" id="transTabsContent">
    <!-- 1. SUBSCRIBERS FINANCIAL DIRECTORY (حسب المشترك مع كشف الحساب والديون) -->
    <div class="tab-pane fade show active" id="subscribers-ledger-pane" role="tabpanel">
        <div class="card glass-card border-0 shadow-sm rounded-4 p-4">
            <div class="d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-3 pb-3 border-bottom border-secondary">
                <div class="d-flex align-items-center gap-2">
                    <span class="text-muted fs-7">تصفية العرض:</span>
                    <button type="button" class="btn btn-sm btn-outline-light rounded-pill px-3 active" onclick="filterLedger('all', this)">الكل (<?= count($user_summaries) ?>)</button>
                    <button type="button" class="btn btn-sm btn-outline-danger rounded-pill px-3" onclick="filterLedger('debt', this)">المدينون فقط (<?= $debtorCount ?>)</button>
                    <button type="button" class="btn btn-sm btn-outline-success rounded-pill px-3" onclick="filterLedger('settled', this)">الخالصون</button>
                </div>
                <input type="text" id="ledgerSearchInput" class="form-control form-control-sm bg-dark border-secondary text-light rounded-pill px-3" placeholder="بحث باسم المشترك أو الهاتف..." onkeyup="searchLedgerTable()" style="max-width: 250px;">
            </div>

            <div class="table-responsive">
                <table class="table table-dark table-hover align-middle mb-0" id="ledgerTable">
                    <thead class="table-secondary">
                        <tr>
                            <th>#</th>
                            <th>المشترك (User)</th>
                            <th>الاسم ورقم الهاتف</th>
                            <th>إجمالي المسدد</th>
                            <th>إجمالي الديون</th>
                            <th>صافي الدين المتبقي (المطلوب)</th>
                            <th>الحالة</th>
                            <th class="text-center">الإجراءات المحاسبية</th>
                        </tr>
                    </thead>
                    <tbody>
                        <?php if (empty($user_summaries)): ?>
                        <tr>
                            <td colspan="8" class="text-center py-5 text-muted">لا يوجد مشتركون مسجلون حالياً</td>
                        </tr>
                        <?php else: ?>
                        <?php foreach ($user_summaries as $i => $us): ?>
                        <?php
                            $uName = $us['username'] ?? '';
                            $fullName = $us['full_name'] ?: '—';
                            $phone = $us['phone'] ?: '—';
                            $balance = (float)($us['balance'] ?? 0);
                            $totalPaid = (float)($us['total_paid'] ?? 0);
                            $totalDebt = (float)($us['total_debt'] ?? 0);
                            $hasDebt = $balance > 0;
                            $userTransJSON = htmlspecialchars(json_encode($us['transactions'] ?? []), ENT_QUOTES, 'UTF-8');
                        ?>
                        <tr class="ledger-row" data-has-debt="<?= $hasDebt ? '1' : '0' ?>">
                            <td class="text-muted fs-8"><?= $i + 1 ?></td>
                            <td class="fw-bold text-info font-monospace fs-6">
                                <i class="fa-solid fa-user me-1 text-primary"></i> <?= htmlspecialchars($uName) ?>
                            </td>
                            <td>
                                <div><?= htmlspecialchars($fullName) ?></div>
                                <small class="text-muted font-monospace fs-8"><?= htmlspecialchars($phone) ?></small>
                            </td>
                            <td class="text-success fw-bold"><?= number_format($totalPaid, 0) ?> د.ع</td>
                            <td class="text-muted"><?= number_format($totalDebt, 0) ?> د.ع</td>
                            <td>
                                <?php if ($balance > 0): ?>
                                <span class="badge bg-danger fs-6 fw-bold px-3 py-2">
                                    <i class="fa-solid fa-circle-exclamation me-1"></i> مطلوب <?= number_format($balance, 0) ?> د.ع
                                </span>
                                <?php elseif ($balance < 0): ?>
                                <span class="badge bg-info-subtle text-info border border-info fs-7 px-2 py-1">
                                    دائن (له) <?= number_format(abs($balance), 0) ?> د.ع
                                </span>
                                <?php else: ?>
                                <span class="badge bg-success-subtle text-success border border-success fs-7 px-2 py-1">
                                    <i class="fa-solid fa-check me-1"></i> خالص (0 د.ع)
                                </span>
                                <?php endif; ?>
                            </td>
                            <td>
                                <?php if ($balance > 0): ?>
                                <span class="badge bg-danger-subtle text-danger border border-danger">عليه ديون</span>
                                <?php else: ?>
                                <span class="badge bg-success-subtle text-success border border-success">مسدد بالكامل</span>
                                <?php endif; ?>
                            </td>
                            <td class="text-center">
                                <div class="btn-group btn-group-sm">
                                    <!-- Quick Payment Button -->
                                    <button type="button" class="btn btn-outline-success" onclick="openQuickPayModal('<?= htmlspecialchars($uName, ENT_QUOTES) ?>', <?= $balance ?>)" title="تسديد وقبض دفعة مالية">
                                        <i class="fa-solid fa-hand-holding-dollar me-1"></i> قبض دفعة
                                    </button>

                                    <!-- Account Statement Modal Button -->
                                    <button type="button" class="btn btn-outline-info" onclick='openStatementModal("<?= htmlspecialchars($uName, ENT_QUOTES) ?>", "<?= htmlspecialchars($fullName, ENT_QUOTES) ?>", <?= $balance ?>, <?= $totalPaid ?>, <?= $totalDebt ?>, <?= $userTransJSON ?>)' title="عرض كشف الحساب التفصيلي">
                                        <i class="fa-solid fa-file-invoice-dollar me-1"></i> كشف الحساب
                                    </button>
                                </div>
                            </td>
                        </tr>
                        <?php endforeach; ?>
                        <?php endif; ?>
                    </tbody>
                </table>
            </div>
        </div>
    </div>

    <!-- 2. ALL GLOBAL TRANSACTIONS (السجل الزمني العام لكافة العمليات) -->
    <div class="tab-pane fade" id="all-transactions-pane" role="tabpanel">
        <div class="card glass-card border-0 shadow-sm rounded-4 p-4">
            <h5 class="fw-bold text-light mb-3"><i class="fa-solid fa-clock-rotate-left me-2 text-primary"></i> السجل الزمني لكافة العمليات</h5>
            <div class="table-responsive">
                <table class="table table-dark table-hover align-middle mb-0">
                    <thead class="table-secondary">
                        <tr>
                            <th>#</th>
                            <th>اسم المشترك</th>
                            <th>نوع الحركة</th>
                            <th>المبلغ</th>
                            <th>الملاحظات</th>
                            <th>التاريخ والوقت</th>
                            <th>المنفذ</th>
                        </tr>
                    </thead>
                    <tbody>
                        <?php if (empty($transactions)): ?>
                        <tr>
                            <td colspan="7" class="text-center py-5 text-muted">لا توجد حركات مالية مسجلة حتى الآن</td>
                        </tr>
                        <?php else: ?>
                        <?php foreach ($transactions as $i => $t): ?>
                        <?php
                            $tUser = $t['username'] ?? '—';
                            $tType = $t['type'] ?? 'payment';
                            $tAmt = (float)($t['amount'] ?? 0);
                            $tNotes = $t['notes'] ?? '—';
                            $tDate = $t['created_at'] ?? '—';
                            $tAdmin = $t['admin'] ?? 'admin';
                        ?>
                        <tr>
                            <td class="text-muted fs-8"><?= $i + 1 ?></td>
                            <td class="fw-bold text-info font-monospace fs-6"><?= htmlspecialchars($tUser) ?></td>
                            <td>
                                <?php if ($tType === 'payment' || $tType === 'تجديد اشتراك'): ?>
                                <span class="badge bg-success-subtle text-success border border-success px-2 py-1"><i class="fa-solid fa-circle-arrow-down me-1"></i> تسديد / دفعة</span>
                                <?php else: ?>
                                <span class="badge bg-danger-subtle text-danger border border-danger px-2 py-1"><i class="fa-solid fa-circle-arrow-up me-1"></i> دين / آجل</span>
                                <?php endif; ?>
                            </td>
                            <td class="fw-bold <?= ($tType === 'payment' || $tType === 'تجديد اشتراك') ? 'text-success' : 'text-danger' ?> fs-6">
                                <?= number_format($tAmt, 0) ?> د.ع
                            </td>
                            <td class="fs-8 text-light"><?= htmlspecialchars($tNotes ?: '—') ?></td>
                            <td class="fs-7 text-muted"><?= htmlspecialchars($tDate) ?></td>
                            <td class="fs-8 text-light"><span class="badge bg-secondary font-monospace">@<?= htmlspecialchars($tAdmin) ?></span></td>
                        </tr>
                        <?php endforeach; ?>
                        <?php endif; ?>
                    </tbody>
                </table>
            </div>
        </div>
    </div>
</div>

<!-- MODAL 1: ACCOUNT STATEMENT (كشف الحساب التفصيلي للمشترك) -->
<div class="modal fade" id="statementModal" tabindex="-1" aria-hidden="true">
    <div class="modal-dialog modal-dialog-centered modal-lg">
        <div class="modal-content glass-card border-info text-light">
            <div class="modal-header border-secondary">
                <h5 class="modal-title fw-bold text-info"><i class="fa-solid fa-file-invoice-dollar me-2"></i> كشف حساب المشترك التفصيلي</h5>
                <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" aria-label="Close"></button>
            </div>
            <div class="modal-body p-4">
                <!-- User Header Box -->
                <div class="d-flex flex-column flex-md-row justify-content-between align-items-md-center bg-dark p-3 rounded-4 border border-secondary mb-4 gap-2">
                    <div>
                        <h4 class="fw-bold text-info mb-1 font-monospace" id="stmtUser">—</h4>
                        <span class="text-muted fs-7" id="stmtFullName">—</span>
                    </div>
                    <div class="text-md-end">
                        <small class="text-muted fs-8 d-block mb-1">صافي الدين المتبقي بذمته:</small>
                        <h4 class="fw-bold mb-0" id="stmtBalance">—</h4>
                    </div>
                </div>

                <!-- Aggregate Cards -->
                <div class="row g-2 mb-4">
                    <div class="col-6">
                        <div class="bg-dark p-3 rounded-3 border border-success-subtle text-center">
                            <small class="text-muted fs-8 d-block mb-1">إجمالي ما سدده المشترك</small>
                            <h5 class="fw-bold text-success mb-0" id="stmtTotalPaid">0 د.ع</h5>
                        </div>
                    </div>
                    <div class="col-6">
                        <div class="bg-dark p-3 rounded-3 border border-danger-subtle text-center">
                            <small class="text-muted fs-8 d-block mb-1">إجمالي الديون المسجلة</small>
                            <h5 class="fw-bold text-danger mb-0" id="stmtTotalDebt">0 د.ع</h5>
                        </div>
                    </div>
                </div>

                <h6 class="fw-bold text-light mb-2"><i class="fa-solid fa-list me-1 text-primary"></i> سجل الحركات والوصولات لهذا المشترك:</h6>
                <div class="table-responsive bg-dark rounded-3 border border-secondary p-2" style="max-height: 250px; overflow-y: auto;">
                    <table class="table table-dark table-sm align-middle mb-0 fs-8">
                        <thead>
                            <tr class="text-muted">
                                <th>نوع الحركة</th>
                                <th>المبلغ</th>
                                <th>الملاحظات</th>
                                <th>التاريخ والوقت</th>
                            </tr>
                        </thead>
                        <tbody id="stmtTransBody">
                            <!-- Populated via JS -->
                        </tbody>
                    </table>
                </div>
            </div>
            <div class="modal-footer border-secondary">
                <button type="button" class="btn btn-secondary btn-sm" data-bs-dismiss="modal">إغلاق</button>
                <button type="button" class="btn btn-success btn-sm px-4 fw-bold" onclick="triggerPayFromStatement()">
                    <i class="fa-solid fa-hand-holding-dollar me-1"></i> قبض وتسديد دفعة الآن
                </button>
            </div>
        </div>
    </div>
</div>

<!-- MODAL 2: QUICK PAY / RECORD TRANSACTION (تسديد وقبض دفعة مالية) -->
<div class="modal fade" id="addTransModal" tabindex="-1" aria-labelledby="addTransModalLabel" aria-hidden="true">
    <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content glass-card border-success text-light">
            <div class="modal-header border-secondary">
                <h5 class="modal-title fw-bold text-success" id="addTransModalLabel"><i class="fa-solid fa-money-bill-transfer me-2"></i> تسجيل حركة مالية (قبض / دين)</h5>
                <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" aria-label="Close"></button>
            </div>
            <form action="/transactions" method="POST">
                <div class="modal-body">
                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">اختر المشترك *</label>
                        <select name="username" id="transModalUserSelect" class="form-select bg-dark border-secondary text-light" required>
                            <option value="">-- اختر المشترك --</option>
                            <?php foreach ($users as $u): ?>
                            <?php $uname = $u['user'] ?? $u['username'] ?? ''; ?>
                            <option value="<?= htmlspecialchars($uname) ?>"><?= htmlspecialchars($uname) ?> (<?= htmlspecialchars($u['full_name'] ?? '') ?>)</option>
                            <?php endforeach; ?>
                        </select>
                    </div>

                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">نوع الحركة المالية *</label>
                        <div class="bg-dark p-3 rounded-3 border border-secondary d-flex flex-column gap-2">
                            <div class="form-check">
                                <input class="form-check-input" type="radio" name="type" id="typePay" value="payment" checked>
                                <label class="form-check-label text-success fw-bold fs-7" for="typePay">
                                    <i class="fa-solid fa-circle-arrow-down me-1"></i> تسديد نقدي / قبض دفعة (يخصم من دين المشترك)
                                </label>
                            </div>
                            <div class="form-check">
                                <input class="form-check-input" type="radio" name="type" id="typeDebt" value="debt">
                                <label class="form-check-label text-danger fw-bold fs-7" for="typeDebt">
                                    <i class="fa-solid fa-circle-arrow-up me-1"></i> تسجيل دين / مؤجل (يزيد من دين المشترك)
                                </label>
                            </div>
                        </div>
                    </div>

                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">المبلغ (د.ع) *</label>
                        <input type="number" name="amount" id="transModalAmount" class="form-control bg-dark border-secondary text-light fs-5 fw-bold" required value="25000" step="1000">
                    </div>

                    <div class="mb-3">
                        <label class="form-label fs-7 fw-semibold">ملاحظات أو رقم الوصل</label>
                        <input type="text" name="notes" class="form-control bg-dark border-secondary text-light" placeholder="تسديد اشتراك / تسديد دفعة من الدين">
                    </div>
                </div>
                <div class="modal-footer border-secondary">
                    <button type="button" class="btn btn-outline-secondary" data-bs-dismiss="modal">إلغاء</button>
                    <button type="submit" class="btn btn-success px-4 fw-bold">حفظ الحركة المالية</button>
                </div>
            </form>
        </div>
    </div>
</div>

<script>
let currentStmtUser = '';
let currentStmtBalance = 0;

// Open Account Statement Modal
function openStatementModal(username, fullName, balance, totalPaid, totalDebt, transactions) {
    currentStmtUser = username;
    currentStmtBalance = balance;

    document.getElementById('stmtUser').innerText = username;
    document.getElementById('stmtFullName').innerText = fullName;
    
    const balEl = document.getElementById('stmtBalance');
    if (balance > 0) {
        balEl.className = 'fw-bold mb-0 text-danger';
        balEl.innerText = 'مطلوب دين ' + balance.toLocaleString() + ' د.ع';
    } else if (balance < 0) {
        balEl.className = 'fw-bold mb-0 text-info';
        balEl.innerText = 'دائن (له) ' + Math.abs(balance).toLocaleString() + ' د.ع';
    } else {
        balEl.className = 'fw-bold mb-0 text-success';
        balEl.innerText = 'خالص (0 د.ع)';
    }

    document.getElementById('stmtTotalPaid').innerText = totalPaid.toLocaleString() + ' د.ع';
    document.getElementById('stmtTotalDebt').innerText = totalDebt.toLocaleString() + ' د.ع';

    const tbody = document.getElementById('stmtTransBody');
    tbody.innerHTML = '';

    if (!transactions || transactions.length === 0) {
        tbody.innerHTML = '<tr><td colspan="4" class="text-center text-muted py-3">لا توجد حركات مسجلة لهذا المشترك</td></tr>';
    } else {
        transactions.forEach(t => {
            const isPay = (t.type === 'payment' || t.type === 'تجديد اشتراك');
            const typeBadge = isPay 
                ? '<span class="badge bg-success-subtle text-success border border-success">تسديد / دفعة</span>' 
                : '<span class="badge bg-danger-subtle text-danger border border-danger">دين / آجل</span>';
            const amtClass = isPay ? 'text-success fw-bold' : 'text-danger fw-bold';
            
            const tr = document.createElement('tr');
            tr.innerHTML = `
                <td>${typeBadge}</td>
                <td class="${amtClass}">${Number(t.amount || 0).toLocaleString()} د.ع</td>
                <td class="text-light">${t.notes || '—'}</td>
                <td class="text-muted">${t.created_at || '—'}</td>
            `;
            tbody.appendChild(tr);
        });
    }

    new bootstrap.Modal(document.getElementById('statementModal')).show();
}

function triggerPayFromStatement() {
    bootstrap.Modal.getInstance(document.getElementById('statementModal')).hide();
    openQuickPayModal(currentStmtUser, currentStmtBalance);
}

// Open Quick Pay Modal
function openQuickPayModal(username, balance) {
    document.getElementById('transModalUserSelect').value = username;
    if (balance > 0) {
        document.getElementById('transModalAmount').value = balance;
    }
    document.getElementById('typePay').checked = true;
    new bootstrap.Modal(document.getElementById('addTransModal')).show();
}

// Filter Ledger Table (All vs Debtors vs Settled)
function filterLedger(mode, btn) {
    document.querySelectorAll('#subscribers-ledger-pane .btn-group button, #subscribers-ledger-pane .d-flex button').forEach(b => b.classList.remove('active'));
    btn.classList.add('active');

    const rows = document.querySelectorAll('#ledgerTable tbody tr.ledger-row');
    rows.forEach(r => {
        const hasDebt = r.getAttribute('data-has-debt') === '1';
        if (mode === 'all') {
            r.style.display = '';
        } else if (mode === 'debt') {
            r.style.display = hasDebt ? '' : 'none';
        } else if (mode === 'settled') {
            r.style.display = !hasDebt ? '' : 'none';
        }
    });
}

// Search Filter
function searchLedgerTable() {
    const input = document.getElementById('ledgerSearchInput').value.toLowerCase();
    const rows = document.querySelectorAll('#ledgerTable tbody tr.ledger-row');
    rows.forEach(row => {
        const text = row.innerText.toLowerCase();
        row.style.display = text.includes(input) ? '' : 'none';
    });
}
</script>
