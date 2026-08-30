<?php

namespace Sasman\Controllers;

use Sasman\Core\Controller;

class StreamsController extends Controller
{
    public function index(): void
    {
        $streamsResp = $this->api->get('/streams');

        $streams = [];
        if (isset($streamsResp['streams']) && is_array($streamsResp['streams'])) {
            $streams = $streamsResp['streams'];
        } elseif (is_array($streamsResp)) {
            foreach ($streamsResp as $k => $v) {
                if (is_array($v) && (isset($v['id']) || isset($v['name']))) {
                    $streams[] = $v;
                }
            }
        }

        $this->render('streams.index', [
            'title' => 'قنوات البث المباشر (IPTV Live) — SASMAN',
            'streams' => $streams,
        ]);
    }

    public function create(): void
    {
        $id = trim($_POST['id'] ?? '');
        $name = trim($_POST['name'] ?? '');
        $source = trim($_POST['source'] ?? $_POST['url'] ?? '');
        $status = trim($_POST['status'] ?? 'active');

        if (!$name || !$source) {
            $this->setFlash('error', 'يرجى إدخال اسم القناة ورابط مصدر البث (Source URL)');
            $this->redirect('/streams');
            return;
        }

        if (!$id) {
            $id = preg_replace('/[^a-z0-9_-]/', '', strtolower($name));
            if (!$id) {
                $id = 'channel_' . time();
            }
        }

        $resp = $this->api->post('/streams', [
            'id' => $id,
            'name' => $name,
            'source' => $source,
            'status' => $status,
        ]);

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success']) || isset($resp['stream'])) {
            $this->setFlash('success', "تمت إضافة قناة [{$name}] بنجاح");
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل إضافة القناة');
        }

        $this->redirect('/streams');
    }

    public function delete(array $params): void
    {
        $id = $params['id'] ?? '';
        $resp = $this->api->delete("/streams/" . rawurlencode($id));

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', 'تم حذف القناة بنجاح');
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل حذف القناة');
        }

        $this->redirect('/streams');
    }
}
