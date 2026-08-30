<?php

namespace Sasman\Controllers;

use Sasman\Core\Controller;

class UsersController extends Controller
{
    public function index(): void
    {
        $usersResp = $this->api->get('/users');
        $profilesResp = $this->api->get('/profiles');
        $sessionsResp = $this->api->get('/sessions');

        $users = [];
        if (isset($usersResp['users']) && is_array($usersResp['users'])) {
            $users = $usersResp['users'];
        } elseif (is_array($usersResp)) {
            foreach ($usersResp as $k => $v) {
                if (is_array($v) && (isset($v['username']) || isset($v['user']))) {
                    $users[] = $v;
                }
            }
        }

        $profiles = [];
        if (isset($profilesResp['profiles']) && is_array($profilesResp['profiles'])) {
            $profiles = $profilesResp['profiles'];
        } elseif (is_array($profilesResp)) {
            foreach ($profilesResp as $k => $v) {
                if (is_array($v) && isset($v['name'])) {
                    $profiles[] = $v;
                }
            }
        }

        $sessions = is_array($sessionsResp) ? $sessionsResp : [];

        // KPI Counts
        $totalUsers = count($users);
        $activeUsers = 0;
        $onlineUsers = 0;
        $expiredUsers = 0;
        $aboutToExpire = 0;
        $totalDebt = 0;

        $now = time();
        $threeDaysLater = $now + (3 * 86400);

        foreach ($users as $u) {
            $username = $u['username'] ?? $u['user'] ?? '';
            $status = $u['status'] ?? 'active';
            $isExp = ($status === 'expired' || $status === 'منتهي');
            $isEnabled = ($status !== 'disabled' && $status !== 'معطل');
            
            $session = $sessions[$username] ?? null;
            $isOnline = !empty($session['online']) || !empty($u['online']);

            if (!$isExp && $isEnabled) {
                $activeUsers++;
            } else {
                $expiredUsers++;
            }

            if ($isOnline) {
                $onlineUsers++;
            }

            $expStr = $u['expires_at'] ?? $u['expiration'] ?? '';
            if ($expStr && $expStr !== '—') {
                $expTs = strtotime($expStr);
                if ($expTs > $now && $expTs <= $threeDaysLater) {
                    $aboutToExpire++;
                }
            }

            $bal = (float)($u['balance'] ?? 0);
            if ($bal > 0) {
                $totalDebt += $bal;
            }
        }

        $this->render('users.index', [
            'title' => 'إدارة المشتركين والحسابات — SASMAN',
            'users' => $users,
            'profiles' => $profiles,
            'total_users' => $totalUsers,
            'active_users' => $activeUsers,
            'online_users' => $onlineUsers,
            'expired_users' => $expiredUsers,
            'about_to_expire' => $aboutToExpire,
            'total_debt' => $totalDebt,
        ]);
    }

    public function create(): void
    {
        $user = trim($_POST['user'] ?? $_POST['username'] ?? '');
        $oldUser = trim($_POST['old_user'] ?? '');
        $pass = trim($_POST['pass'] ?? $_POST['password'] ?? '');
        $profile = trim($_POST['profile'] ?? '');
        $fullName = trim($_POST['full_name'] ?? '');
        $phone = trim($_POST['phone'] ?? '');
        $expiresAt = trim($_POST['expires_at'] ?? '');

        if ($oldUser) {
            $this->requirePermission('can_edit_users', '🚫 ليس لديك صلاحية لتعديل بيانات المشتركين');
        } else {
            $this->requirePermission('can_create_users', '🚫 ليس لديك صلاحية لإضافة مشتركين جدد');
        }

        if (!$user || !$pass || !$profile) {
            $this->setFlash('error', 'يرجى إدخال اسم المستخدم وكلمة المرور والباقة');
            $this->redirect('/users');
            return;
        }

        $payload = [
            'user' => $user,
            'pass' => $pass,
            'profile' => $profile,
            'full_name' => $fullName,
            'phone' => $phone,
        ];

        if ($oldUser) {
            $payload['old_user'] = $oldUser;
        }
        if ($expiresAt) {
            $payload['expires_at'] = $expiresAt;
        }

        $resp = $this->api->post('/users', $payload);

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', $oldUser ? "تم تحديث المشترك [{$user}] بنجاح" : "تمت إضافة المشترك [{$user}] بنجاح");
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل حفظ المشترك');
        }

        $this->redirect('/users');
    }

    public function renew(array $params): void
    {
        $this->requirePermission('can_renew_users', '🚫 ليس لديك صلاحية لتجديد اشتراكات المشتركين');

        $username = $params['username'] ?? '';
        $profile = trim($_POST['profile'] ?? '');
        $price = (float)($_POST['price'] ?? 0);
        $validityDays = (int)($_POST['validity_days'] ?? 30);
        $paymentStatus = trim($_POST['payment_status'] ?? 'paid');
        $notes = trim($_POST['notes'] ?? 'تجديد اشتراك عبر لوحة التحكم');

        if (!$username) {
            $this->setFlash('error', 'اسم المشترك مطلوب');
            $this->redirect('/users');
            return;
        }

        $payload = [
            'profile' => $profile,
            'price' => $price,
            'validity_days' => $validityDays,
            'payment_status' => $paymentStatus,
            'notes' => $notes,
        ];

        $resp = $this->api->post("/users/{$username}/renew", $payload);

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', "تم تجديد اشتراك المشترك [{$username}] بنجاح");
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل تجديد الاشتراك');
        }

        $this->redirect('/users');
    }

    public function toggleStatus(array $params): void
    {
        $this->requirePermission('can_toggle_users', '🚫 ليس لديك صلاحية لتعطيل أو تفعيل حساب المشترك');

        $username = $params['username'] ?? '';
        $resp = $this->api->post("/users/{$username}/toggle-status");

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || isset($resp['enabled']) || !empty($resp['success'])) {
            $msg = $resp['message'] ?? 'تم تغيير حالة حساب المشترك بنجاح';
            $this->setFlash('success', "{$msg} [{$username}]");
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل تغيير حالة الحساب');
        }

        $this->redirect('/users');
    }

    public function disconnect(array $params): void
    {
        $this->requirePermission('can_disconnect_users', '🚫 ليس لديك صلاحية لفصل جلسة المشترك من المايكروتك');

        $username = $params['username'] ?? '';
        $resp = $this->api->post("/users/{$username}/disconnect");

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $msg = $resp['message'] ?? 'تم فصل جلسة المشترك من المايكروتك بنجاح';
            $this->setFlash('success', "{$msg} [{$username}]");
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل فصل الجلسة');
        }

        $this->redirect('/users');
    }

    public function details(array $params): void
    {
        $username = $params['username'] ?? '';
        $resp = $this->api->get("/users/{$username}/details");
        $this->json($resp);
    }

    public function delete(array $params): void
    {
        $this->requirePermission('can_delete_users', '🚫 ليس لديك صلاحية لحذف المشتركين نهائياً');

        $username = $params['username'] ?? '';
        $resp = $this->api->delete("/users/{$username}");

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', "تم حذف المشترك [{$username}] بنجاح");
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل حذف المشترك');
        }

        $this->redirect('/users');
    }
}
