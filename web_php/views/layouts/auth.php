<!DOCTYPE html>
<html lang="ar" dir="rtl" data-bs-theme="dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title><?= htmlspecialchars($title ?? 'تسجيل الدخول — SASMAN') ?></title>
    <link rel="icon" type="image/x-icon" href="/favicon.ico">
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Cairo:wght@400;500;600;700;800;900&display=swap" rel="stylesheet">
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/css/bootstrap.rtl.min.css">
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.5.1/css/all.min.css">
    <link rel="stylesheet" href="/assets/css/style.css">
</head>
<body class="pro-auth-body d-flex align-items-center justify-content-center min-vh-100 font-cairo position-relative overflow-x-hidden">
    <!-- Futuristic Network Constellation Background Overlay -->
    <div class="auth-network-overlay" style="position: absolute; inset: 0; pointer-events: none; opacity: 0.12; z-index: 1;">
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1440 900" preserveAspectRatio="none" style="width: 100%; height: 100%;">
            <line x1="120" y1="80" x2="300" y2="250" stroke="#38bdf8" stroke-width="1.2"/>
            <line x1="300" y1="250" x2="600" y2="120" stroke="#38bdf8" stroke-width="1.2"/>
            <line x1="600" y1="120" x2="900" y2="300" stroke="#38bdf8" stroke-width="1.2"/>
            <line x1="900" y1="300" x2="1200" y2="150" stroke="#38bdf8" stroke-width="1.2"/>
            <line x1="1200" y1="150" x2="1380" y2="400" stroke="#38bdf8" stroke-width="1.2"/>
            <line x1="200" y1="500" x2="450" y2="650" stroke="#38bdf8" stroke-width="1.2"/>
            <line x1="450" y1="650" x2="800" y2="550" stroke="#38bdf8" stroke-width="1.2"/>
            <line x1="800" y1="550" x2="1100" y2="700" stroke="#38bdf8" stroke-width="1.2"/>
            <circle cx="120" cy="80" r="5" fill="#38bdf8"/>
            <circle cx="300" cy="250" r="7" fill="#38bdf8"/>
            <circle cx="600" cy="120" r="6" fill="#38bdf8"/>
            <circle cx="900" cy="300" r="8" fill="#38bdf8"/>
            <circle cx="1200" cy="150" r="6" fill="#38bdf8"/>
            <circle cx="1380" cy="400" r="5" fill="#38bdf8"/>
            <circle cx="200" cy="500" r="5" fill="#38bdf8"/>
            <circle cx="450" cy="650" r="7" fill="#38bdf8"/>
            <circle cx="800" cy="550" r="6" fill="#38bdf8"/>
            <circle cx="1100" cy="700" r="8" fill="#38bdf8"/>
        </svg>
    </div>

    <div class="container py-4 position-relative z-2" style="max-width: 480px;">
        <?php $this->partial('alerts'); ?>
        <?= $content ?>
    </div>

    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/js/bootstrap.bundle.min.js"></script>
</body>
</html>
