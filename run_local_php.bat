@echo off
title SASMAN PHP Frontend Server (:9000)
echo ===================================================
echo   SASMAN Modern PHP Frontend Layer v5.2.0
echo ===================================================
echo.
echo [1/2] Connecting to SASMAN Go Core API at http://127.0.0.1:8080 ...
echo [2/2] Starting PHP Web Server on http://127.0.0.1:9000 ...
echo.
echo ===================================================
echo   Dashboard URL:  http://localhost:9000/
echo   Portal URL:     http://localhost:9000/portal
echo ===================================================
echo.

php -S 127.0.0.1:9000 -t web_php/public
