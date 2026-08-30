<?php

namespace Sasman\Controllers;

use Sasman\Core\Controller;

class DashboardController extends Controller
{
    public function index(): void
    {
        // Fetch Live Stats from Core API
        $usersResp = $this->api->get('/users');
        $nasResp = $this->api->get('/nas/status');
        $meResp = $this->api->get('/auth/me');

        $users = [];
        if (isset($usersResp['users']) && is_array($usersResp['users'])) {
            $users = $usersResp['users'];
        } elseif (is_array($usersResp)) {
            $users = $usersResp;
        }

        $totalUsers = count($users);
        $activeUsers = 0;
        $expiredUsers = 0;
        $onlineUsers = 0;
        $aboutToExpire = 0;
        $totalDebt = 0;
        $onlineSessions = [];

        $now = time();
        $threeDaysLater = $now + (3 * 86400);

        foreach ($users as $u) {
            $uName = $u['user'] ?? $u['username'] ?? '';
            $isExpired = !empty($u['expired']);
            $isEnabled = !isset($u['enabled']) || !empty($u['enabled']);
            $session = $u['session'] ?? [];
            $isOnline = !empty($session['online']) || !empty($u['online']);

            if (!$isExpired && $isEnabled) {
                $activeUsers++;
            } else {
                $expiredUsers++;
            }

            if ($isOnline) {
                $onlineUsers++;
                $onlineSessions[] = [
                    'username' => $uName,
                    'ip' => $session['ip'] ?? $session['framed_ip_address'] ?? $u['framed_ip_address'] ?? '127.0.0.1',
                    'uptime' => $session['uptime'] ?? (!empty($session['session_seconds']) ? gmdate("H:i:s", $session['session_seconds']) : 'متصل الآن'),
                    'calling_station' => $session['calling_station'] ?? '',
                ];
            }

            // Check about to expire within 3 days
            $expStr = $u['expires_at'] ?? $u['expiration'] ?? '';
            if ($expStr && $expStr !== '—') {
                $expTs = strtotime($expStr);
                if ($expTs > $now && $expTs <= $threeDaysLater) {
                    $aboutToExpire++;
                }
            }

            // User Debt
            $bal = (float)($u['balance'] ?? 0);
            if ($bal > 0) {
                $totalDebt += $bal;
            }
        }

        $activePercent = ($totalUsers > 0) ? round(($activeUsers / $totalUsers) * 100) : 0;
        $onlinePercent = ($totalUsers > 0) ? round(($onlineUsers / $totalUsers) * 100) : 0;
        $expiredPercent = ($totalUsers > 0) ? round(($expiredUsers / $totalUsers) * 100) : 0;
        $adminBalance = (float)($meResp['balance'] ?? 0);

        $this->render('dashboard.index', [
            'title' => 'لوحة التحكم والإحصائيات الرئيسية — SASMAN',
            'total_users' => $totalUsers,
            'active_users' => $activeUsers,
            'online_users' => $onlineUsers,
            'expired_users' => $expiredUsers,
            'about_to_expire' => $aboutToExpire,
            'active_percent' => $activePercent,
            'online_percent' => $onlinePercent,
            'expired_percent' => $expiredPercent,
            'admin_balance' => $adminBalance,
            'total_debt' => $totalDebt,
            'nas_status' => $nasResp,
            'recent_users' => array_slice($users, 0, 8),
            'sessions' => $onlineSessions,
        ]);
    }

    public function statsApi(): void
    {
        $usersResp = $this->api->get('/users');
        $nasResp = $this->api->get('/nas/status');

        $users = is_array($usersResp) ? $usersResp : ($usersResp['users'] ?? []);
        $onlineCount = 0;
        foreach ($users as $u) {
            if (!empty($u['session']['online']) || !empty($u['online'])) {
                $onlineCount++;
            }
        }

        $this->json([
            'success' => true,
            'users_count' => count($users),
            'online_count' => $onlineCount,
            'nas' => $nasResp,
        ]);
    }
}
