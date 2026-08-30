<?php

namespace Sasman\Controllers;

use Sasman\Core\Controller;

class DevicesController extends Controller
{
    public function index(): void
    {
        $devicesResp = $this->api->get('/devices');
        $summaryResp = $this->api->get('/devices/summary');
        $vendorsResp = $this->api->get('/devices/vendors');
        $typesResp = $this->api->get('/devices/types');

        $devices = [];
        if (isset($devicesResp['devices']) && is_array($devicesResp['devices'])) {
            $devices = $devicesResp['devices'];
        } elseif (is_array($devicesResp)) {
            foreach ($devicesResp as $k => $v) {
                if (is_array($v) && (isset($v['ip']) || isset($v['name']))) {
                    $devices[] = $v;
                }
            }
        }

        $this->render('devices.index', [
            'title' => 'فحص أجهزة المشتركين والصحونات (CPE) — SASMAN',
            'devices' => $devices,
            'summary' => $summaryResp,
            'vendors' => is_array($vendorsResp) ? $vendorsResp : [],
            'types' => is_array($typesResp) ? $typesResp : [],
        ]);
    }

    public function create(): void
    {
        $name = trim($_POST['name'] ?? '');
        $vendorSlug = trim($_POST['vendor_slug'] ?? 'ubiquiti');
        $typeSlug = trim($_POST['type_slug'] ?? 'cpe');
        $ip = trim($_POST['ip'] ?? '');
        $port = (int)($_POST['port'] ?? 80);
        $username = trim($_POST['username'] ?? 'ubnt');
        $password = trim($_POST['password'] ?? 'ubnt');
        $authType = trim($_POST['auth_type'] ?? 'http_basic');
        $description = trim($_POST['description'] ?? '');

        if (!$ip) {
            $this->setFlash('error', 'يرجى إدخال عنوان IP للجهاز');
            $this->redirect('/devices');
            return;
        }

        $resp = $this->api->post('/devices', [
            'name' => $name ?: "Device-{$ip}",
            'vendor_slug' => $vendorSlug,
            'type_slug' => $typeSlug,
            'ip' => $ip,
            'port' => $port,
            'username' => $username,
            'password' => $password,
            'auth_type' => $authType,
            'description' => $description,
        ]);

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success']) || isset($resp['device'])) {
            $this->setFlash('success', "تمت إضافة الجهاز [{$ip}] بنجاح");
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل إضافة الجهاز');
        }

        $this->redirect('/devices');
    }

    public function delete(array $params): void
    {
        $id = $params['id'] ?? '';
        $resp = $this->api->delete("/devices/" . rawurlencode($id));

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', 'تم حذف الجهاز بنجاح');
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل حذف الجهاز');
        }

        $this->redirect('/devices');
    }

    public function poll(array $params): void
    {
        $id = $params['id'] ?? '';
        $resp = $this->api->post("/devices/{$id}/poll");

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', 'تم فحص وقراءة بيانات الجهاز لحظياً');
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل فحص الجهاز');
        }

        $this->redirect('/devices');
    }
}
