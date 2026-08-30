<?php

namespace Sasman\Controllers;

use Sasman\Core\Controller;

class SettingsController extends Controller
{
    public function index(): void
    {
        $meResp = $this->api->get('/auth/me');
        $telegramResp = $this->isSuperAdmin() ? $this->api->get('/auth/backup/telegram') : [];
        $tunnelResp = $this->isSuperAdmin() ? $this->api->get('/auth/tunnel/config') : [];
        $shutdownResp = $this->isSuperAdmin() ? $this->api->get('/auth/shutdown/config') : [];

        $this->render('settings.index', [
            'title' => 'إعدادات الحساب والنظام والنسخ — SASMAN',
            'admin' => $meResp,
            'telegram' => $telegramResp,
            'tunnel' => $tunnelResp,
            'shutdown' => $shutdownResp,
        ]);
    }

    public function changePassword(): void
    {
        $oldPassword = trim($_POST['old_password'] ?? '');
        $newPassword = trim($_POST['new_password'] ?? '');
        $confirmPassword = trim($_POST['confirm_password'] ?? '');

        if (!$oldPassword || !$newPassword) {
            $this->setFlash('error', 'يرجى إدخال كلمة المرور الحالية والجديدة');
            $this->redirect('/settings');
            return;
        }

        if ($newPassword !== $confirmPassword) {
            $this->setFlash('error', 'كلمة المرور الجديدة غير متطابقة مع التأكيد');
            $this->redirect('/settings');
            return;
        }

        $resp = $this->api->post('/auth/password', [
            'current_password' => $oldPassword,
            'new_password' => $newPassword,
        ]);

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', 'تم تغيير كلمة المرور بنجاح');
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل تغيير كلمة المرور');
        }

        $this->redirect('/settings');
    }

    public function saveTelegram(): void
    {
        $this->requireSuperAdmin();
        $enabled = isset($_POST['enabled']) ? 1 : 0;
        $botToken = trim($_POST['bot_token'] ?? '');
        $chatID = trim($_POST['chat_id'] ?? '');
        $intervalHours = (int)($_POST['interval_hours'] ?? 24);

        $resp = $this->api->post('/auth/backup/telegram', [
            'enabled' => $enabled == 1,
            'bot_token' => $botToken,
            'chat_id' => $chatID,
            'interval_hours' => $intervalHours,
        ]);

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', 'تم حفظ إعدادات النسخ الاحتياطي عبر تيليجرام بنجاح');
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل حفظ إعدادات تيليجرام');
        }

        $this->redirect('/settings');
    }

    public function testTelegram(): void
    {
        $this->requireSuperAdmin();
        $resp = $this->api->post('/auth/backup/telegram/test');

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', 'تم إرسال نسخة احتياطية تجريبية إلى محادثة تيليجرام بنجاح! 🚀');
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل إرسال النسخة التجريبية لتيليجرام');
        }

        $this->redirect('/settings');
    }

    public function saveTunnel(): void
    {
        $this->requireSuperAdmin();
        $subdomain = trim($_POST['subdomain'] ?? '');
        $centralDomain = trim($_POST['central_domain'] ?? 'sas-man.net');

        $resp = $this->api->post('/auth/tunnel/config', [
            'subdomain' => $subdomain,
            'central_domain' => $centralDomain,
        ]);

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', 'تم تحديث إعدادات النطاق الفرعي والنفق السحابي بنجاح');
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل حفظ إعدادات النفق');
        }

        $this->redirect('/settings');
    }

    public function downloadBackup(): void
    {
        $this->requireSuperAdmin();
        $token = $_SESSION['sasman_token'] ?? $_SESSION['token'] ?? '';
        $ch = curl_init('http://127.0.0.1:8080/radius/api/auth/backup');
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
        curl_setopt($ch, CURLOPT_FOLLOWLOCATION, true);
        curl_setopt($ch, CURLOPT_HTTPHEADER, [
            'Authorization: Bearer ' . $token,
        ]);
        $response = curl_exec($ch);
        $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
        curl_close($ch);

        if ($httpCode === 200 && !empty($response)) {
            $filename = 'sasman-backup-' . date('Y-m-d_H-i') . '.sqlite';
            header('Content-Description: File Transfer');
            header('Content-Type: application/x-sqlite3');
            header('Content-Disposition: attachment; filename="' . $filename . '"');
            header('Expires: 0');
            header('Cache-Control: must-revalidate');
            header('Pragma: public');
            header('Content-Length: ' . strlen($response));
            echo $response;
            exit;
        }

        $this->setFlash('error', 'فشل تحميل ملف النسخة الاحتياطية');
        $this->redirect('/settings');
    }

    public function restoreDatabase(): void
    {
        $this->requireSuperAdmin();
        if (!isset($_FILES['file']) || $_FILES['file']['error'] !== UPLOAD_ERR_OK) {
            $this->setFlash('error', 'يرجى اختيار ملف قاعدة بيانات صالح (.sqlite أو .db)');
            $this->redirect('/settings');
            return;
        }

        $filePath = $_FILES['file']['tmp_name'];
        $fileName = $_FILES['file']['name'];
        $token = $_SESSION['sasman_token'] ?? $_SESSION['token'] ?? '';

        $cFile = curl_file_create($filePath, $_FILES['file']['type'], $fileName);
        $ch = curl_init('http://127.0.0.1:8080/radius/api/auth/restore');
        curl_setopt($ch, CURLOPT_POST, true);
        curl_setopt($ch, CURLOPT_POSTFIELDS, ['file' => $cFile]);
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
        curl_setopt($ch, CURLOPT_HTTPHEADER, [
            'Authorization: Bearer ' . $token,
        ]);
        $res = curl_exec($ch);
        $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
        curl_close($ch);

        $json = json_decode($res, true);
        if ($httpCode === 200 || !empty($json['message'])) {
            $this->setFlash('success', $json['message'] ?? 'تمت استعادة قاعدة البيانات بنجاح.');
        } else {
            $this->setFlash('error', $json['error'] ?? 'فشل استعادة قاعدة البيانات.');
        }

        $this->redirect('/settings');
    }

    public function importSAS4(): void
    {
        $this->requireSuperAdmin();
        $url = trim($_POST['url'] ?? '');
        $username = trim($_POST['username'] ?? '');
        $password = trim($_POST['password'] ?? '');

        if (!$url || !$username || !$password) {
            $this->setFlash('error', 'يرجى إدخال رابط سيرفر SAS4، واسم المستخدم، وكلمة المرور');
            $this->redirect('/settings');
            return;
        }

        $resp = $this->api->post('/import/sas4', [
            'url' => $url,
            'username' => $username,
            'password' => $password,
        ]);

        if (isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', $resp['message'] ?? 'تم استيراد كافة بيانات المشتركين والباقات من SAS4 بنجاح! 🚀');
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل الاتصال بسيرفر SAS4 واستيراد البيانات');
        }

        $this->redirect('/settings');
    }

    public function exportExcel(): void
    {
        $this->requireSuperAdmin();
        $token = $_SESSION['sasman_token'] ?? $_SESSION['token'] ?? '';
        $ch = curl_init('http://127.0.0.1:8080/radius/api/export/excel');
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
        curl_setopt($ch, CURLOPT_FOLLOWLOCATION, true);
        curl_setopt($ch, CURLOPT_HTTPHEADER, [
            'Authorization: Bearer ' . $token,
        ]);
        $response = curl_exec($ch);
        $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
        curl_close($ch);

        if ($httpCode === 200 && !empty($response)) {
            $filename = 'sasman-subscribers-' . date('Y-m-d') . '.xlsx';
            header('Content-Description: File Transfer');
            header('Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet');
            header('Content-Disposition: attachment; filename="' . $filename . '"');
            header('Expires: 0');
            header('Cache-Control: must-revalidate');
            header('Pragma: public');
            header('Content-Length: ' . strlen($response));
            echo $response;
            exit;
        }

        $this->setFlash('error', 'فشل تصدير ملف الإكسيل');
        $this->redirect('/settings');
    }

    public function importExcel(): void
    {
        $this->requireSuperAdmin();
        if (!isset($_FILES['file']) || $_FILES['file']['error'] !== UPLOAD_ERR_OK) {
            $this->setFlash('error', 'يرجى اختيار ملف إكسيل صالح (.xlsx)');
            $this->redirect('/settings');
            return;
        }

        $filePath = $_FILES['file']['tmp_name'];
        $fileName = $_FILES['file']['name'];
        $token = $_SESSION['sasman_token'] ?? $_SESSION['token'] ?? '';

        $cFile = curl_file_create($filePath, $_FILES['file']['type'], $fileName);
        $ch = curl_init('http://127.0.0.1:8080/radius/api/import/excel');
        curl_setopt($ch, CURLOPT_POST, true);
        curl_setopt($ch, CURLOPT_POSTFIELDS, ['file' => $cFile]);
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
        curl_setopt($ch, CURLOPT_HTTPHEADER, [
            'Authorization: Bearer ' . $token,
        ]);
        $res = curl_exec($ch);
        $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
        curl_close($ch);

        $json = json_decode($res, true);
        if ($httpCode === 200 || !empty($json['message'])) {
            $this->setFlash('success', $json['message'] ?? 'تم استيراد المشتركين من ملف الإكسيل بنجاح.');
        } else {
            $this->setFlash('error', $json['error'] ?? 'فشل استيراد ملف الإكسيل.');
        }

        $this->redirect('/settings');
    }
}
