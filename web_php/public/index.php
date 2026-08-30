<?php

// Front Controller for SASMAN PHP Layer
session_start();

// 1. Load Configuration
$config = require __DIR__ . '/../config/app.php';

// 2. Simple PSR-4 Autoloader
spl_autoload_register(function ($class) {
    $prefix = 'Sasman\\';
    $baseDir = __DIR__ . '/../src/';

    $len = strlen($prefix);
    if (strncmp($prefix, $class, $len) !== 0) {
        return;
    }

    $relativeClass = substr($class, $len);
    $file = $baseDir . str_replace('\\', '/', $relativeClass) . '.php';

    if (file_exists($file)) {
        require $file;
    }
});

// 3. Initialize Core Instances
$apiClient = new \Sasman\Core\ApiClient($config);
$view = new \Sasman\Core\View(__DIR__ . '/../views');
$router = new \Sasman\Core\Router($apiClient, $view, $config);

// 4. Transparent API Proxy for JS modules (core.js, users.js, nas.js, vouchers.js, etc.)
$uri = parse_url($_SERVER['REQUEST_URI'] ?? '/', PHP_URL_PATH);
if (str_starts_with($uri, '/radius/api/') || (str_starts_with($uri, '/api/') && !str_starts_with($uri, '/api/stats'))) {
    $endpoint = str_starts_with($uri, '/radius/api/') ? substr($uri, 12) : substr($uri, 5);
    $method = $_SERVER['REQUEST_METHOD'] ?? 'GET';
    
    $rawInput = file_get_contents('php://input');
    $inputData = !empty($rawInput) ? json_decode($rawInput, true) : $_POST;
    if (empty($inputData) && $method === 'GET') {
        $inputData = $_GET;
    }
    
    $resp = $apiClient->request($method, $endpoint, $inputData ?: []);
    $code = $resp['http_code'] ?? 200;
    http_response_code($code);
    header('Content-Type: application/json; charset=utf-8');
    echo json_encode($resp, JSON_UNESCAPED_UNICODE);
    exit;
}

// 5. Define Routes

// Auth Routes
$router->get('/login', 'AuthController@showLogin');
$router->post('/login', 'AuthController@login');
$router->get('/logout', 'AuthController@logout');

// Protected Dashboard & Operations Routes
$auth = [\Sasman\Middleware\AuthMiddleware::class];

$router->get('/', 'DashboardController@index', $auth);
$router->get('/api/stats', 'DashboardController@statsApi', $auth);

// Subscribers (Users)
$router->get('/users', 'UsersController@index', $auth);
$router->post('/users', 'UsersController@create', $auth);
$router->get('/users/:username/details', 'UsersController@details', $auth);
$router->get('/users/:username/renew', 'UsersController@renew', $auth);
$router->post('/users/:username/renew', 'UsersController@renew', $auth);
$router->get('/users/:username/toggle-status', 'UsersController@toggleStatus', $auth);
$router->get('/users/:username/disconnect', 'UsersController@disconnect', $auth);
$router->get('/users/:username/delete', 'UsersController@delete', $auth);

// Live Sessions (PPPoE / Hotspot Online Users)
$router->get('/sessions', 'SessionsController@index', $auth);
$router->get('/sessions/:username/disconnect', 'SessionsController@disconnect', $auth);

// Vouchers (Cards)
$router->get('/vouchers', 'VouchersController@index', $auth);
$router->post('/vouchers', 'VouchersController@generate', $auth);
$router->post('/vouchers/generate', 'VouchersController@generate', $auth);
$router->get('/vouchers/clear', 'VouchersController@clearAll', $auth);
$router->get('/vouchers/:id/delete', 'VouchersController@delete', $auth);

// Profiles (Speed Packages)
$router->get('/profiles', 'ProfilesController@index', $auth);
$router->post('/profiles', 'ProfilesController@create', $auth);
$router->get('/profiles/:name/delete', 'ProfilesController@delete', $auth);

// NAS & MikroTik
$router->get('/nas', 'NasController@index', $auth);
$router->post('/nas', 'NasController@create', $auth);
$router->get('/nas/:ip/delete', 'NasController@delete', $auth);
$router->post('/nas/quick-setup', 'NasController@quickSetup', $auth);
$router->post('/nas/:id/generate-cert', 'NasController@generateCert', $auth);

// Devices & Antennas (CPE)
$router->get('/devices', 'DevicesController@index', $auth);
$router->post('/devices', 'DevicesController@create', $auth);
$router->get('/devices/:id/delete', 'DevicesController@delete', $auth);
$router->post('/devices/:id/poll', 'DevicesController@poll', $auth);

// Resellers & Sub-Admins
$router->get('/admins', 'AdminsController@index', $auth);
$router->post('/admins', 'AdminsController@create', $auth);
$router->post('/admins/recharge', 'AdminsController@recharge', $auth);
$router->post('/admins/withdraw', 'AdminsController@withdraw', $auth);
$router->post('/admins/:id/permissions', 'AdminsController@updatePermissions', $auth);
$router->post('/admins/permissions', 'AdminsController@updatePermissions', $auth);
$router->get('/admins/:id/delete', 'AdminsController@delete', $auth);

// Transactions & Billing
$router->get('/transactions', 'TransactionsController@index', $auth);
$router->post('/transactions', 'TransactionsController@create', $auth);

// WhatsApp
$router->get('/whatsapp', 'WhatsAppController@index', $auth);
$router->post('/whatsapp/config', 'WhatsAppController@saveConfig', $auth);
$router->post('/whatsapp/templates', 'WhatsAppController@saveTemplate', $auth);
$router->post('/whatsapp/test', 'WhatsAppController@test', $auth);
$router->post('/whatsapp/send-debt-reminder', 'WhatsAppController@sendDebtReminders', $auth);
$router->get('/whatsapp/logout', 'WhatsAppController@logout', $auth);
$router->post('/whatsapp/logout', 'WhatsAppController@logout', $auth);

// IPTV Live Streams
$router->get('/streams', 'StreamsController@index', $auth);
$router->post('/streams', 'StreamsController@create', $auth);
$router->get('/streams/:id/delete', 'StreamsController@delete', $auth);

// Logs & Audit Trail
$router->get('/logs', 'LogsController@index', $auth);
$router->post('/logs/clear', 'LogsController@clear', $auth);

// System Settings, Backups, SAS4 & Excel Migration
$router->get('/settings', 'SettingsController@index', $auth);
$router->post('/settings/password', 'SettingsController@changePassword', $auth);
$router->post('/settings/telegram', 'SettingsController@saveTelegram', $auth);
$router->post('/settings/backup/telegram/test', 'SettingsController@testTelegram', $auth);
$router->post('/settings/tunnel', 'SettingsController@saveTunnel', $auth);
$router->get('/settings/backup/download', 'SettingsController@downloadBackup', $auth);
$router->post('/settings/database/restore', 'SettingsController@restoreDatabase', $auth);
$router->post('/settings/import/sas4', 'SettingsController@importSAS4', $auth);
$router->get('/settings/export/excel', 'SettingsController@exportExcel', $auth);
$router->post('/settings/import/excel', 'SettingsController@importExcel', $auth);

// Subscriber Portal (Public)
$router->get('/portal', 'PortalController@index');
$router->post('/portal/login', 'PortalController@login');
$router->post('/portal/redeem', 'PortalController@redeem');
$router->post('/portal/password', 'PortalController@changePassword');
$router->get('/portal/logout', 'PortalController@logout');

// 6. Dispatch Request
$router->dispatch();
