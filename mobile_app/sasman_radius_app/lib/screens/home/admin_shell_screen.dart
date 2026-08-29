import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../config/app_theme.dart';
import '../../l10n/app_localizations.dart';
import '../../providers/language_provider.dart';
import '../../screens/admin/logs/logs_screen.dart';
import '../../screens/admin/nas/nas_screen.dart';
import '../../screens/admin/profiles/profiles_screen.dart';
import '../../screens/admin/settings/advanced_settings_screen.dart';
import '../../screens/admin/settings/agents_screen.dart';
import '../../screens/admin/streams/streams_screen.dart';
import '../../screens/admin/users/users_screen.dart';
import '../../screens/admin/vouchers/vouchers_screen.dart';
import '../../screens/admin/whatsapp/whatsapp_screen.dart';
import '../../services/api_service.dart';
import '../../services/auth_service.dart';
import '../../services/storage_service.dart';
import '../../widgets/notification_helper.dart';
import '../auth/login_screen.dart';
import 'widgets/admin_side_navigation.dart';
import 'widgets/dashboard_view.dart';
import 'widgets/license_gate_widgets.dart';
import 'widgets/section_model.dart';
import 'widgets/settings_view.dart';

class AdminShellScreen extends StatefulWidget {
  const AdminShellScreen({super.key});

  @override
  State<AdminShellScreen> createState() => _AdminShellScreenState();
}

class _AdminShellScreenState extends State<AdminShellScreen> {
  final _storage = StorageService();
  late final _api = ApiService(_storage);
  late final _auth = AuthService(_api, _storage);
  int _index = 0;
  bool _checkingLicense = true;
  bool _activatingLicense = false;
  bool _licenseValid = false;
  bool _routerConnected = false;
  String _licenseMsg = '';
  String _serial = '';
  String _expires = '';

  // Admin info for role-based visibility
  String _adminRole = 'agent';

  final _licenseKeyCtrl = TextEditingController();
  final _routerAddressCtrl = TextEditingController();
  final _routerUserCtrl = TextEditingController();
  final _routerPassCtrl = TextEditingController();
  final _usersScreenKey = GlobalKey<UsersScreenState>();

  // Sections that are superadmin-only by index
  static const _superAdminOnlyIndices = {2, 5, 9}; // Agents, NAS, Advanced

  late final List<Section> _allSections = [];

  List<Section> get _sections {
    if (_adminRole == 'superadmin') return _allSections;
    return _allSections
        .asMap()
        .entries
        .where((e) => !_superAdminOnlyIndices.contains(e.key))
        .map((e) => e.value)
        .toList();
  }


  @override
  void initState() {
    super.initState();
    _loadLicenseStatus();
    _loadAdminInfo();
  }

  /// Build sections with localized titles (called on every build so
  /// the titles update when the language changes).
  List<Section> _buildSections(AppLocalizations loc) {
    return [
      Section(loc.navDashboard, Icons.dashboard, DashboardView(
        api: _api,
        onNavigateToUsers: (filter) {
          final usersIdx = _sections.indexWhere(
            (sec) => sec.title == loc.navUsers,
          );
          if (usersIdx != -1) {
            setState(() { _index = usersIdx; });
            WidgetsBinding.instance.addPostFrameCallback((_) {
              _usersScreenKey.currentState?.setFilter(filter);
            });
          }
        },
      )),
      Section(loc.navUsers, Icons.people, UsersScreen(key: _usersScreenKey, api: _api)),
      Section(loc.navAgents, Icons.supervised_user_circle, AgentsScreen(api: _api)),
      Section(loc.navProfiles, Icons.speed, ProfilesScreen(api: _api)),
      Section(loc.navVouchers, Icons.confirmation_number, VouchersScreen(api: _api)),
      Section(loc.navNas, Icons.router, NasScreen(api: _api)),
      Section(loc.navStreams, Icons.live_tv, StreamsScreen(api: _api)),
      Section(loc.navWhatsApp, Icons.chat, WhatsAppScreen(api: _api)),
      Section(loc.navLogs, Icons.history, LogsScreen(api: _api)),
      Section(loc.navAdvancedSettings, Icons.tune, AdvancedSettingsScreen(api: _api)),
      Section(loc.navSettings, Icons.settings, SettingsView(
        storage: _storage, auth: _auth, api: _api,
      )),
    ];
  }

  Future<void> _loadAdminInfo() async {
    try {
      final res = await _api.get('/radius/api/auth/me');
      final me = _decodeMap(res.body);
      if (mounted) {
        setState(() {
          _adminRole = me['role']?.toString() ?? 'agent';
        });
      }
    } catch (_) {}
  }

  @override
  void dispose() {
    _licenseKeyCtrl.dispose();
    _routerAddressCtrl.dispose();
    _routerUserCtrl.dispose();
    _routerPassCtrl.dispose();
    super.dispose();
  }

  Future<void> _loadLicenseStatus() async {
    if (mounted) setState(() => _checkingLicense = true);
    try {
      final res = await _api.get('/radius/api/license/status');
      final data = _decodeMap(res.body);
      if (!mounted) return;
      setState(() {
        _licenseValid = data['valid'] == true;
        _routerConnected = data['router_connected'] == true;
        _licenseMsg = data['message']?.toString() ?? '';
        _serial = data['serial']?.toString() ?? '';
        _expires = data['expires']?.toString() ?? data['expires_at']?.toString() ?? '';
        _checkingLicense = false;
      });
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _licenseValid = false;
        _routerConnected = false;
        _licenseMsg = e.message;
        _checkingLicense = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _licenseValid = false;
        _routerConnected = false;
        _licenseMsg = e.toString();
        _checkingLicense = false;
      });
    }
  }

  Future<void> _activateLicense() async {
    final loc = AppLocalizations.of(context);
    final key = _licenseKeyCtrl.text.trim();
    if (key.isEmpty) {
      NotificationHelper.showError(context, loc.enterLicenseKey);
      return;
    }

    final body = <String, dynamic>{'key': key};
    if (!_routerConnected && _routerAddressCtrl.text.trim().isNotEmpty) {
      body['address'] = _routerAddressCtrl.text.trim();
      body['user'] = _routerUserCtrl.text.trim();
      body['pass'] = _routerPassCtrl.text;
    }

    setState(() => _activatingLicense = true);
    try {
      await _api.post('/radius/api/license/activate', body: body);
      if (!mounted) return;
      NotificationHelper.showSuccess(context, loc.activateSuccess);
      _licenseKeyCtrl.clear();
      await _loadLicenseStatus();
    } on ApiException catch (e) {
      if (mounted) NotificationHelper.showError(context, e.message);
    } finally {
      if (mounted) setState(() => _activatingLicense = false);
    }
  }

  Future<void> _connectRouter() async {
    final loc = AppLocalizations.of(context);
    final address = _routerAddressCtrl.text.trim();
    final user = _routerUserCtrl.text.trim();
    if (address.isEmpty || user.isEmpty) {
      NotificationHelper.showError(context, loc.enterRouterAddress);
      return;
    }

    setState(() => _activatingLicense = true);
    try {
      await _api.post(
        '/radius/api/router/connect',
        body: {'address': address, 'user': user, 'pass': _routerPassCtrl.text},
      );
      if (!mounted) return;
      NotificationHelper.showSuccess(context, loc.connectRouterSuccess);
      await _loadLicenseStatus();
    } on ApiException catch (e) {
      if (mounted) NotificationHelper.showError(context, e.message);
    } finally {
      if (mounted) setState(() => _activatingLicense = false);
    }
  }

  Future<void> _copyText(String label, String value) async {
    final loc = AppLocalizations.of(context);
    await Clipboard.setData(ClipboardData(text: value));
    if (!mounted) return;
    NotificationHelper.showSuccess(context, '${loc.copiedLabel} $label');
  }

  Future<void> _openUrl(String rawUrl) async {
    final loc = AppLocalizations.of(context);
    final opened = await launchUrl(
      Uri.parse(rawUrl),
      mode: LaunchMode.externalApplication,
    );
    if (!opened && mounted) {
      NotificationHelper.showError(context, loc.failedOpenUrl);
    }
  }

  Future<void> _logout() async {
    final loc = AppLocalizations.of(context);
    await _auth.logout();
    if (!mounted) return;
    NotificationHelper.showSuccess(context, loc.logoutSuccess);
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(builder: (_) => const LoginScreen()),
    );
  }

  @override
  Widget build(BuildContext context) {
    final loc = AppLocalizations.of(context);
    final langProvider = context.watch<LanguageProvider>();

    // Rebuild sections with current locale
    _allSections.clear();
    _allSections.addAll(_buildSections(loc));

    final selectedIndex = _index.clamp(0, _sections.length - 1);
    final selected = _sections.isNotEmpty ? _sections[selectedIndex] : null;

    return LayoutBuilder(
      builder: (context, constraints) {
        final wide = constraints.maxWidth >= 980;
        final locked = !_checkingLicense && !_licenseValid;
        return Scaffold(
          appBar: AppBar(
            title: Text(
              locked
                  ? loc.licenseActivation
                  : (selected?.title ?? ''),
              style: const TextStyle(fontSize: 16),
              overflow: TextOverflow.ellipsis,
            ),
            actions: [
              // Language switcher button
              if (!locked && !_checkingLicense)
                Padding(
                  padding: const EdgeInsets.symmetric(vertical: 8),
                  child: GestureDetector(
                    onTap: () => langProvider.cycleLocale(),
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 8,
                        vertical: 4,
                      ),
                      decoration: BoxDecoration(
                        color: AppTheme.primary.withValues(alpha: 0.15),
                        borderRadius: BorderRadius.circular(20),
                        border: Border.all(
                          color: AppTheme.primary.withValues(alpha: 0.3),
                        ),
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Text(
                            langProvider.currentFlag,
                            style: const TextStyle(fontSize: 16),
                          ),
                          const SizedBox(width: 4),
                          Text(
                            langProvider.currentLanguageName,
                            style: const TextStyle(
                              fontWeight: FontWeight.w800,
                              fontSize: 11,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              IconButton(
                tooltip: loc.refresh,
                onPressed: locked || _checkingLicense
                    ? _loadLicenseStatus
                    : () {
                        _loadLicenseStatus();
                        _loadAdminInfo();
                        setState(() {});
                      },
                icon: const Icon(Icons.refresh),
              ),
              IconButton(
                tooltip: loc.logout,
                onPressed: _logout,
                icon: const Icon(Icons.logout),
              ),
              const SizedBox(width: 4),
            ],
          ),
          drawer: wide || locked || _checkingLicense
              ? null
              : _AdminNavigationDrawer(
                  sections: _sections,
                  selectedIndex: selectedIndex,
                  onDestinationSelected: (value) {
                    setState(() => _index = value);
                    Navigator.of(context).pop();
                  },
                ),
          body: _checkingLicense
              ? const Center(child: CircularProgressIndicator())
              : locked
              ? LicenseGate(
                  message: _licenseMsg,
                  serial: _serial,
                  expires: _expires,
                  routerConnected: _routerConnected,
                  busy: _activatingLicense,
                  licenseKeyController: _licenseKeyCtrl,
                  routerAddressController: _routerAddressCtrl,
                  routerUserController: _routerUserCtrl,
                  routerPassController: _routerPassCtrl,
                  onRefresh: _loadLicenseStatus,
                  onActivate: _activateLicense,
                  onConnectRouter: _connectRouter,
                  onCopy: _copyText,
                  onOpenUrl: _openUrl,
                )
              : wide
              ? Row(
                  children: [
                    AdminSideNavigation(
                      sections: _sections,
                      selectedIndex: selectedIndex,
                      onDestinationSelected: (value) =>
                          setState(() => _index = value),
                    ),
                    const VerticalDivider(width: 1, color: AppTheme.border),
                    Expanded(child: selected!.child),
                  ],
                )
              : selected!.child,
        );
      },
    );
  }
}

/// Mobile navigation drawer using extracted widgets.
class _AdminNavigationDrawer extends StatelessWidget {
  const _AdminNavigationDrawer({
    required this.sections,
    required this.selectedIndex,
    required this.onDestinationSelected,
  });

  final List<Section> sections;
  final int selectedIndex;
  final ValueChanged<int> onDestinationSelected;

  @override
  Widget build(BuildContext context) {
    return NavigationDrawer(
      selectedIndex: selectedIndex,
      onDestinationSelected: onDestinationSelected,
      children: [
        const Padding(
          padding: EdgeInsets.fromLTRB(20, 28, 20, 18),
          child: Row(
            children: [
              // Brand mark
              SizedBox(width: 44, height: 44),
            ],
          ),
        ),
        for (final section in sections)
          NavigationDrawerDestination(
            icon: Icon(section.icon),
            label: Text(section.title),
          ),
      ],
    );
  }
}

Map<String, dynamic> _decodeMap(String body) {
  final decoded = jsonDecode(body);
  if (decoded is Map) {
    return Map<String, dynamic>.from(decoded);
  }
  return {};
}

/// DashboardView is defined in dashboard_screen.dart but we need
/// a type alias here. The actual DashboardView screen handles navigation.
/// This is imported via the dashboard_screen import above.
/// NOTE: DashboardView and SettingsView are separate screen files.
/// They remain in their original locations and are imported here.