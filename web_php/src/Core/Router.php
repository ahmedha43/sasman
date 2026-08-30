<?php

namespace Sasman\Core;

class Router
{
    private array $routes = [];
    private ApiClient $api;
    private View $view;
    private array $config;

    public function __construct(ApiClient $api, View $view, array $config)
    {
        $this->api = $api;
        $this->view = $view;
        $this->config = $config;
    }

    public function get(string $path, string $handler, array $middlewares = []): void
    {
        $this->addRoute('GET', $path, $handler, $middlewares);
    }

    public function post(string $path, string $handler, array $middlewares = []): void
    {
        $this->addRoute('POST', $path, $handler, $middlewares);
    }

    public function any(string $path, string $handler, array $middlewares = []): void
    {
        $this->addRoute('ANY', $path, $handler, $middlewares);
    }

    private function addRoute(string $method, string $path, string $handler, array $middlewares): void
    {
        $this->routes[] = [
            'method' => $method,
            'path' => $path,
            'handler' => $handler,
            'middlewares' => $middlewares,
        ];
    }

    public function dispatch(): void
    {
        $uri = parse_url($_SERVER['REQUEST_URI'] ?? '/', PHP_URL_PATH);
        $method = $_SERVER['REQUEST_METHOD'] ?? 'GET';

        // Normalize URI
        $uri = '/' . trim($uri, '/');
        if ($uri === '//') $uri = '/';

        foreach ($this->routes as $route) {
            if ($route['method'] !== 'ANY' && $route['method'] !== $method) {
                continue;
            }

            // Convert route pattern to regex (e.g. /users/:id -> /users/(?P<id>[^/]+))
            $pattern = preg_replace('/\:([a-zA-Z0-9_]+)/', '(?P<$1>[^/]+)', $route['path']);
            $pattern = '#^' . $pattern . '$#';

            if (preg_match($pattern, $uri, $matches)) {
                // Execute Middlewares
                foreach ($route['middlewares'] as $middlewareClass) {
                    $middleware = new $middlewareClass($this->api, $this->config);
                    if (!$middleware->handle()) {
                        return; // Stopped by middleware
                    }
                }

                // Extract params
                $params = [];
                foreach ($matches as $key => $val) {
                    if (is_string($key)) {
                        $params[$key] = $val;
                    }
                }

                // Dispatch to controller
                [$controllerName, $action] = explode('@', $route['handler']);
                $fullControllerClass = "Sasman\\Controllers\\{$controllerName}";

                if (!class_exists($fullControllerClass)) {
                    http_response_code(500);
                    echo "Controller not found: {$fullControllerClass}";
                    return;
                }

                $controller = new $fullControllerClass($this->api, $this->view, $this->config);
                if (!method_exists($controller, $action)) {
                    http_response_code(500);
                    echo "Action not found: {$action}";
                    return;
                }

                $controller->$action($params);
                return;
            }
        }

        // 404 Not Found
        http_response_code(404);
        $this->view->render('errors.404', ['title' => 'الصفحة غير موجودة'], 'auth');
    }
}
