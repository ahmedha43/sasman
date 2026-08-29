import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../../../config/app_theme.dart';
import '../../../l10n/app_localizations.dart';
import '../../../providers/language_provider.dart';
import '../../../providers/theme_provider.dart';
import '../../../services/api_service.dart';
import '../../../services/auth_service.dart';
import '../../../services/storage_service.dart';
import '../../../widgets/notification_helper.dart';
import '../../auth/login_screen.dart';

class SettingsView extends StatelessWidget {
  const SettingsView({
    super.key,
    required this.storage,
    required this.auth,
    required this.api,
  });

  final StorageService storage;
  final AuthService auth;
  final ApiService api;

  @override
  Widget build(BuildContext context) {
    final loc = AppLocalizations.of(context);
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        Text(
          loc.navSettings,
          style: Theme.of(context).textTheme.headlineSmall?.copyWith(
            fontWeight: FontWeight.w900,
          ),
        ),
        const SizedBox(height: 8),
        Text(
          loc.serverInfo,
          style: const TextStyle(color: AppTheme.textSecondary),
        ),
        const SizedBox(height: 20),

        // ── Language Settings ──
        Container(
          padding: const EdgeInsets.all(16),
          decoration: AppTheme.premiumCard(),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  const Icon(Icons.language, color: AppTheme.primaryLight),
                  const SizedBox(width: 8),
                  const Text(
                    'Language / اللغة',
                    style: TextStyle(fontWeight: FontWeight.w900, fontSize: 16),
                  ),
                ],
              ),
              const SizedBox(height: 14),
              _LanguageTile(
                flag: '🇮🇶',
                name: 'العربية',
                code: 'ar',
              ),
              const SizedBox(height: 8),
              _LanguageTile(
                flag: '🇺🇸',
                name: 'English',
                code: 'en',
              ),
              const SizedBox(height: 8),
              _LanguageTile(
                flag: '🇹🇷',
                name: 'Türkçe',
                code: 'tr',
              ),
              const SizedBox(height: 8),
              _LanguageTile(
                flag: '🇹🇯',
                name: 'Kurdish / کوردی',
                code: 'ku',
              ),
              const SizedBox(height: 8),
              _LanguageTile(
                flag: '🇮🇷',
                name: 'فارسی',
                code: 'fa',
              ),
              const SizedBox(height: 8),
              _LanguageTile(
                flag: '🇫🇷',
                name: 'Français',
                code: 'fr',
              ),
              const SizedBox(height: 8),
              _LanguageTile(
                flag: '🇩🇪',
                name: 'Deutsch',
                code: 'de',
              ),
              const SizedBox(height: 8),
              _LanguageTile(
                flag: '🇪🇸',
                name: 'Español',
                code: 'es',
              ),
              const SizedBox(height: 8),
              _LanguageTile(
                flag: '🇷🇺',
                name: 'Русский',
                code: 'ru',
              ),
            ],
          ),
        ),
        const SizedBox(height: 16),

        // ── Theme Settings ──
        Container(
          padding: const EdgeInsets.all(16),
          decoration: AppTheme.premiumCard(),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  const Icon(Icons.dark_mode_rounded, color: AppTheme.primaryLight),
                  const SizedBox(width: 8),
                  const Text(
                    'Appearance / المظهر',
                    style: TextStyle(fontWeight: FontWeight.w900, fontSize: 16),
                  ),
                ],
              ),
              const SizedBox(height: 14),
              _ThemeTile(
                icon: Icons.brightness_auto_rounded,
                title: 'System Default',
                subtitle: 'اتباع إعدادات النظام',
                mode: ThemeMode.system,
              ),
              const SizedBox(height: 8),
              _ThemeTile(
                icon: Icons.light_mode_rounded,
                title: 'Light Mode',
                subtitle: 'الوضع الفاتح',
                mode: ThemeMode.light,
              ),
              const SizedBox(height: 8),
              _ThemeTile(
                icon: Icons.dark_mode_rounded,
                title: 'Dark Mode',
                subtitle: 'الوضع الداكن',
                mode: ThemeMode.dark,
              ),
            ],
          ),
        ),
        const SizedBox(height: 16),

        // ── Server Info ──
        Container(
          padding: const EdgeInsets.all(16),
          decoration: AppTheme.premiumCard(),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  const Icon(Icons.dns_rounded, color: AppTheme.primaryLight),
                  const SizedBox(width: 8),
                  Text(
                    loc.serverAddress,
                    style: const TextStyle(fontWeight: FontWeight.w900, fontSize: 16),
                  ),
                ],
              ),
              const SizedBox(height: 14),
              FutureBuilder<String?>(
                future: storage.getApiBaseUrl(),
                builder: (context, snapshot) {
                  return Text(
                    snapshot.data ?? '-',
                    style: const TextStyle(
                      color: AppTheme.textSecondary,
                      fontWeight: FontWeight.w700,
                    ),
                  );
                },
              ),
            ],
          ),
        ),
        const SizedBox(height: 16),

        // ── Logout ──
        SizedBox(
          width: double.infinity,
          child: ElevatedButton.icon(
            style: ElevatedButton.styleFrom(
              backgroundColor: AppTheme.danger,
            ),
            onPressed: () async {
              await auth.logout();
              if (!context.mounted) return;
              NotificationHelper.showSuccess(context, loc.logoutSuccess);
              Navigator.of(context).pushReplacement(
                MaterialPageRoute(builder: (_) => const LoginScreen()),
              );
            },
            icon: const Icon(Icons.logout),
            label: Text(loc.logout),
          ),
        ),
      ],
    );
  }
}

class _LanguageTile extends StatelessWidget {
  const _LanguageTile({
    required this.flag,
    required this.name,
    required this.code,
  });

  final String flag;
  final String name;
  final String code;

  @override
  Widget build(BuildContext context) {
    final langProvider = context.watch<LanguageProvider>();
    final selected = langProvider.locale.languageCode == code;
    return Material(
      color: selected
          ? AppTheme.primary.withValues(alpha: 0.15)
          : Colors.transparent,
      borderRadius: BorderRadius.circular(12),
      child: InkWell(
        borderRadius: BorderRadius.circular(12),
        onTap: () => langProvider.setLocale(Locale(code)),
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(12),
            border: selected
                ? Border.all(color: AppTheme.primary.withValues(alpha: 0.4))
                : Border.all(color: AppTheme.border),
          ),
          child: Row(
            children: [
              Text(flag, style: const TextStyle(fontSize: 24)),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  name,
                  style: TextStyle(
                    fontWeight: selected ? FontWeight.w900 : FontWeight.w700,
                    color: selected ? Colors.white : AppTheme.textSecondary,
                  ),
                ),
              ),
              if (selected)
                const Icon(Icons.check_circle, color: AppTheme.primary, size: 22),
            ],
          ),
        ),
      ),
    );
  }
}

class _ThemeTile extends StatelessWidget {
  const _ThemeTile({
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.mode,
  });

  final IconData icon;
  final String title;
  final String subtitle;
  final ThemeMode mode;

  @override
  Widget build(BuildContext context) {
    final themeProvider = context.watch<ThemeProvider>();
    final selected = themeProvider.themeMode == mode;
    return Material(
      color: selected
          ? AppTheme.primary.withValues(alpha: 0.15)
          : Colors.transparent,
      borderRadius: BorderRadius.circular(12),
      child: InkWell(
        borderRadius: BorderRadius.circular(12),
        onTap: () {
          if (mode == ThemeMode.system) {
            themeProvider.setSystemTheme();
          } else if (mode == ThemeMode.light) {
            themeProvider.setLightTheme();
          } else {
            themeProvider.setDarkTheme();
          }
        },
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(12),
            border: selected
                ? Border.all(color: AppTheme.primary.withValues(alpha: 0.4))
                : Border.all(color: AppTheme.border),
          ),
          child: Row(
            children: [
              Icon(icon, color: selected ? AppTheme.primary : AppTheme.textSecondary, size: 22),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      title,
                      style: TextStyle(
                        fontWeight: selected ? FontWeight.w900 : FontWeight.w700,
                        color: selected ? Colors.white : AppTheme.textSecondary,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      subtitle,
                      style: TextStyle(
                        fontSize: 12,
                        color: selected ? AppTheme.textMuted : AppTheme.textMuted,
                      ),
                    ),
                  ],
                ),
              ),
              if (selected)
                const Icon(Icons.check_circle, color: AppTheme.primary, size: 22),
            ],
          ),
        ),
      ),
    );
  }
}
