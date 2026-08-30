<?php

namespace Sasman\Middleware;

use Sasman\Core\ApiClient;

class AuthMiddleware
{
    private ApiClient $api;
    private array $config;

    public function __construct(ApiClient $api, array $config)
    {
        $this->api = $api;
        $this->config = $config;
    }

    public function handle(): bool
    {
        if (!isset($_SESSION['sasman_token']) || empty($_SESSION['sasman_token'])) {
            header('Location: /login');
            exit;
        }

        return true;
    }
}
