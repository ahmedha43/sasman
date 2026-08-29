import 'package:flutter/material.dart';

import '../../config/app_theme.dart';
import '../../services/api_service.dart';
import '../../services/storage_service.dart';
import '../../widgets/gradient_background.dart';
import '../../widgets/notification_helper.dart';
import 'login_screen.dart';

class ServerSetupScreen extends StatefulWidget {
  const ServerSetupScreen({super.key});

  @override
  State<ServerSetupScreen> createState() => _ServerSetupScreenState();
}

class _ServerSetupScreenState extends State<ServerSetupScreen> {
  final _ipController = TextEditingController();
  final _cloudflareController = TextEditingController();
  final _storage = StorageService();
  late final ApiService _api = ApiService(_storage);
  bool _loading = false;
  String? _message;
  bool _success = false;

  @override
  void dispose() {
    _ipController.dispose();
    _cloudflareController.dispose();
    super.dispose();
  }

  String get _selectedAddress {
    final cloudflare = _cloudflareController.text.trim();
    return cloudflare.isNotEmpty ? cloudflare : _ipController.text.trim();
  }

  Future<void> _test({bool saveAfterSuccess = false}) async {
    setState(() {
      _loading = true;
      _message = null;
      _success = false;
    });
    try {
      await _api.testConnection(_selectedAddress);
      final normalized = await _api.normalizeBaseUrl(_selectedAddress);
      if (saveAfterSuccess) {
        await _storage.setApiBaseUrl(normalized);
        if (!mounted) return;
        NotificationHelper.showSuccess(context, 'تم حفظ عنوان السيرفر بنجاح');
        Navigator.of(context).pushReplacement(
          MaterialPageRoute(builder: (_) => const LoginScreen()),
        );
        return;
      }
      setState(() {
        _success = true;
        _message = 'تم الوصول إلى السيرفر بنجاح';
      });
      if (mounted) {
        NotificationHelper.showSuccess(context, 'تم الوصول إلى السيرفر بنجاح');
      }
    } on ApiException catch (e) {
      setState(() => _message = e.message);
      if (mounted) {
        NotificationHelper.showError(context, e.message);
      }
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: GradientBackground(
        child: SafeArea(
          child: Center(
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(24),
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 480),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    // Logo
                    Container(
                      width: 72,
                      height: 72,
                      decoration: BoxDecoration(
                        gradient: AppTheme.gradientAccent,
                        borderRadius: BorderRadius.circular(20),
                        boxShadow: [
                          BoxShadow(
                            color: AppTheme.accent.withValues(alpha: 0.35),
                            blurRadius: 24,
                            offset: const Offset(0, 8),
                          ),
                        ],
                      ),
                      child: const Icon(
                        Icons.router_rounded,
                        color: Colors.white,
                        size: 36,
                      ),
                    ),
                    const SizedBox(height: 20),
                    const Text(
                      'إعداد الاتصال',
                      style: TextStyle(
                        color: Colors.white,
                        fontSize: 26,
                        fontWeight: FontWeight.w900,
                      ),
                    ),
                    const SizedBox(height: 28),

                    // Setup Card
                    Container(
                      decoration: AppTheme.glassCard(),
                      child: Padding(
                        padding: const EdgeInsets.all(28),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.stretch,
                          children: [
                            // Info box
                            Container(
                              padding: const EdgeInsets.all(14),
                              decoration: BoxDecoration(
                                color: AppTheme.info.withValues(alpha: 0.08),
                                borderRadius: BorderRadius.circular(
                                  AppTheme.radiusMd,
                                ),
                                border: Border.all(
                                  color: AppTheme.info.withValues(alpha: 0.2),
                                ),
                              ),
                              child: const Row(
                                children: [
                                  Icon(
                                    Icons.info_rounded,
                                    color: AppTheme.infoLight,
                                    size: 20,
                                  ),
                                  SizedBox(width: 10),
                                  Expanded(
                                    child: Text(
                                      'أدخل عنوان السيرفر أو رابط Cloudflare الخاص بلوحة الراديوس.',
                                      style: TextStyle(
                                        color: AppTheme.textSecondary,
                                        fontSize: 13,
                                      ),
                                    ),
                                  ),
                                ],
                              ),
                            ),
                            const SizedBox(height: 24),
                            TextField(
                              controller: _ipController,
                              keyboardType: TextInputType.url,
                              style: const TextStyle(
                                color: AppTheme.textPrimary,
                              ),
                              decoration: const InputDecoration(
                                labelText: 'عنوان السيرفر أو IP',
                                hintText: 'http://192.168.1.10:8080',
                                prefixIcon: Icon(Icons.dns_rounded),
                              ),
                            ),
                            const SizedBox(height: 16),
                            TextField(
                              controller: _cloudflareController,
                              keyboardType: TextInputType.url,
                              style: const TextStyle(
                                color: AppTheme.textPrimary,
                              ),
                              decoration: const InputDecoration(
                                labelText: 'رابط Cloudflare اختياري',
                                hintText: 'https://example.trycloudflare.com',
                                prefixIcon: Icon(Icons.cloud_rounded),
                              ),
                            ),
                            const SizedBox(height: 8),
                            const Text(
                              'إذا كتبت رابط Cloudflare سيتم استخدامه بدل عنوان IP.',
                              style: TextStyle(
                                color: AppTheme.textMuted,
                                fontSize: 12,
                              ),
                            ),
                            if (_message != null) ...[
                              const SizedBox(height: 16),
                              Container(
                                padding: const EdgeInsets.all(12),
                                decoration: BoxDecoration(
                                  color: _success
                                      ? AppTheme.success
                                          .withValues(alpha: 0.1)
                                      : AppTheme.danger
                                          .withValues(alpha: 0.1),
                                  borderRadius: BorderRadius.circular(
                                    AppTheme.radiusSm,
                                  ),
                                  border: Border.all(
                                    color: _success
                                        ? AppTheme.success
                                            .withValues(alpha: 0.3)
                                        : AppTheme.danger
                                            .withValues(alpha: 0.3),
                                  ),
                                ),
                                child: Row(
                                  children: [
                                    Icon(
                                      _success
                                          ? Icons.check_circle_rounded
                                          : Icons.error_outline_rounded,
                                      color: _success
                                          ? AppTheme.successLight
                                          : AppTheme.dangerLight,
                                      size: 18,
                                    ),
                                    const SizedBox(width: 8),
                                    Expanded(
                                      child: Text(
                                        _message!,
                                        style: TextStyle(
                                          color: _success
                                              ? AppTheme.successLight
                                              : AppTheme.dangerLight,
                                          fontWeight: FontWeight.w600,
                                          fontSize: 13,
                                        ),
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                            ],
                            const SizedBox(height: 24),
                            OutlinedButton.icon(
                              onPressed: _loading ? null : () => _test(),
                              icon: const Icon(Icons.wifi_tethering_rounded),
                              label: const Text('اختبار الاتصال'),
                            ),
                            const SizedBox(height: 10),
                            Container(
                              decoration: BoxDecoration(
                                gradient: _loading
                                    ? null
                                    : AppTheme.gradientPrimary,
                                color: _loading
                                    ? AppTheme.bgSurface
                                    : null,
                                borderRadius: BorderRadius.circular(
                                  AppTheme.radiusMd,
                                ),
                                boxShadow: _loading
                                    ? null
                                    : [
                                        BoxShadow(
                                          color: AppTheme.primary
                                              .withValues(alpha: 0.3),
                                          blurRadius: 12,
                                          offset: const Offset(0, 4),
                                        ),
                                      ],
                              ),
                              child: ElevatedButton.icon(
                                onPressed: _loading
                                    ? null
                                    : () => _test(saveAfterSuccess: true),
                                icon: _loading
                                    ? const SizedBox(
                                        width: 18,
                                        height: 18,
                                        child: CircularProgressIndicator(
                                          strokeWidth: 2,
                                          color: Colors.white70,
                                        ),
                                      )
                                    : const Icon(
                                        Icons.arrow_back_rounded,
                                        color: Colors.white,
                                      ),
                                label: const Text(
                                  'حفظ ومتابعة',
                                  style: TextStyle(color: Colors.white),
                                ),
                                style: ElevatedButton.styleFrom(
                                  backgroundColor: Colors.transparent,
                                  shadowColor: Colors.transparent,
                                  minimumSize: const Size(0, 52),
                                ),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
