<?php

namespace Sasman\Controllers;

use Sasman\Core\Controller;

class PortalController extends Controller
{
    public function index(): void
    {
        $sessionUser = $_SESSION['portal_user'] ?? '';
        $queryUser = trim($_GET['username'] ?? '');
        
        $username = $queryUser ?: $sessionUser;
        // User is authenticated ONLY if session user is set and matches
        $isAuth = !empty($sessionUser) && (empty($queryUser) || $sessionUser === $queryUser);
        
        $statusData = null;
        $streams = [];

        if ($username) {
            $statusData = $this->api->get('/portal/status', ['username' => $username]);
            $streamsResp = $this->api->get('/portal/streams', ['username' => $username]);
            if (is_array($streamsResp)) {
                $streams = $streamsResp;
            } elseif (isset($streamsResp['streams'])) {
                $streams = $streamsResp['streams'];
            }
        }

        $this->render('portal.index', [
            'title' => 'بوابة خدمة المشتركين — SASMAN Portal',
            'username' => $username,
            'is_auth' => $isAuth,
            'status' => $statusData,
            'streams' => $streams,
        ], 'portal');
    }

    public function login(): void
    {
        $username = trim($_POST['username'] ?? '');
        $username = ltrim($username, '@');
        $password = trim($_POST['password'] ?? '');

        if (!$username || !$password) {
            $this->setFlash('error', 'يرجى إدخال اسم المستخدم وكلمة المرور');
            $this->redirect('/portal');
            return;
        }

        $resp = $this->api->post('/portal/login', [
            'username' => $username,
            'password' => $password,
        ]);

        if (($resp['http_code'] ?? 500) < 300 && (isset($resp['token']) || !empty($resp['success']) || isset($resp['username']))) {
            $realUser = $resp['username'] ?? $username;
            $_SESSION['portal_user'] = $realUser;
            $this->setFlash('success', "مرحباً بك {$realUser}");
            $this->redirect('/portal');
            return;
        } else {
            $this->setFlash('error', $resp['error'] ?? 'اسم المستخدم أو كلمة المرور غير صحيحة');
            $this->redirect('/portal');
            return;
        }
    }

    public function logout(): void
    {
        unset($_SESSION['portal_user']);
        $this->setFlash('info', 'تم تسجيل الخروج من بوابة المشترك');
        $this->redirect('/portal');
    }

    public function redeem(): void
    {
        $username = trim($_POST['username'] ?? $_SESSION['portal_user'] ?? '');
        $username = ltrim($username, '@');
        $code = trim($_POST['code'] ?? '');

        if (!$username || !$code) {
            $this->setFlash('error', 'يرجى إدخال كود الكرت واسم المستخدم');
            $this->redirect('/portal');
            return;
        }

        $resp = $this->api->post('/vouchers/redeem', [
            'username' => $username,
            'code' => $code,
        ]);

        if (($resp['http_code'] ?? 500) < 300 && (isset($resp['message']) || !empty($resp['success']))) {
            if (!empty($_SESSION['portal_user'])) {
                $_SESSION['portal_user'] = $username;
            }
            $this->setFlash('success', 'تم شحن الكارت وتجديد اشتراكك بنجاح! 🎉');
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل شحن الكارت. يرجى التأكد من الرمز');
        }

        $this->redirect('/portal' . (empty($_SESSION['portal_user']) ? '?username=' . urlencode($username) : ''));
    }

    public function changePassword(): void
    {
        // Strict Security Gate: Password change is strictly forbidden without authenticated session
        $sessionUser = $_SESSION['portal_user'] ?? '';
        if (empty($sessionUser)) {
            $this->setFlash('error', '🚫 عملية غير مصرح بها! يجب تسجيل الدخول بالرمز السري أولاً لتغيير كلمة المرور');
            $this->redirect('/portal');
            return;
        }

        $oldPassword = trim($_POST['old_password'] ?? '');
        $newPassword = trim($_POST['new_password'] ?? '');

        if (!$oldPassword || !$newPassword) {
            $this->setFlash('error', 'يرجى إدخال كلمة المرور الحالية والجديدة');
            $this->redirect('/portal');
            return;
        }

        $resp = $this->api->post('/portal/password', [
            'username' => $sessionUser,
            'old_password' => $oldPassword,
            'new_password' => $newPassword,
        ]);

        if (($resp['http_code'] ?? 500) < 300 && (isset($resp['message']) || !empty($resp['success']))) {
            $this->setFlash('success', 'تم تغيير وتحديث كلمة المرور بنجاح');
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل تغيير كلمة المرور. يرجى التأكد من كلمة المرور الحالية');
        }

        $this->redirect('/portal');
    }
}
