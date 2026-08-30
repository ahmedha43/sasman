<?php

namespace Sasman\Core;

class ApiClient
{
    private string $baseUrl;
    private ?string $token = null;
    private array $config;

    public function __construct(array $config)
    {
        $this->config = $config;
        $this->baseUrl = $this->resolveBaseUrl();
        
        if (isset($_SESSION['sasman_token'])) {
            $this->token = $_SESSION['sasman_token'];
        }
    }

    private function resolveBaseUrl(): string
    {
        $host = $_SERVER['HTTP_HOST'] ?? '';
        $centralDomain = $this->config['central_domain'] ?? 'sas-man.net';

        // If running in cloud mode via subdomain (e.g., mur.sas-man.net)
        if ($host && str_ends_with(strtolower($host), '.' . strtolower($centralDomain))) {
            $scheme = (isset($_SERVER['HTTPS']) && $_SERVER['HTTPS'] === 'on') ? 'https' : 'http';
            return "{$scheme}://{$host}/radius/api";
        }

        return $this->config['backend_api_url'] ?? 'http://127.0.0.1:8080/radius/api';
    }

    public function setToken(string $token): void
    {
        $this->token = $token;
        $_SESSION['sasman_token'] = $token;
    }

    public function clearToken(): void
    {
        $this->token = null;
        unset($_SESSION['sasman_token'], $_SESSION['sasman_admin']);
    }

    public function request(string $method, string $endpoint, array $data = [], array $headers = []): array
    {
        $url = rtrim($this->baseUrl, '/') . '/' . ltrim($endpoint, '/');
        
        $ch = curl_init();
        
        $defaultHeaders = [
            'Accept: application/json',
            'User-Agent: SASMAN-PHP-Frontend/5.2.0'
        ];

        // Pass host header to support multi-tenant subdomain resolution
        if (isset($_SERVER['HTTP_HOST'])) {
            $defaultHeaders[] = 'Host: ' . $_SERVER['HTTP_HOST'];
        }

        if ($this->token) {
            $defaultHeaders[] = 'Authorization: Bearer ' . $this->token;
        }

        $allHeaders = array_merge($defaultHeaders, $headers);

        $method = strtoupper($method);
        curl_setopt($ch, CURLOPT_CUSTOMREQUEST, $method);
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
        curl_setopt($ch, CURLOPT_TIMEOUT, 15);
        curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, false);
        curl_setopt($ch, CURLOPT_SSL_VERIFYHOST, false);

        if (in_array($method, ['POST', 'PUT', 'PATCH', 'DELETE'])) {
            if (!empty($data)) {
                $payload = json_encode($data);
                curl_setopt($ch, CURLOPT_POSTFIELDS, $payload);
                $allHeaders[] = 'Content-Type: application/json';
            }
        } elseif ($method === 'GET' && !empty($data)) {
            $url .= '?' . http_build_query($data);
        }

        curl_setopt($ch, CURLOPT_URL, $url);
        curl_setopt($ch, CURLOPT_HTTPHEADER, $allHeaders);

        $response = curl_exec($ch);
        $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
        $curlError = curl_error($ch);
        curl_close($ch);

        if ($curlError) {
            return [
                'success' => false,
                'error' => 'فشل الاتصال بمحرك SASMAN Core: ' . $curlError,
                'http_code' => 503
            ];
        }

        $decoded = json_decode($response, true);
        if ($decoded === null && $httpCode >= 400) {
            return [
                'success' => false,
                'error' => 'خطأ في الخادم (HTTP ' . $httpCode . ')',
                'http_code' => $httpCode,
                'raw' => $response
            ];
        }

        if (is_array($decoded)) {
            if (array_is_list($decoded)) {
                return $decoded;
            }
            $decoded['http_code'] = $httpCode;
            return $decoded;
        }

        return [
            'success' => $httpCode >= 200 && $httpCode < 300,
            'data' => $decoded,
            'http_code' => $httpCode,
            'raw' => $response
        ];
    }

    public function get(string $endpoint, array $params = []): array
    {
        return $this->request('GET', $endpoint, $params);
    }

    public function post(string $endpoint, array $data = []): array
    {
        return $this->request('POST', $endpoint, $data);
    }

    public function put(string $endpoint, array $data = []): array
    {
        return $this->request('PUT', $endpoint, $data);
    }

    public function delete(string $endpoint, array $data = []): array
    {
        return $this->request('DELETE', $endpoint, $data);
    }
}
