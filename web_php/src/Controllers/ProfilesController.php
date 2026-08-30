<?php

namespace Sasman\Controllers;

use Sasman\Core\Controller;

class ProfilesController extends Controller
{
    public function index(): void
    {
        $resp = $this->api->get('/profiles');
        
        $profiles = [];
        if (isset($resp['profiles']) && is_array($resp['profiles'])) {
            $profiles = $resp['profiles'];
        } elseif (is_array($resp)) {
            foreach ($resp as $k => $v) {
                if (is_array($v) && isset($v['name'])) {
                    $profiles[] = $v;
                }
            }
        }

        $totalProfiles = count($profiles);
        $totalPrice = 0;
        $totalValidity = 0;
        $totalDownload = 0;

        foreach ($profiles as $p) {
            $totalPrice += (float)($p['price'] ?? 0);
            $totalValidity += (int)($p['validity_days'] ?? 30);
            
            $limitStr = $p['limit'] ?? $p['rate_limit'] ?? '';
            $dl = 0;
            if ($limitStr) {
                $parts = explode('/', $limitStr);
                if (count($parts) === 2) {
                    $dl = (int)preg_replace('/[^0-9]/', '', $parts[1]);
                }
            }
            $totalDownload += $dl;
        }

        $avgPrice = ($totalProfiles > 0) ? round($totalPrice / $totalProfiles) : 0;
        $avgValidity = ($totalProfiles > 0) ? round($totalValidity / $totalProfiles) : 30;
        $avgDownload = ($totalProfiles > 0) ? round($totalDownload / $totalProfiles) : 0;

        $this->render('profiles.index', [
            'title' => 'باقات السرعة والاشتراكات — SASMAN',
            'profiles' => $profiles,
            'total_profiles' => $totalProfiles,
            'avg_price' => $avgPrice,
            'avg_validity' => $avgValidity,
            'avg_download' => $avgDownload,
        ]);
    }

    public function create(): void
    {
        if (!$this->canManageProfiles()) {
            $this->setFlash('error', '🚫 ليس لديك صلاحية لإضافة أو تعديل باقات السرعة');
            $this->redirect('/profiles');
            return;
        }

        $name = trim($_POST['name'] ?? '');
        $originalName = trim($_POST['original_name'] ?? '');
        $download = trim($_POST['download'] ?? '');
        $upload = trim($_POST['upload'] ?? '');
        $price = (float)($_POST['price'] ?? 0);
        $agentPrice = (float)($_POST['agent_price'] ?? $price);
        $validity = (string)($_POST['validity'] ?? $_POST['duration_days'] ?? '30');
        $pool = trim($_POST['pool'] ?? '');
        $mikrotikGroup = trim($_POST['mikrotik_group'] ?? '');
        $simultaneous = trim($_POST['simultaneous'] ?? '1');

        // If rate_limit was passed (e.g. 10M/30M)
        $rateLimit = trim($_POST['rate_limit'] ?? '');
        if ($rateLimit && (!$download || !$upload)) {
            $parts = explode('/', $rateLimit);
            if (count($parts) === 2) {
                $upload = str_ireplace(['m', 'k', 'g', ' '], '', trim($parts[0]));
                $download = str_ireplace(['m', 'k', 'g', ' '], '', trim($parts[1]));
            } else {
                $download = str_ireplace(['m', 'k', 'g', ' '], '', $rateLimit);
                $upload = $download;
            }
        }

        if (!$name || (!$download && !$upload)) {
            $this->setFlash('error', 'يرجى إدخال اسم الباقة وسرعة التحميل والرفع');
            $this->redirect('/profiles');
            return;
        }

        $payload = [
            'name' => $name,
            'download' => (int)$download,
            'upload' => (int)$upload,
            'price' => $price,
            'agent_price' => $agentPrice,
            'validity_days' => (int)$validity,
            'pool' => $pool,
            'mikrotik_group' => $mikrotikGroup,
            'simultaneous' => (int)$simultaneous,
        ];

        // If editing an existing profile and name changed, handle rename
        if ($originalName && $originalName !== $name) {
            $this->api->delete("/profiles/{$originalName}");
        }

        $resp = $this->api->post('/profiles', $payload);

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', $originalName ? "تم تحديث الباقة [{$name}] بنجاح" : "تم إنشاء الباقة [{$name}] بنجاح");
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل حفظ الباقة');
        }

        $this->redirect('/profiles');
    }

    public function delete(array $params): void
    {
        if (!$this->canManageProfiles()) {
            $this->setFlash('error', '🚫 ليس لديك صلاحية لحذف باقات السرعة');
            $this->redirect('/profiles');
            return;
        }

        $name = $params['name'] ?? '';
        $resp = $this->api->delete("/profiles/{$name}");

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', "تم حذف الباقة [{$name}] بنجاح");
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل حذف الباقة');
        }

        $this->redirect('/profiles');
    }
}
