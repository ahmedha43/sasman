<?php

namespace Sasman\Controllers;

use Sasman\Core\Controller;

class VouchersController extends Controller
{
    public function index(): void
    {
        $vouchersResp = $this->api->get('/vouchers');
        $profilesResp = $this->api->get('/profiles');

        $vouchers = [];
        if (isset($vouchersResp['vouchers']) && is_array($vouchersResp['vouchers'])) {
            $vouchers = $vouchersResp['vouchers'];
        } elseif (is_array($vouchersResp)) {
            foreach ($vouchersResp as $k => $v) {
                if (is_array($v) && (isset($v['code']) || isset($v['username']))) {
                    $vouchers[] = $v;
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

        $totalVouchers = count($vouchers);
        $unusedCount = 0;
        $usedCount = 0;
        $totalWorth = 0;

        foreach ($vouchers as $v) {
            $isUsed = !empty($v['is_used']) || (!empty($v['status']) && $v['status'] !== 'active');
            if ($isUsed) {
                $usedCount++;
            } else {
                $unusedCount++;
                $totalWorth += (float)($v['price'] ?? 0);
            }
        }

        $this->render('vouchers.index', [
            'title' => 'أستوديو كروت الشحن والطباعة — SASMAN',
            'vouchers' => $vouchers,
            'profiles' => $profiles,
            'total_vouchers' => $totalVouchers,
            'unused_count' => $unusedCount,
            'used_count' => $usedCount,
            'total_worth' => $totalWorth,
        ]);
    }

    public function generate(): void
    {
        $this->requirePermission('can_generate_vouchers', '🚫 ليس لديك صلاحية لتوليد كروت الشحن');

        $count = (int)($_POST['count'] ?? 10);
        $profileName = trim($_POST['profile_name'] ?? $_POST['profile'] ?? '');
        $length = (int)($_POST['code_length'] ?? $_POST['length'] ?? 10);
        $price = (float)($_POST['price'] ?? 0);
        $codeType = trim($_POST['code_type'] ?? 'numeric');

        if (!$profileName || $count <= 0) {
            $this->setFlash('error', 'يرجى تحديد الباقة وعدد الكروت');
            $this->redirect('/vouchers');
            return;
        }

        $payload = [
            'profile_name' => $profileName,
            'count' => $count,
            'price' => $price,
            'code_length' => $length,
            'code_type' => $codeType,
        ];

        $resp = $this->api->post('/vouchers/generate', $payload);

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', "تم توليد [{$count}] كرت شحن بنجاح");
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل توليد الكروت');
        }

        $this->redirect('/vouchers');
    }

    public function delete(array $params): void
    {
        $this->requirePermission('can_delete_vouchers', '🚫 ليس لديك صلاحية لحذف كروت الشحن');

        $id = $params['id'] ?? '';
        $resp = $this->api->delete("/vouchers/{$id}");

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', "تم حذف كرت الشحن بنجاح");
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل حذف كرت الشحن');
        }

        $this->redirect('/vouchers');
    }

    public function clearAll(): void
    {
        $this->requirePermission('can_delete_vouchers', '🚫 ليس لديك صلاحية لمسح وتفريغ كروت الشحن');

        $resp = $this->api->post('/vouchers/clear');

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', 'تم مسح كافة الكروت غير المستخدمة بنجاح');
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل مسح الكروت');
        }

        $this->redirect('/vouchers');
    }
}
