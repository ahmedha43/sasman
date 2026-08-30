<?php

namespace Sasman\Controllers;

use Sasman\Core\Controller;

class AdminsController extends Controller
{
    public function index(): void
    {
        $adminsResp = $this->api->get('/auth/admins');
        $transResp = $this->api->get('/auth/admins/transactions');

        $admins = [];
        if (isset($adminsResp['admins']) && is_array($adminsResp['admins'])) {
            $admins = $adminsResp['admins'];
        } elseif (is_array($adminsResp)) {
            foreach ($adminsResp as $k => $v) {
                if (is_array($v) && (isset($v['username']) || isset($v['id']))) {
                    $admins[] = $v;
                }
            }
        }

        $transactions = [];
        if (isset($transResp['transactions']) && is_array($transResp['transactions'])) {
            $transactions = $transResp['transactions'];
        } elseif (is_array($transResp)) {
            foreach ($transResp as $k => $v) {
                if (is_array($v) && isset($v['amount'])) {
                    $transactions[] = $v;
                }
            }
        }

        $this->render('admins.index', [
            'title' => 'إدارة الموزعين والوكلاء الفرعيين — SASMAN',
            'admins' => $admins,
            'transactions' => $transactions,
        ]);
    }

    public function create(): void
    {
        $username = trim($_POST['username'] ?? '');
        $password = trim($_POST['password'] ?? '');
        $name = trim($_POST['name'] ?? '');
        $email = trim($_POST['email'] ?? '');
        $role = trim($_POST['role'] ?? 'agent');

        $perms = [
            'can_create_users' => !empty($_POST['can_create_users']),
            'can_edit_users' => !empty($_POST['can_edit_users']),
            'can_delete_users' => !empty($_POST['can_delete_users']),
            'can_toggle_users' => !empty($_POST['can_toggle_users']),
            'can_disconnect_users' => !empty($_POST['can_disconnect_users']),
            'can_renew_users' => !empty($_POST['can_renew_users']),
            'can_generate_vouchers' => !empty($_POST['can_generate_vouchers']),
            'can_delete_vouchers' => !empty($_POST['can_delete_vouchers']),
            'can_print_vouchers' => !empty($_POST['can_print_vouchers']),
            'can_manage_profiles' => !empty($_POST['can_manage_profiles']),
            'can_manage_nas' => !empty($_POST['can_manage_nas']),
            'can_manage_devices' => !empty($_POST['can_manage_devices']),
            'can_manage_transactions' => !empty($_POST['can_manage_transactions']),
            'can_manage_subagents' => !empty($_POST['can_manage_subagents']),
            'can_view_logs' => !empty($_POST['can_view_logs']),
            'can_clear_logs' => !empty($_POST['can_clear_logs']),
            'can_manage_whatsapp' => !empty($_POST['can_manage_whatsapp']),
            'can_manage_streams' => !empty($_POST['can_manage_streams']),
        ];

        if (!$username || !$password) {
            $this->setFlash('error', 'يرجى إدخال اسم المستخدم وكلمة المرور');
            $this->redirect('/admins');
            return;
        }

        $payload = array_merge([
            'username' => $username,
            'password' => $password,
            'name' => $name,
            'email' => $email,
            'role' => $role,
        ], $perms);

        $resp = $this->api->post('/auth/register', $payload);

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success']) || isset($resp['admin'])) {
            $this->setFlash('success', "تم إنشاء حساب الوكيل [{$username}] بنجاح");
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل إنشاء حساب الوكيل');
        }

        $this->redirect('/admins');
    }

    public function updatePermissions(array $params): void
    {
        $id = (int)($params['id'] ?? $_POST['admin_id'] ?? 0);
        if ($id <= 0) {
            $this->setFlash('error', 'معرّف الوكيل غير صالح');
            $this->redirect('/admins');
            return;
        }

        $perms = [
            'can_create_users' => !empty($_POST['can_create_users']),
            'can_edit_users' => !empty($_POST['can_edit_users']),
            'can_delete_users' => !empty($_POST['can_delete_users']),
            'can_toggle_users' => !empty($_POST['can_toggle_users']),
            'can_disconnect_users' => !empty($_POST['can_disconnect_users']),
            'can_renew_users' => !empty($_POST['can_renew_users']),
            'can_generate_vouchers' => !empty($_POST['can_generate_vouchers']),
            'can_delete_vouchers' => !empty($_POST['can_delete_vouchers']),
            'can_print_vouchers' => !empty($_POST['can_print_vouchers']),
            'can_manage_profiles' => !empty($_POST['can_manage_profiles']),
            'can_manage_nas' => !empty($_POST['can_manage_nas']),
            'can_manage_devices' => !empty($_POST['can_manage_devices']),
            'can_manage_transactions' => !empty($_POST['can_manage_transactions']),
            'can_manage_subagents' => !empty($_POST['can_manage_subagents']),
            'can_view_logs' => !empty($_POST['can_view_logs']),
            'can_clear_logs' => !empty($_POST['can_clear_logs']),
            'can_manage_whatsapp' => !empty($_POST['can_manage_whatsapp']),
            'can_manage_streams' => !empty($_POST['can_manage_streams']),
        ];

        $resp = $this->api->post("/auth/admins/{$id}/permissions", $perms);

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success']) || isset($resp['admin'])) {
            $this->setFlash('success', 'تم تحديث مصفوفة صلاحيات الوكيل بنجاح');
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل تحديث الصلاحيات');
        }

        $this->redirect('/admins');
    }

    public function recharge(): void
    {
        $adminId = (int)($_POST['admin_id'] ?? 0);
        $amount = (float)($_POST['amount'] ?? 0);
        $notes = trim($_POST['notes'] ?? 'شحن رصيد وكيل');

        if ($adminId <= 0 || $amount <= 0) {
            $this->setFlash('error', 'يرجى تحديد الوكيل والمبلغ بشكل صحيح');
            $this->redirect('/admins');
            return;
        }

        $resp = $this->api->post('/auth/recharge', [
            'admin_id' => $adminId,
            'amount' => $amount,
            'notes' => $notes,
        ]);

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', 'تم شحن رصيد الوكيل بنجاح');
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل شحن الرصيد');
        }

        $this->redirect('/admins');
    }

    public function withdraw(): void
    {
        $adminId = (int)($_POST['admin_id'] ?? 0);
        $amount = (float)($_POST['amount'] ?? 0);
        $notes = trim($_POST['notes'] ?? 'سحب رصيد وكيل');

        if ($adminId <= 0 || $amount <= 0) {
            $this->setFlash('error', 'يرجى تحديد الوكيل والمبلغ بشكل صحيح');
            $this->redirect('/admins');
            return;
        }

        $resp = $this->api->post('/auth/withdraw', [
            'admin_id' => $adminId,
            'amount' => $amount,
            'notes' => $notes,
        ]);

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', 'تم سحب الرصيد من الوكيل بنجاح');
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل سحب الرصيد');
        }

        $this->redirect('/admins');
    }

    public function delete(array $params): void
    {
        $id = (int)($params['id'] ?? 0);
        $resp = $this->api->delete("/auth/admins/{$id}");

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', 'تم حذف حساب الوكيل بنجاح');
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل حذف الحساب');
        }

        $this->redirect('/admins');
    }
}
