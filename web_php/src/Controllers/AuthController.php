<?php

namespace Sasman\Controllers;

use Sasman\Core\Controller;

class AuthController extends Controller
{
    public function showLogin(): void
    {
        if (isset($_SESSION['sasman_token'])) {
            $this->redirect('/');
            return;
        }

        $this->render('auth.login', [
            'title' => 'تسجيل الدخول — SASMAN Platform'
        ], 'auth');
    }

    public function login(): void
    {
        $username = trim($_POST['username'] ?? '');
        $password = trim($_POST['password'] ?? '');

        if (!$username || !$password) {
            $this->setFlash('error', 'يرجى إدخال اسم المستخدم وكلمة المرور');
            $this->redirect('/login');
            return;
        }

        $resp = $this->api->post('/auth/login', [
            'username' => $username,
            'password' => $password,
        ]);

        if (isset($resp['token']) && !empty($resp['token'])) {
            $this->api->setToken($resp['token']);
            $admin = $resp['admin'] ?? [];
            $_SESSION['sasman_admin'] = [
                'id' => (int)($admin['id'] ?? 0),
                'username' => $admin['username'] ?? $username,
                'name' => $admin['name'] ?? $admin['full_name'] ?? $username,
                'full_name' => $admin['name'] ?? $admin['full_name'] ?? $username,
                'email' => $admin['email'] ?? '',
                'role' => $admin['role'] ?? 'agent',
                'parent_id' => $admin['parent_id'] ?? null,
                'balance' => (float)($admin['balance'] ?? 0),
                'can_manage_profiles' => !empty($admin['can_manage_profiles']),
                'can_manage_nas' => !empty($admin['can_manage_nas']),
            ];
            
            $this->setFlash('success', 'تم تسجيل الدخول بنجاح');
            $this->redirect('/');
            return;
        }

        $errorMsg = $resp['error'] ?? 'اسم المستخدم أو كلمة المرور غير صحيحة';
        $this->setFlash('error', $errorMsg);
        $this->redirect('/login');
    }

    public function logout(): void
    {
        $this->api->post('/auth/logout');
        $this->api->clearToken();
        $this->setFlash('success', 'تم تسجيل الخروج بنجاح');
        $this->redirect('/login');
    }
}
