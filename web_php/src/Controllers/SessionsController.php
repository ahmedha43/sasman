<?php

namespace Sasman\Controllers;

use Sasman\Core\Controller;

class SessionsController extends Controller
{
    public function index(): void
    {
        $sessionsResp = $this->api->get('/sessions');
        $sessions = $sessionsResp['sessions'] ?? $sessionsResp['online_users'] ?? [];

        $this->render('sessions.index', [
            'title' => 'المشتركون المتصلون حالياً (Active Sessions) — SASMAN',
            'sessions' => $sessions,
        ]);
    }

    public function disconnect(array $params): void
    {
        $username = $params['username'] ?? '';
        $resp = $this->api->post("/sessions/disconnect", ['username' => $username]);

        if (isset($resp['success']) && $resp['success']) {
            $this->setFlash('success', "تم فصل المستخدم [{$username}] فوراً من المايكروتك");
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل فصل المستخدم');
        }

        $this->redirect('/sessions');
    }
}
