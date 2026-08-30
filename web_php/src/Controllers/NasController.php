<?php

namespace Sasman\Controllers;

use Sasman\Core\Controller;

class NasController extends Controller
{
    public function index(): void
    {
        $statusResp = $this->api->get('/nas/status');
        $codeResp = $this->api->get('/nas/provision-code');
        $nasListResp = $this->api->get('/nas');

        $routers = [];
        if (isset($nasListResp['nas']) && is_array($nasListResp['nas'])) {
            $routers = $nasListResp['nas'];
        } elseif (is_array($nasListResp)) {
            foreach ($nasListResp as $k => $v) {
                if (is_array($v) && (isset($v['ip']) || isset($v['nasname']) || isset($v['name']) || isset($v['shortname']))) {
                    $routers[] = $v;
                }
            }
        }

        $provisionCode = $codeResp['command'] ?? $codeResp['code'] ?? $codeResp['script_url'] ?? '';

        $this->render('nas.index', [
            'title' => 'راوترات المايكروتك و RadSec — SASMAN',
            'nas' => $statusResp,
            'provision_code' => $provisionCode,
            'routers' => $routers,
        ]);
    }

    public function create(): void
    {
        if (!$this->canManageNas()) {
            $this->setFlash('error', '🚫 ليس لديك صلاحية لإضافة راوترات المايكروتك أو تعديلها');
            $this->redirect('/nas');
            return;
        }

        $nasname = trim($_POST['nasname'] ?? '');
        $shortname = trim($_POST['shortname'] ?? '');
        $secret = trim($_POST['secret'] ?? '');
        $type = trim($_POST['type'] ?? 'other');
        $description = trim($_POST['description'] ?? '');

        if (!$nasname || !$secret) {
            $this->setFlash('error', 'يرجى إدخال عنوان IP وسر المايكروتك (Secret)');
            $this->redirect('/nas');
            return;
        }

        $resp = $this->api->post('/nas', [
            'nasname' => $nasname,
            'shortname' => $shortname ?: $nasname,
            'secret' => $secret,
            'type' => $type,
            'description' => $description,
        ]);

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', "تمت إضافة راوتر المايكروتك [{$nasname}] بنجاح");
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل إضافة الراوتر');
        }

        $this->redirect('/nas');
    }

    public function delete(array $params): void
    {
        if (!$this->canManageNas()) {
            $this->setFlash('error', '🚫 ليس لديك صلاحية لحذف راوترات المايكروتك');
            $this->redirect('/nas');
            return;
        }

        $ip = $params['ip'] ?? '';
        $resp = $this->api->delete("/nas/" . rawurlencode($ip));

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', "تم حذف الراوتر [{$ip}] بنجاح");
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل حذف الراوتر');
        }

        $this->redirect('/nas');
    }

    public function quickSetup(): void
    {
        $resp = $this->api->post('/nas/quick-setup');

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', 'تم تطبيق الإعداد السريع للمايكروتك وراديوس بنجاح');
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل الإعداد السريع');
        }

        $this->redirect('/nas');
    }

    public function generateCert(array $params): void
    {
        $id = $params['id'] ?? '';
        $resp = $this->api->post("/nas/{$id}/generate-cert");

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', 'تم توليد شهادة التشفير RadSec mTLS بنجاح');
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل توليد الشهادة');
        }

        $this->redirect('/nas');
    }
}
