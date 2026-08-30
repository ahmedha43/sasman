<!DOCTYPE html>
<html lang="ar" dir="rtl" data-bs-theme="dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title><?= htmlspecialchars($title ?? 'بوابة المشتركين — SASMAN') ?></title>
    <link rel="icon" type="image/x-icon" href="/favicon.ico">
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Cairo:wght@400;500;600;700;800;900&display=swap" rel="stylesheet">
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/css/bootstrap.rtl.min.css">
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.5.1/css/all.min.css">
    <link rel="stylesheet" href="/assets/css/style.css">
</head>
<body class="pro-portal-body text-light font-cairo min-vh-100 d-flex flex-column justify-content-between position-relative overflow-x-hidden">
    <!-- Futuristic Network Constellation Background Overlay -->
    <div class="portal-network-overlay" style="position: absolute; inset: 0; pointer-events: none; opacity: 0.12; z-index: 1;">
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1440 900" preserveAspectRatio="none" style="width: 100%; height: 100%;">
            <line x1="100" y1="100" x2="320" y2="220" stroke="#8b5cf6" stroke-width="1.2"/>
            <line x1="320" y1="220" x2="580" y2="100" stroke="#8b5cf6" stroke-width="1.2"/>
            <line x1="580" y1="100" x2="880" y2="280" stroke="#8b5cf6" stroke-width="1.2"/>
            <line x1="880" y1="280" x2="1180" y2="140" stroke="#8b5cf6" stroke-width="1.2"/>
            <line x1="1180" y1="140" x2="1350" y2="350" stroke="#8b5cf6" stroke-width="1.2"/>
            <line x1="220" y1="480" x2="480" y2="620" stroke="#8b5cf6" stroke-width="1.2"/>
            <line x1="480" y1="620" x2="820" y2="520" stroke="#8b5cf6" stroke-width="1.2"/>
            <line x1="820" y1="520" x2="1120" y2="680" stroke="#8b5cf6" stroke-width="1.2"/>
            <circle cx="100" cy="100" r="5" fill="#8b5cf6"/>
            <circle cx="320" cy="220" r="7" fill="#8b5cf6"/>
            <circle cx="580" cy="100" r="6" fill="#8b5cf6"/>
            <circle cx="880" cy="280" r="8" fill="#8b5cf6"/>
            <circle cx="1180" cy="140" r="6" fill="#8b5cf6"/>
            <circle cx="1350" cy="350" r="5" fill="#8b5cf6"/>
            <circle cx="220" cy="480" r="5" fill="#8b5cf6"/>
            <circle cx="480" cy="620" r="7" fill="#8b5cf6"/>
            <circle cx="820" cy="520" r="6" fill="#8b5cf6"/>
            <circle cx="1120" cy="680" r="8" fill="#8b5cf6"/>
        </svg>
    </div>

    <!-- Header Navigation -->
    <header class="navbar navbar-expand-lg border-bottom border-white border-opacity-10 p-3 glass-header position-relative z-2" style="background: rgba(15, 23, 42, 0.75); backdrop-filter: blur(16px);">
        <div class="container">
            <a class="navbar-brand d-flex align-items-center gap-2 fw-bold text-light" href="/portal">
                <div class="portal-brand-icon">
                    <i class="fa-solid fa-satellite-dish fs-4 text-white"></i>
                </div>
                <div>
                    <span class="d-block font-monospace fw-bold fs-5 text-light" style="letter-spacing: 0.5px;">SASMAN PORTAL</span>
                    <small class="text-white-50 fs-9 d-block">بوابة الخدمة الذاتية للمشتركين</small>
                </div>
            </a>
            <div class="d-flex align-items-center gap-2">
                <a href="/login" class="btn btn-outline-light btn-sm rounded-pill px-3 fs-8 fw-semibold">
                    <i class="fa-solid fa-shield-halved me-1 text-primary"></i> دخول الإدارة
                </a>
            </div>
        </div>
    </header>

    <!-- Main Content -->
    <main class="container my-4 flex-grow-1 position-relative z-2" style="max-width: 960px;">
        <?php $this->partial('alerts'); ?>
        <?= $content ?>
    </main>

    <!-- Footer -->
    <footer class="footer p-3 text-center border-top border-white border-opacity-10 text-white-50 fs-8 position-relative z-2" style="background: rgba(15, 23, 42, 0.75); backdrop-filter: blur(16px);">
        <div class="container">
            <span>منظومة <strong><?= htmlspecialchars($app_name ?? 'SASMAN') ?></strong> — بوابة المشتركين السحابية المباشرة © <?= date('Y') ?></span>
        </div>
    </footer>

    <style>
    .pro-portal-body {
        background: radial-gradient(circle at 50% 15%, #1e1b4b 0%, #0f172a 50%, #020617 100%);
    }
    .portal-brand-icon {
        width: 42px;
        height: 42px;
        border-radius: 12px;
        background: linear-gradient(135deg, #8b5cf6 0%, #6366f1 100%);
        border: 1px solid rgba(255, 255, 255, 0.2);
        display: flex;
        align-items: center;
        justify-content: center;
        box-shadow: 0 4px 12px rgba(139, 92, 246, 0.3);
    }
    </style>

    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/js/bootstrap.bundle.min.js"></script>
</body>
</html>
