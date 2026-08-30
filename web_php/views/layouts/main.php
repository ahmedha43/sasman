<!DOCTYPE html>
<html lang="ar" dir="rtl" data-bs-theme="dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title><?= htmlspecialchars($title ?? 'SASMAN Platform') ?></title>
    <!-- Google Fonts: Cairo -->
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Cairo:wght@400;500;600;700;800;900&display=swap" rel="stylesheet">
    <!-- Bootstrap 5 RTL CSS -->
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/css/bootstrap.rtl.min.css">
    <!-- FontAwesome 6 Icons -->
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.5.1/css/all.min.css">
    <!-- Chart.js for Live Network & Financial Graphs -->
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <!-- Custom Glassmorphism Styling -->
    <link rel="stylesheet" href="/assets/css/style.css">
</head>
<body class="bg-main text-light font-cairo">
    <div class="d-flex" id="wrapper">
        <!-- Sidebar Partial -->
        <?php $this->partial('sidebar'); ?>

        <!-- Page Content -->
        <div id="page-content-wrapper" class="flex-grow-1 min-vh-100 d-flex flex-column">
            <!-- Header Partial -->
            <?php $this->partial('header'); ?>

            <!-- Main Body Container -->
            <main class="container-fluid p-4 flex-grow-1">
                <!-- Alerts Partial -->
                <?php $this->partial('alerts'); ?>

                <!-- Injected View Content -->
                <?= $content ?>
            </main>

            <!-- Footer -->
            <footer class="footer p-3 text-center border-top border-dark-subtle text-muted fs-7">
                <span>منظومة <strong><?= htmlspecialchars($app_name) ?></strong> v<?= htmlspecialchars($app_version) ?> — منصة إدارة شبكات المايكروتك و RADIUS الشاملة © <?= date('Y') ?></span>
            </footer>
        </div>
    </div>

    <!-- Bootstrap 5 JS Bundle -->
    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/js/bootstrap.bundle.min.js"></script>
    <script src="/assets/js/app.js"></script>
</body>
</html>
