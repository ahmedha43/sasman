<?php

namespace Sasman\Controllers;

use Sasman\Core\Controller;

class LogsController extends Controller
{
    public function index(): void
    {
        $search = trim($_GET['search'] ?? '');
        $actionType = trim($_GET['action_type'] ?? '');
        
        $params = [];
        if ($search) $params['search'] = $search;
        if ($actionType) $params['action_type'] = $actionType;
        
        $queryStr = $params ? '?' . http_build_query($params) : '';
        $auditResp = $this->api->get('/audit-logs' . $queryStr);

        $logs = [];
        if (isset($auditResp['logs']) && is_array($auditResp['logs'])) {
            $logs = $auditResp['logs'];
        } elseif (is_array($auditResp)) {
            foreach ($auditResp as $k => $v) {
                if (is_array($v) && (isset($v['action_type']) || isset($v['details']))) {
                    $logs[] = $v;
                }
            }
        }

        $this->render('logs.index', [
            'title' => 'سجل النشاطات والرقابة (Audit Trail) — SASMAN',
            'logs' => $logs,
            'total' => $auditResp['total'] ?? count($logs),
            'search' => $search,
            'action_type' => $actionType,
        ]);
    }

    public function clear(): void
    {
        $resp = $this->api->delete('/audit-logs');

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $this->setFlash('success', 'تم مسح سجل النشاطات القديم بنجاح');
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل مسح السجلات');
        }

        $this->redirect('/logs');
    }
}
