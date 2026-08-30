<?php

namespace Sasman\Core;

abstract class Controller
{
    protected ApiClient $api;
    protected View $view;
    protected array $config;

    public function __construct(ApiClient $api, View $view, array $config)
    {
        $this->api = $api;
        $this->view = $view;
        $this->config = $config;
    }

    protected function isSuperAdmin(): bool
    {
        $role = strtolower($_SESSION['sasman_admin']['role'] ?? '');
        $parentId = $_SESSION['sasman_admin']['parent_id'] ?? null;
        return ($role === 'superadmin' || ($role === 'admin' && empty($parentId)));
    }

    protected function isAgent(): bool
    {
        return !$this->isSuperAdmin();
    }

    protected function hasPermission(string $perm): bool
    {
        if ($this->isSuperAdmin()) {
            return true;
        }

        $admin = $_SESSION['sasman_admin'] ?? [];

        // Check direct boolean key on admin object
        if (isset($admin[$perm])) {
            return !empty($admin[$perm]);
        }

        // Check inside permissions JSON/array
        if (isset($admin['permissions'])) {
            $perms = is_array($admin['permissions']) ? $admin['permissions'] : json_decode($admin['permissions'], true);
            if (is_array($perms) && isset($perms[$perm])) {
                return !empty($perms[$perm]);
            }
        }

        // Default permissions for agents if unspecified
        $defaults = [
            'can_create_users' => true,
            'can_edit_users' => true,
            'can_delete_users' => false,
            'can_toggle_users' => true,
            'can_disconnect_users' => true,
            'can_renew_users' => true,
            'can_generate_vouchers' => true,
            'can_delete_vouchers' => false,
            'can_print_vouchers' => true,
            'can_manage_profiles' => false,
            'can_manage_nas' => false,
            'can_manage_devices' => false,
            'can_manage_transactions' => true,
            'can_manage_subagents' => false,
            'can_view_logs' => true,
            'can_clear_logs' => false,
            'can_manage_whatsapp' => false,
            'can_manage_streams' => false,
        ];

        return $defaults[$perm] ?? false;
    }

    protected function requirePermission(string $perm, string $errorMsg = ''): void
    {
        if (!$this->hasPermission($perm)) {
            $msg = $errorMsg ?: '🚫 ليس لديك صلاحية كافية لتنفيذ هذا الإجراء (' . $perm . ')';
            $this->setFlash('error', $msg);
            $this->redirect('/');
        }
    }

    protected function canManageProfiles(): bool
    {
        return $this->hasPermission('can_manage_profiles');
    }

    protected function canManageNas(): bool
    {
        return $this->hasPermission('can_manage_nas');
    }

    protected function requireSuperAdmin(): void
    {
        if (!$this->isSuperAdmin()) {
            $this->setFlash('error', '🚫 عذراً، هذه الصفحة مخصصة للإدارة العامة للمنظومة (Superadmin) فقط');
            $this->redirect('/');
        }
    }

    protected function render(string $view, array $data = [], string $layout = 'main'): void
    {
        // Automatically refresh admin profile, permissions & balance if logged in
        if (isset($_SESSION['sasman_token']) && !empty($_SESSION['sasman_admin'])) {
            $me = $this->api->get('/auth/me');
            if (isset($me['id'])) {
                $_SESSION['sasman_admin'] = array_merge($_SESSION['sasman_admin'], $me);
            }
        }

        $admin = $_SESSION['sasman_admin'] ?? [];
        $permissions = [
            'can_create_users' => $this->hasPermission('can_create_users'),
            'can_edit_users' => $this->hasPermission('can_edit_users'),
            'can_delete_users' => $this->hasPermission('can_delete_users'),
            'can_toggle_users' => $this->hasPermission('can_toggle_users'),
            'can_disconnect_users' => $this->hasPermission('can_disconnect_users'),
            'can_renew_users' => $this->hasPermission('can_renew_users'),
            'can_generate_vouchers' => $this->hasPermission('can_generate_vouchers'),
            'can_delete_vouchers' => $this->hasPermission('can_delete_vouchers'),
            'can_print_vouchers' => $this->hasPermission('can_print_vouchers'),
            'can_manage_profiles' => $this->hasPermission('can_manage_profiles'),
            'can_manage_nas' => $this->hasPermission('can_manage_nas'),
            'can_manage_devices' => $this->hasPermission('can_manage_devices'),
            'can_manage_transactions' => $this->hasPermission('can_manage_transactions'),
            'can_manage_subagents' => $this->hasPermission('can_manage_subagents'),
            'can_view_logs' => $this->hasPermission('can_view_logs'),
            'can_clear_logs' => $this->hasPermission('can_clear_logs'),
            'can_manage_whatsapp' => $this->hasPermission('can_manage_whatsapp'),
            'can_manage_streams' => $this->hasPermission('can_manage_streams'),
        ];

        $commonData = [
            'app_name' => $this->config['app_name'] ?? 'SASMAN Platform',
            'app_version' => $this->config['app_version'] ?? '5.2.0',
            'current_user' => $admin,
            'is_superadmin' => $this->isSuperAdmin(),
            'is_agent' => $this->isAgent(),
            'perms' => $permissions,
            'can_manage_profiles' => $this->canManageProfiles(),
            'can_manage_nas' => $this->canManageNas(),
            'is_logged_in' => isset($_SESSION['sasman_token']),
            'flash_success' => $_SESSION['flash_success'] ?? null,
            'flash_error' => $_SESSION['flash_error'] ?? null,
        ];
        
        unset($_SESSION['flash_success'], $_SESSION['flash_error']);

        $this->view->render($view, array_merge($commonData, $data), $layout);
    }

    protected function json(array $data, int $statusCode = 200): void
    {
        http_response_code($statusCode);
        header('Content-Type: application/json; charset=utf-8');
        echo json_encode($data, JSON_UNESCAPED_UNICODE);
        exit;
    }

    protected function redirect(string $url): void
    {
        header("Location: {$url}");
        exit;
    }

    protected function setFlash(string $type, string $message): void
    {
        $_SESSION['flash_' . $type] = $message;
    }
}
