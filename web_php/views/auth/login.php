<!-- Branding Header -->
<div class="text-center mb-4">
    <div class="logo-box-pro d-inline-flex p-3 mb-3 shadow-lg rounded-4 position-relative">
        <div class="logo-glow-behind"></div>
        <i class="fa-solid fa-server fs-1 text-white position-relative z-2"></i>
    </div>
    <h3 class="fw-bold text-light mb-1 font-monospace" style="letter-spacing: 0.5px;">SASMAN MIKROTIK RADIUS</h3>
    <p class="text-white-50 fs-7 mb-0">لوحة التحكم وإدارة الشبكات وراديوس السحابي</p>
</div>

<!-- Pro Glassmorphic Login Card -->
<div class="card glass-card border-0 shadow-2xl p-4 p-sm-5 rounded-4 pro-login-card position-relative overflow-hidden">
    <!-- Ambient Glass Glow -->
    <div class="login-card-glow"></div>

    <div class="mb-4 text-center position-relative z-2">
        <h5 class="fw-bold text-light mb-1"><i class="fa-solid fa-shield-halved me-2 text-primary"></i> تسجيل الدخول الإداري</h5>
        <span class="text-muted fs-8">أدخل بيانات المدير للوصول إلى لوحة التحكم</span>
    </div>

    <form action="/login" method="POST" class="needs-validation position-relative z-2">
        <!-- Username Input -->
        <div class="mb-3">
            <label class="form-label text-light fs-8 fw-semibold mb-1">اسم المستخدم (Username)</label>
            <div class="input-group login-input-group">
                <span class="input-group-text bg-dark border-secondary border-opacity-50 text-info">
                    <i class="fa-solid fa-user-shield"></i>
                </span>
                <input type="text" name="username" id="loginUsername" class="form-control bg-dark border-secondary border-opacity-50 text-light py-2" placeholder="admin" required autofocus>
            </div>
        </div>

        <!-- Password Input with Show/Hide Toggle -->
        <div class="mb-3">
            <label class="form-label text-light fs-8 fw-semibold mb-1">كلمة المرور (Password)</label>
            <div class="input-group login-input-group">
                <span class="input-group-text bg-dark border-secondary border-opacity-50 text-warning">
                    <i class="fa-solid fa-lock"></i>
                </span>
                <input type="password" name="password" id="loginPassword" class="form-control bg-dark border-secondary border-opacity-50 text-light py-2" placeholder="••••••••" required>
                <button class="btn btn-dark border-secondary border-opacity-50 text-muted" type="button" onclick="togglePasswordVisibility()" title="إظهار / إخفاء كلمة المرور">
                    <i class="fa-solid fa-eye" id="passToggleIcon"></i>
                </button>
            </div>
        </div>

        <!-- Quick Demo Credentials Auto-Fill Pill -->
        <div class="d-flex justify-content-between align-items-center mb-4">
            <div class="form-check fs-8">
                <input class="form-check-input" type="checkbox" id="rememberMe" checked>
                <label class="form-check-label text-white-50" for="rememberMe">تذكر تسجيل دخولي</label>
            </div>
            <a href="javascript:void(0)" class="text-info fs-8 text-decoration-none fw-semibold" onclick="quickFillAdmin()">
                <i class="fa-solid fa-wand-magic-sparkles me-1"></i> تعبئة تجريبية
            </a>
        </div>

        <!-- Submit Button -->
        <button type="submit" class="btn btn-primary w-100 py-3 fw-bold shadow-lg rounded-pill pro-submit-btn">
            <i class="fa-solid fa-right-to-bracket me-2"></i> دخول المنظومة الآن
        </button>
    </form>

    <!-- Self-Service Subscriber Portal Link -->
    <div class="text-center mt-4 pt-3 border-top border-secondary border-opacity-25 position-relative z-2">
        <a href="/portal" class="text-info-emphasis fs-8 text-decoration-none fw-semibold hover-glow d-inline-flex align-items-center gap-1">
            <i class="fa-solid fa-user-astronaut text-info"></i> الانتقال إلى بوابة المشتركين (الخدمة الذاتية) <i class="fa-solid fa-arrow-left fs-9"></i>
        </a>
    </div>
</div>

<!-- System Security Footer -->
<div class="text-center mt-4 text-white-50 fs-9 position-relative z-2">
    <div><i class="fa-solid fa-lock me-1 text-success"></i> اتصال محمي ومؤمن عبر بروتوكول RadSec mTLS RFC 6614 & AES-256</div>
    <div class="mt-1 font-monospace opacity-75">SASMAN Core Engine v5.2.0 • Baghdad Time <?= date('Y') ?></div>
</div>

<style>
/* ============================================================
   PRO FUTURISTIC LOGIN STYLING
============================================================ */
.pro-auth-body {
    background: radial-gradient(circle at 50% 20%, #0f172a 0%, #090d16 60%, #020617 100%);
}

.logo-box-pro {
    background: linear-gradient(135deg, #0ea5e9 0%, #2563eb 100%);
    border: 1px solid rgba(255, 255, 255, 0.25);
    width: 72px;
    height: 72px;
    align-items: center;
    justify-content: center;
}

.logo-glow-behind {
    position: absolute;
    inset: -10px;
    background: radial-gradient(circle, rgba(14, 165, 233, 0.4) 0%, rgba(14, 165, 233, 0) 70%);
    border-radius: 50%;
    filter: blur(10px);
    pointer-events: none;
}

.pro-login-card {
    background: rgba(15, 23, 42, 0.75) !important;
    backdrop-filter: blur(20px);
    -webkit-backdrop-filter: blur(20px);
    border: 1px solid rgba(255, 255, 255, 0.12) !important;
    box-shadow: 0 25px 60px -15px rgba(0, 0, 0, 0.7) !important;
}

.login-card-glow {
    position: absolute;
    top: -40%;
    right: -30%;
    width: 250px;
    height: 250px;
    background: radial-gradient(circle, rgba(14, 165, 233, 0.15) 0%, rgba(14, 165, 233, 0) 70%);
    pointer-events: none;
    border-radius: 50%;
}

.login-input-group .form-control {
    font-size: 0.95rem;
    transition: all 0.25s ease;
}
.login-input-group .form-control:focus {
    border-color: #0ea5e9 !important;
    box-shadow: 0 0 15px rgba(14, 165, 233, 0.3) !important;
}

.pro-submit-btn {
    background: linear-gradient(135deg, #0ea5e9 0%, #2563eb 100%);
    border: none;
    font-size: 1.05rem;
    transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}
.pro-submit-btn:hover {
    transform: translateY(-2px);
    box-shadow: 0 12px 25px rgba(14, 165, 233, 0.4) !important;
    background: linear-gradient(135deg, #38bdf8 0%, #1d4ed8 100%);
}
.pro-submit-btn:active {
    transform: scale(0.98);
}

.hover-glow:hover {
    color: #38bdf8 !important;
    text-shadow: 0 0 8px rgba(56, 189, 248, 0.5);
}
</style>

<script>
function togglePasswordVisibility() {
    const input = document.getElementById('loginPassword');
    const icon = document.getElementById('passToggleIcon');
    if (input.type === 'password') {
        input.type = 'text';
        icon.className = 'fa-solid fa-eye-slash text-warning';
    } else {
        input.type = 'password';
        icon.className = 'fa-solid fa-eye';
    }
}

function quickFillAdmin() {
    document.getElementById('loginUsername').value = 'admin';
    document.getElementById('loginPassword').value = 'admin';
}
</script>
