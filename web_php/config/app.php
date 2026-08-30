<?php

// Configuration for SASMAN PHP Frontend Layer
return [
    'app_name' => 'SASMAN Platform',
    'app_version' => '5.2.0',
    'environment' => getenv('APP_ENV') ?: 'production', // 'development' or 'production'
    
    // Core Go Backend API Base URL
    // In local mode: http://127.0.0.1:8080
    // In cloud mode: uses host header subdomain e.g. https://mur.sas-man.net
    'backend_api_url' => getenv('SASMAN_API_URL') ?: 'http://127.0.0.1:8080/radius/api',
    'central_domain' => getenv('SASMAN_CENTRAL_DOMAIN') ?: 'sas-man.net',
    
    // Session lifetime in seconds (12 hours)
    'session_lifetime' => 43200,
    
    // Supported languages
    'default_locale' => 'ar',
    'direction' => 'rtl',
];
