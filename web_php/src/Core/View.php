<?php

namespace Sasman\Core;

class View
{
    private string $viewsPath;
    private array $data = [];

    public function __construct(string $viewsPath)
    {
        $this->viewsPath = rtrim($viewsPath, '/\\');
    }

    public function render(string $view, array $data = [], string $layout = 'main'): void
    {
        $this->data = array_merge($this->data, $data);
        extract($this->data);

        $viewFile = $this->viewsPath . '/' . str_replace('.', '/', $view) . '.php';
        if (!file_exists($viewFile)) {
            throw new \Exception("View file not found: {$viewFile}");
        }

        // Buffer the view content
        ob_start();
        include $viewFile;
        $content = ob_get_clean();

        if ($layout) {
            $layoutFile = $this->viewsPath . '/layouts/' . $layout . '.php';
            if (file_exists($layoutFile)) {
                include $layoutFile;
                return;
            }
        }

        echo $content;
    }

    public function partial(string $partial, array $data = []): void
    {
        $mergedData = array_merge($this->data, $data);
        extract($mergedData);

        $partialFile = $this->viewsPath . '/partials/' . str_replace('.', '/', $partial) . '.php';
        if (file_exists($partialFile)) {
            include $partialFile;
        }
    }
}
