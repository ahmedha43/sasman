import 'package:flutter/material.dart';

import '../../config/app_theme.dart';
import '../../services/api_service.dart';
import '../../services/auth_service.dart';
import '../../services/storage_service.dart';
import '../../widgets/gradient_background.dart';
import '../../widgets/notification_helper.dart';
import '../home/admin_shell_screen.dart';
import 'server_setup_screen.dart';

class LoginScreen extends StatefulWidget {
  const LoginScreen({super.key});

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final _usernameController = TextEditingController(text: 'admin');
  final _passwordController = TextEditingController(text: 'admin');
  final _storage = StorageService();
  late final _auth = AuthService(ApiService(_storage), _storage);
  bool _obscure = true;
  bool _loading = false;
  String? _error;
  String? _baseUrl;

  @override
  void initState() {
    super.initState();
    _storage.getApiBaseUrl().then((value) {
      if (mounted) setState(() => _baseUrl = value);
    });
  }

  @override
  void dispose() {
    _usernameController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  Future<void> _login() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      await _auth.login(_usernameController.text, _passwordController.text);
      if (!mounted) return;
      NotificationHelper.showSuccess(context, 'تم تسجيل الدخول بنجاح');
      Navigator.of(context).pushReplacement(
        MaterialPageRoute(builder: (_) => const AdminShellScreen()),
      );
    } on ApiException catch (e) {
      setState(() => _error = e.message);
      if (mounted) {
        NotificationHelper.showError(context, e.message);
      }
    } catch (_) {
      setState(() => _error = 'تعذر تسجيل الدخول');
      if (mounted) {
        NotificationHelper.showError(context, 'تعذر تسجيل الدخول');
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
                constraints: const BoxConstraints(maxWidth: 440),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    // Logo
                    Container(
                      width: 72,
                      height: 72,
                      decoration: BoxDecoration(
                        gradient: AppTheme.gradientPrimary,
                        borderRadius: BorderRadius.circular(20),
                        boxShadow: [
                          BoxShadow(
                            color: AppTheme.primary.withValues(alpha: 0.35),
                            blurRadius: 24,
                            offset: const Offset(0, 8),
                          ),
                        ],
                      ),
                      child: const Icon(
                        Icons.shield_rounded,
                        color: Colors.white,
                        size: 36,
                      ),
                    ),
                    const SizedBox(height: 20),
                    const Text(
                      'SASMAN RADIUS',
                      style: TextStyle(
                        color: Colors.white,
                        fontSize: 26,
                        fontWeight: FontWeight.w900,
                        letterSpacing: 2,
                      ),
                    ),
                    const SizedBox(height: 28),

                    // Login Card
                    Container(
                      decoration: AppTheme.glassCard(),
                      child: Padding(
                        padding: const EdgeInsets.all(28),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.stretch,
                          children: [
                            Text(
                              'تسجيل الدخول',
                              textAlign: TextAlign.center,
                              style: Theme.of(context)
                                  .textTheme
                                  .headlineSmall
                                  ?.copyWith(
                                    fontWeight: FontWeight.w800,
                                    color: AppTheme.textPrimary,
                                  ),
                            ),
                            if (_baseUrl != null) ...[
                              const SizedBox(height: 8),
                              Container(
                                padding: const EdgeInsets.symmetric(
                                  horizontal: 12,
                                  vertical: 6,
                                ),
                                decoration: BoxDecoration(
                                  color: AppTheme.bgSurface,
                                  borderRadius: BorderRadius.circular(
                                    AppTheme.radiusSm,
                                  ),
                                ),
                                child: Text(
                                  _baseUrl!,
                                  textAlign: TextAlign.center,
                                  style: const TextStyle(
                                    color: AppTheme.textMuted,
                                    fontSize: 12,
                                    fontFamily: 'monospace',
                                  ),
                                ),
                              ),
                            ],
                            const SizedBox(height: 28),
                            TextField(
                              controller: _usernameController,
                              style: const TextStyle(
                                color: AppTheme.textPrimary,
                              ),
                              decoration: const InputDecoration(
                                labelText: 'اسم المستخدم',
                                prefixIcon: Icon(Icons.person_rounded),
                              ),
                            ),
                            const SizedBox(height: 16),
                            TextField(
                              controller: _passwordController,
                              obscureText: _obscure,
                              style: const TextStyle(
                                color: AppTheme.textPrimary,
                              ),
                              decoration: InputDecoration(
                                labelText: 'كلمة المرور',
                                prefixIcon: const Icon(Icons.lock_rounded),
                                suffixIcon: IconButton(
                                  onPressed: () =>
                                      setState(() => _obscure = !_obscure),
                                  icon: Icon(
                                    _obscure
                                        ? Icons.visibility_rounded
                                        : Icons.visibility_off_rounded,
                                  ),
                                ),
                              ),
                            ),
                            if (_error != null) ...[
                              const SizedBox(height: 16),
                              Container(
                                padding: const EdgeInsets.all(12),
                                decoration: BoxDecoration(
                                  color: AppTheme.danger.withValues(alpha: 0.1),
                                  borderRadius: BorderRadius.circular(
                                    AppTheme.radiusSm,
                                  ),
                                  border: Border.all(
                                    color:
                                        AppTheme.danger.withValues(alpha: 0.3),
                                  ),
                                ),
                                child: Row(
                                  children: [
                                    const Icon(
                                      Icons.error_outline_rounded,
                                      color: AppTheme.dangerLight,
                                      size: 18,
                                    ),
                                    const SizedBox(width: 8),
                                    Expanded(
                                      child: Text(
                                        _error!,
                                        style: const TextStyle(
                                          color: AppTheme.dangerLight,
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
                            // Login button with gradient
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
                                onPressed: _loading ? null : _login,
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
                                        Icons.login_rounded,
                                        color: Colors.white,
                                      ),
                                label: const Text(
                                  'دخول إلى اللوحة',
                                  style: TextStyle(color: Colors.white),
                                ),
                                style: ElevatedButton.styleFrom(
                                  backgroundColor: Colors.transparent,
                                  shadowColor: Colors.transparent,
                                  minimumSize: const Size(0, 52),
                                ),
                              ),
                            ),
                            const SizedBox(height: 8),
                            TextButton.icon(
                              onPressed: _loading
                                  ? null
                                  : () =>
                                      Navigator.of(context).pushReplacement(
                                        MaterialPageRoute(
                                          builder: (_) =>
                                              const ServerSetupScreen(),
                                        ),
                                      ),
                              icon: const Icon(
                                Icons.settings_ethernet_rounded,
                                size: 18,
                              ),
                              label: const Text('تغيير عنوان السيرفر'),
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
