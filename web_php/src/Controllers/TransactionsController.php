<?php

namespace Sasman\Controllers;

use Sasman\Core\Controller;

class TransactionsController extends Controller
{
    public function index(): void
    {
        $transResp = $this->api->get('/transactions');
        $usersResp = $this->api->get('/users');

        $transactions = $transResp['transactions'] ?? [];
        $userSummaries = $transResp['user_summaries'] ?? [];
        $globalPayments = (float)($transResp['global_payments'] ?? 0);
        $globalDebts = (float)($transResp['global_debts'] ?? 0);

        $users = [];
        if (isset($usersResp['users']) && is_array($usersResp['users'])) {
            $users = $usersResp['users'];
        } elseif (is_array($usersResp)) {
            foreach ($usersResp as $k => $v) {
                if (is_array($v) && (isset($v['user']) || isset($v['username']))) {
                    $users[] = $v;
                }
            }
        }

        // If user_summaries was not returned by older backend, compute on PHP side
        if (empty($userSummaries) && !empty($users)) {
            $userTransMap = [];
            foreach ($transactions as $t) {
                $u = $t['username'] ?? '';
                $userTransMap[$u][] = $t;
            }

            foreach ($users as $u) {
                $uName = $u['user'] ?? $u['username'] ?? '';
                $bal = (float)($u['balance'] ?? 0);
                $uTrans = $userTransMap[$uName] ?? [];
                
                $uPaid = 0;
                $uDebt = 0;
                foreach ($uTrans as $t) {
                    $amt = (float)($t['amount'] ?? 0);
                    $type = $t['type'] ?? 'payment';
                    if ($type === 'payment' || $type === 'تجديد اشتراك') {
                        $uPaid += $amt;
                    } else {
                        $uDebt += $amt;
                    }
                }

                $userSummaries[] = [
                    'username' => $uName,
                    'full_name' => $u['full_name'] ?? '',
                    'phone' => $u['phone'] ?? '',
                    'balance' => $bal,
                    'total_paid' => $uPaid,
                    'total_debt' => $uDebt,
                    'transactions' => $uTrans,
                ];
            }
        }

        $this->render('transactions.index', [
            'title' => 'السجل المالي وحسابات المشتركين — SASMAN',
            'transactions' => $transactions,
            'user_summaries' => $userSummaries,
            'global_payments' => $globalPayments,
            'global_debts' => $globalDebts,
            'users' => $users,
        ]);
    }

    public function create(): void
    {
        $username = trim($_POST['username'] ?? '');
        $type = trim($_POST['type'] ?? 'payment');
        $amount = (float)($_POST['amount'] ?? 0);
        $notes = trim($_POST['notes'] ?? '');

        if (!$username || $amount <= 0) {
            $this->setFlash('error', 'يرجى تحديد المشترك وإدخال مبلغ صحيح');
            $this->redirect('/transactions');
            return;
        }

        $resp = $this->api->post('/transactions', [
            'username' => $username,
            'type' => $type,
            'amount' => $amount,
            'notes' => $notes,
        ]);

        if (($resp['http_code'] ?? 500) < 300 || isset($resp['message']) || !empty($resp['success'])) {
            $typeText = ($type === 'payment') ? 'تسديد دفعة' : 'تسجيل دين';
            $this->setFlash('success', "تم تسجيل {$typeText} للمشترك [{$username}] بمبلغ " . number_format($amount, 0) . " د.ع بنجاح");
        } else {
            $this->setFlash('error', $resp['error'] ?? 'فشل تسجيل الحركة المالية');
        }

        $this->redirect('/transactions');
    }
}
