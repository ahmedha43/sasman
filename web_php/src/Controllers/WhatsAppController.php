<?php

namespace Sasman\Controllers;

use Sasman\Core\Controller;

class WhatsAppController extends Controller
{
    public function index(): void
    {
        $configResp = $this->api->get('/whatsapp/config');
        $qrResp = $this->api->get('/whatsapp/qr');
        $templatesResp = $this->api->get('/whatsapp/templates');

        $templates = [];
        if (isset($templatesResp['templates']) && is_array($templatesResp['templates'])) {
            $templates = $templatesResp['templates'];
        } elseif (is_array($templatesResp)) {
            foreach ($templatesResp as $k => $v) {
                if (is_array($v) && (isset($v['template_key']) || isset($v['_template_key']))) {
                    $templates[] = [
                        'key' => $v['template_key'] ?? $v['_template_key'] ?? '',
                        'text' => $v['template_text'] ?? '',
                    ];
                }
            }
        }

        // Key-value map of templates
        $templateMap = [];
        foreach ($templates as $t) {
            $templateMap[$t['key']] = $t['text'];
        }

        $this->render('whatsapp.index', [
            'title' => 'إعدادات الواتساب وقوالب الرسائل — SASMAN',
            'config' => $configResp,
            'qr' => $qrResp,
            'templates' => $templateMap,
        ]);
    }

    public function saveConfig(): void
    {
        $enabled = isset($_POST['enabled']) ? 1 : 0;
        $reminderEnabled = isset($_POST['reminder_enabled']) ? 1 : 0;
        $reminderHours = (int)($_POST['reminder_hours'] ?? 48);

        $resp = $this->api->post('/whatsapp/config', [
            'enabled' => $enabled,
            'reminder_enabled' => $reminderEnabled,
            'reminder_hours' => $reminderHours,
        ]);

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', 'تم حفظ إعدادات الواتساب بنجاح');
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل حفظ الإعدادات');
        }

        $this->redirect('/whatsapp');
    }

    public function saveTemplate(): void
    {
        $key = trim($_POST['template_key'] ?? '');
        $text = trim($_POST['template_text'] ?? '');

        if (!$key || !$text) {
            $this->setFlash('error', 'يرجى إدخال مفتاح القالب ونص الرسالة');
            $this->redirect('/whatsapp');
            return;
        }

        $resp = $this->api->post('/whatsapp/templates', [
            'template_key' => $key,
            'template_text' => $text,
        ]);

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', "تم حفظ قالب الرسالة [{$key}] بنجاح");
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل حفظ القالب');
        }

        $this->redirect('/whatsapp');
    }

    public function test(): void
    {
        $phone = trim($_POST['phone'] ?? '');
        $message = trim($_POST['message'] ?? 'رسالة تجريبية من منظومة SASMAN');

        if (!$phone) {
            $this->setFlash('error', 'يرجى إدخال رقم هاتف المستلم');
            $this->redirect('/whatsapp');
            return;
        }

        $resp = $this->api->post('/whatsapp/test', [
            'phone' => $phone,
            'message' => $message,
        ]);

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', "تم إرسال الرسالة التجريبية إلى [{$phone}] بنجاح");
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل إرسال الرسالة التجريبية');
        }

        $this->redirect('/whatsapp');
    }

    public function sendDebtReminders(): void
    {
        $resp = $this->api->post('/whatsapp/send-debt-reminder');

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', 'تم بدء إرسال تذكيرات الديون لكافة المشتركين المدينين');
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل إرسال تذكيرات الديون');
        }

        $this->redirect('/whatsapp');
    }

    public function logout(): void
    {
        $resp = $this->api->post('/whatsapp/logout');
        $this->setFlash('success', 'تم فصل جلسة الواتساب بنجاح');
        $this->redirect('/whatsapp');
    }
}
