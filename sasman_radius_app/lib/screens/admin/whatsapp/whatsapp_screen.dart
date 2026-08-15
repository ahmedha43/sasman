import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';

import '../../../config/app_theme.dart';
import '../../../l10n/app_localizations.dart';
import '../../../services/api_service.dart';
import '../../../widgets/notification_helper.dart';

// ─── Models ──────────────────────────────────────────────────────────────────

class _WAConfig {
  final int enabled;
  final String phoneNumber;
  final int reminderEnabled;
  final int reminderHours;
  final String status; // connected | waiting | disconnected

  const _WAConfig({
    required this.enabled,
    required this.phoneNumber,
    required this.reminderEnabled,
    required this.reminderHours,
    required this.status,
  });

  factory _WAConfig.fromJson(Map<String, dynamic> j) => _WAConfig(
    enabled: (j['enabled'] as num?)?.toInt() ?? 0,
    phoneNumber: j['phone_number']?.toString() ?? '',
    reminderEnabled: (j['reminder_enabled'] as num?)?.toInt() ?? 0,
    reminderHours: (j['reminder_hours'] as num?)?.toInt() ?? 24,
    status: j['status']?.toString() ?? 'disconnected',
  );
}

class _WATemplate {
  final int id;
  final String key;
  String text;
  _WATemplate({required this.id, required this.key, required this.text});

  factory _WATemplate.fromJson(Map<String, dynamic> j) => _WATemplate(
    id: (j['id'] as num?)?.toInt() ?? 0,
    key: j['template_key']?.toString() ?? '',
    text: j['template_text']?.toString() ?? '',
  );
}

// ─── Labels for template keys ─────────────────────────────────────────────────
Map<String, String> _templateLabels(AppLocalizations l) => {
  'renew_paid': l.waTemplateRenewPaid,
  'renew_debt': l.waTemplateRenewDebt,
  'add_debt': l.waTemplateAddDebt,
  'payment': l.waTemplatePayment,
  'expiry_reminder': l.waTemplateExpiryReminder,
  'debt_reminder': l.waTemplateDebtReminder,
};

// ─── Main Widget ──────────────────────────────────────────────────────────────

class WhatsAppScreen extends StatefulWidget {
  const WhatsAppScreen({super.key, required this.api});
  final ApiService api;

  @override
  State<WhatsAppScreen> createState() => _WhatsAppScreenState();
}

class _WhatsAppScreenState extends State<WhatsAppScreen>
    with SingleTickerProviderStateMixin {
  late final TabController _tabs;

  _WAConfig? _config;
  List<_WATemplate> _templates = [];
  bool _loadingConfig = true;
  bool _loadingTemplates = true;
  String? _configError;
  String? _templateError;

  // QR state
  String? _qrImageBase64;
  bool _loadingQR = false;

  // Broadcast
  final _broadcastCtrl = TextEditingController();
  bool _sendingBroadcast = false;
  bool _sendingDebt = false;

  // WA Polling
  Timer? _waPollTimer;

  @override
  void initState() {
    super.initState();
    _tabs = TabController(length: 3, vsync: this);
    _loadConfig();
    _loadTemplates();
    _broadcastCtrl.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _waPollTimer?.cancel();
    _tabs.dispose();
    _broadcastCtrl.dispose();
    super.dispose();
  }

  void _startWAPolling() {
    _waPollTimer?.cancel();
    _waPollTimer = Timer.periodic(const Duration(seconds: 5), (_) async {
      try {
        final res = await widget.api.get('/radius/api/whatsapp/config');
        final decoded = jsonDecode(res.body);
        if (decoded is Map<String, dynamic> && decoded['status'] == 'connected') {
          _waPollTimer?.cancel();
          await _loadConfig();
          if (mounted) {
            final l = AppLocalizations.of(context);
            NotificationHelper.showSuccess(context, '${l.waDisconnected.split(' ').first} ✅');
          }
        }
      } catch (_) {}
    });
  }

  // ─── Loaders ───────────────────────────────────────────────────────────────

  Future<void> _loadConfig() async {
    setState(() {
      _loadingConfig = true;
      _configError = null;
    });
    try {
      final res = await widget.api.get('/radius/api/whatsapp/config');
      final decoded = jsonDecode(res.body);
      if (decoded is Map<String, dynamic>) {
        setState(() => _config = _WAConfig.fromJson(decoded));
      } else {
        final l = AppLocalizations.of(context);
        setState(() => _configError = l.waInvalidServerData);
      }
    } on ApiException catch (e) {
      setState(() => _configError = e.message);
    } catch (e) {
      final l = AppLocalizations.of(context);
      setState(() => _configError = l.waConfigLoadError);
    } finally {
      setState(() => _loadingConfig = false);
    }
  }

  Future<void> _loadTemplates() async {
    setState(() {
      _loadingTemplates = true;
      _templateError = null;
    });
    try {
      final res = await widget.api.get('/radius/api/whatsapp/templates');
      final decoded = jsonDecode(res.body);
      if (decoded is List) {
        setState(() {
          _templates = decoded
              .whereType<Map<String, dynamic>>()
              .map(_WATemplate.fromJson)
              .toList();
        });
      } else {
        final l = AppLocalizations.of(context);
        setState(() => _templateError = l.waInvalidServerData);
      }
    } on ApiException catch (e) {
      setState(() => _templateError = e.message);
    } catch (e) {
      final l = AppLocalizations.of(context);
      setState(() => _templateError = l.waTemplatesLoadError);
    } finally {
      setState(() => _loadingTemplates = false);
    }
  }

  // ─── Save Config ───────────────────────────────────────────────────────────

  Future<void> _showConfigDialog() async {
    final l = AppLocalizations.of(context);
    final phoneCtrl = TextEditingController(text: _config?.phoneNumber ?? '');
    var enabled = (_config?.enabled ?? 0) == 1;
    var reminderEnabled = (_config?.reminderEnabled ?? 0) == 1;
    var reminderHours = _config?.reminderHours ?? 24;
    final hoursCtrl = TextEditingController(text: reminderHours.toString());
    bool saving = false;

    await showDialog<void>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setDlg) => AlertDialog(
          title: Row(
            children: [
              const Icon(Icons.settings, color: AppTheme.primary),
              const SizedBox(width: 10),
              Text(l.waConfigTitle),
            ],
          ),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                SwitchListTile(
                  value: enabled,
                  title: Text(l.waEnableService),
                  onChanged: (v) => setDlg(() => enabled = v),
                  contentPadding: EdgeInsets.zero,
                ),
                const SizedBox(height: 8),
                TextField(
                  controller: phoneCtrl,
                  keyboardType: TextInputType.phone,
                  decoration: InputDecoration(
                    labelText: l.waPhoneNumberLabel,
                    prefixIcon: const Icon(Icons.phone),
                    hintText: '9647700000000',
                  ),
                ),
                const SizedBox(height: 14),
                const Divider(),
                const SizedBox(height: 8),
                Text(
                  l.waReminderSettingsTitle,
                  style: const TextStyle(
                    fontWeight: FontWeight.w700,
                    color: AppTheme.textSecondary,
                    fontSize: 13,
                  ),
                ),
                const SizedBox(height: 8),
                SwitchListTile(
                  value: reminderEnabled,
                  title: Text(l.waEnableReminder),
                  onChanged: (v) => setDlg(() => reminderEnabled = v),
                  contentPadding: EdgeInsets.zero,
                ),
                if (reminderEnabled)
                  TextField(
                    controller: hoursCtrl,
                    keyboardType: TextInputType.number,
                    decoration: InputDecoration(
                      labelText: l.waReminderHoursLabel,
                      prefixIcon: const Icon(Icons.timer),
                    ),
                  ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(),
              child: Text(l.cancel),
            ),
            ElevatedButton(
              onPressed: saving
                  ? null
                  : () async {
                      setDlg(() => saving = true);
                      try {
                        await widget.api.post(
                          '/radius/api/whatsapp/config',
                          body: {
                            'enabled': enabled ? 1 : 0,
                            'phone_number': phoneCtrl.text.trim(),
                            'reminder_enabled': reminderEnabled ? 1 : 0,
                            'reminder_hours':
                                int.tryParse(hoursCtrl.text) ?? 24,
                          },
                        );
                        await _loadConfig();
                        if (ctx.mounted) {
                          NotificationHelper.showSuccess(
                            ctx,
                            l.waConfigSaved,
                          );
                          Navigator.of(ctx).pop();
                        }
                      } on ApiException catch (e) {
                        if (ctx.mounted) {
                          NotificationHelper.showError(ctx, e.message);
                        }
                      } finally {
                        if (ctx.mounted) setDlg(() => saving = false);
                      }
                    },
              child: saving
                  ? const SizedBox(
                      width: 18,
                      height: 18,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : Text(l.waSaveConfig),
            ),
          ],
        ),
      ),
    );
  }

  // ─── QR ────────────────────────────────────────────────────────────────────

  Future<void> _fetchQR() async {
    final l = AppLocalizations.of(context);
    setState(() {
      _loadingQR = true;
      _qrImageBase64 = null;
    });
    try {
      final res = await widget.api.get('/radius/api/whatsapp/qr');
      final decoded = jsonDecode(res.body) as Map<String, dynamic>;
      final status = decoded['status']?.toString() ?? '';
      if (status == 'connected') {
        await _loadConfig();
        if (mounted) {
          NotificationHelper.showSuccess(context, l.waAlreadyConnected);
        }
      } else if (status == 'qr' && decoded['qr'] != null) {
        setState(() => _qrImageBase64 = decoded['qr'].toString());
        _startWAPolling(); // Start polling for connection status
      } else if (status == 'waiting') {
        _startWAPolling(); // Start polling for connection status
        if (mounted) {
          NotificationHelper.showSuccess(
            context,
            l.waWaitingLink,
          );
        }
      } else {
        if (mounted) {
          NotificationHelper.showError(
            context,
            decoded['error']?.toString() ?? l.waQRFetchFailed,
          );
        }
      }
    } on ApiException catch (e) {
      if (mounted) NotificationHelper.showError(context, e.message);
    } catch (e) {
      if (mounted) NotificationHelper.showError(context, l.waUnexpectedError);
    } finally {
      if (mounted) setState(() => _loadingQR = false);
    }
  }

  Future<void> _logoutWhatsapp() async {
    final l = AppLocalizations.of(context);
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l.waDisconnectConfirmTitle),
        content: Text(l.waDisconnectConfirmMessage),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(l.cancel),
          ),
          ElevatedButton(
            style: ElevatedButton.styleFrom(
              backgroundColor: AppTheme.danger,
            ),
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(l.waDisconnectBtn),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await widget.api.post('/radius/api/whatsapp/logout');
      await _loadConfig();
      setState(() => _qrImageBase64 = null);
      if (mounted) {
        NotificationHelper.showSuccess(context, l.waDisconnected);
      }
    } on ApiException catch (e) {
      if (mounted) NotificationHelper.showError(context, e.message);
    }
  }

  // ─── Templates ─────────────────────────────────────────────────────────────

  Future<void> _saveTemplate(_WATemplate template) async {
    final l = AppLocalizations.of(context);
    try {
      await widget.api.post(
        '/radius/api/whatsapp/templates',
        body: {
          'template_key': template.key,
          'template_text': template.text,
        },
      );
      if (mounted) {
        final labels = _templateLabels(l);
        NotificationHelper.showSuccess(
          context,
          '${l.waTemplateSaved.split(' ').last} "${labels[template.key] ?? template.key}"',
        );
      }
    } on ApiException catch (e) {
      if (mounted) NotificationHelper.showError(context, e.message);
    }
  }

  // ─── Broadcast ─────────────────────────────────────────────────────────────

  Future<void> _sendBroadcast() async {
    final l = AppLocalizations.of(context);
    final msg = _broadcastCtrl.text.trim();
    if (msg.isEmpty) {
      NotificationHelper.showError(context, l.waWriteMessageFirst);
      return;
    }
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l.waBroadcastConfirmTitle),
        content: Text(
          '${l.waBroadcastConfirmMessage}\n\n"$msg"',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(l.cancel),
          ),
          ElevatedButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(l.waSendNow),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    setState(() => _sendingBroadcast = true);
    try {
      await widget.api.post(
        '/radius/api/whatsapp/broadcast',
        body: {'message': msg},
      );
      _broadcastCtrl.clear();
      if (mounted) {
        NotificationHelper.showSuccess(
          context,
          l.waBroadcastSent,
        );
      }
    } on ApiException catch (e) {
      if (mounted) NotificationHelper.showError(context, e.message);
    } finally {
      if (mounted) setState(() => _sendingBroadcast = false);
    }
  }

  // ─── Test Notification ──────────────────────────────────────────────────────

  Future<void> _sendTestNotification() async {
    final l = AppLocalizations.of(context);
    final phoneCtrl = TextEditingController(
      text: _config?.phoneNumber ?? '',
    );
    await showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l.waTestTitle),
        content: TextField(
          controller: phoneCtrl,
          keyboardType: TextInputType.phone,
          decoration: InputDecoration(
            labelText: l.waPhoneNumberLabel,
            hintText: '9647700000000',
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: Text(l.cancel),
          ),
          ElevatedButton(
            onPressed: () async {
              final phone = phoneCtrl.text.trim();
              if (phone.isEmpty) {
                NotificationHelper.showError(ctx, l.waEnterPhone);
                return;
              }
              Navigator.of(ctx).pop();
              try {
                await widget.api.post(
                  '/radius/api/whatsapp/test',
                  body: {
                    'phone': phone,
                    'message': l.waTestMessage,
                  },
                );
                if (mounted) {
                  NotificationHelper.showSuccess(
                    context,
                    l.waTestSent,
                  );
                }
              } on ApiException catch (e) {
                if (mounted) NotificationHelper.showError(context, e.message);
              } finally {
              }
            },
            child: Text(l.waSendTestBtn),
          ),
        ],
      ),
    );
  }

  Future<void> _sendDebtReminder() async {
    final l = AppLocalizations.of(context);
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l.waDebtReminderConfirmTitle),
        content: Text(l.waDebtReminderConfirmMessage),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(l.cancel),
          ),
          ElevatedButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(l.waSendBtn),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    setState(() => _sendingDebt = true);
    try {
      await widget.api.post('/radius/api/whatsapp/send-debt-reminder');
      if (mounted) {
        NotificationHelper.showSuccess(
          context,
          l.waDebtReminderSent,
        );
      }
    } on ApiException catch (e) {
      if (mounted) NotificationHelper.showError(context, e.message);
    } finally {
      if (mounted) setState(() => _sendingDebt = false);
    }
  }

  // ─── Build ─────────────────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return Scaffold(
      body: Column(
        children: [
          // ── Header
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 16, 16, 0),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        l.waTitle,
                        style: Theme.of(context).textTheme.headlineSmall
                            ?.copyWith(fontWeight: FontWeight.w900),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        l.waSubtitle,
                        style: const TextStyle(color: AppTheme.textSecondary),
                      ),
                    ],
                  ),
                ),
                IconButton(
                  tooltip: l.refresh,
                  onPressed: () {
                    _loadConfig();
                    _loadTemplates();
                    setState(() => _qrImageBase64 = null);
                  },
                  icon: const Icon(Icons.refresh),
                ),
              ],
            ),
          ),
          // ── Tabs
          Container(
            margin: const EdgeInsets.fromLTRB(16, 10, 16, 0),
            decoration: BoxDecoration(
              color: AppTheme.surfaceMuted,
              borderRadius: BorderRadius.circular(12),
              border: Border.all(color: AppTheme.border),
            ),
            child: TabBar(
              controller: _tabs,
              indicatorSize: TabBarIndicatorSize.tab,
              indicator: BoxDecoration(
                color: AppTheme.primary,
                borderRadius: BorderRadius.circular(12),
              ),
              labelColor: Colors.white,
              unselectedLabelColor: AppTheme.textSecondary,
              dividerColor: Colors.transparent,
              tabs: [
                Tab(text: l.waTabSettings),
                Tab(text: l.waTabTemplates),
                Tab(text: l.waTabBroadcast),
              ],
            ),
          ),
          const SizedBox(height: 10),
          // ── Tab Views
          Expanded(
            child: TabBarView(
              controller: _tabs,
              children: [
                _buildConfigTab(),
                _buildTemplatesTab(),
                _buildBroadcastTab(),
              ],
            ),
          ),
        ],
      ),
    );
  }

  // ─── Tab 1: Settings & QR ─────────────────────────────────────────────────

  Widget _buildConfigTab() {
    final l = AppLocalizations.of(context);
    if (_loadingConfig) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_configError != null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.error_outline, size: 48, color: AppTheme.danger),
            const SizedBox(height: 12),
            Text(_configError!, textAlign: TextAlign.center),
            const SizedBox(height: 12),
            ElevatedButton.icon(
              onPressed: _loadConfig,
              icon: const Icon(Icons.refresh),
              label: Text(l.waRetry),
            ),
          ],
        ),
      );
    }

    final cfg = _config;
    final isConnected = cfg?.status == 'connected';
    final statusColor = isConnected ? AppTheme.success : AppTheme.danger;
    final statusText = isConnected
        ? l.waStatusConnected
        : cfg?.status == 'waiting'
        ? l.waStatusWaiting
        : l.waStatusDisconnected;

    return RefreshIndicator(
      onRefresh: _loadConfig,
      child: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          // ── Status Card
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                children: [
                  Container(
                    width: 44,
                    height: 44,
                    decoration: BoxDecoration(
                      color: statusColor.withValues(alpha: 0.12),
                      borderRadius: BorderRadius.circular(12),
                    ),
                    child: Icon(
                      isConnected ? Icons.check_circle : Icons.wifi_off,
                      color: statusColor,
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          statusText,
                          style: TextStyle(
                            fontWeight: FontWeight.w800,
                            color: statusColor,
                            fontSize: 16,
                          ),
                        ),
                        if ((cfg?.phoneNumber ?? '').isNotEmpty)
                          Text(
                            cfg!.phoneNumber,
                            style: const TextStyle(
                              color: AppTheme.textSecondary,
                            ),
                          ),
                      ],
                    ),
                  ),
                  ElevatedButton.icon(
                    onPressed: _showConfigDialog,
                    icon: const Icon(Icons.edit, size: 16),
                    label: Text(l.waEdit),
                  ),
                ],
              ),
            ),
          ),
          const SizedBox(height: 12),
          // ── Reminder info
          if (cfg != null)
            Card(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      l.waAutoReminder,
                      style: const TextStyle(
                        fontWeight: FontWeight.w800,
                        fontSize: 15,
                      ),
                    ),
                    const SizedBox(height: 10),
                    Row(
                      children: [
                        Icon(
                          cfg.reminderEnabled == 1
                              ? Icons.notifications_active
                              : Icons.notifications_off,
                          color: cfg.reminderEnabled == 1
                              ? AppTheme.success
                              : AppTheme.textSecondary,
                        ),
                        const SizedBox(width: 10),
                        Text(
                          cfg.reminderEnabled == 1
                              ? l.waReminderEnabledText.replaceAll('{hours}', cfg.reminderHours.toString())
                              : l.waReminderDisabled,
                          style: TextStyle(
                            color: cfg.reminderEnabled == 1
                                ? AppTheme.textDark
                                : AppTheme.textSecondary,
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ),
          const SizedBox(height: 20),
          // ── QR Section
          if (!isConnected) ...[
            Text(
              l.waQRLinkTitle,
              style: const TextStyle(fontWeight: FontWeight.w800, fontSize: 15),
            ),
            const SizedBox(height: 8),
            Text(
              l.waQRLinkDescription,
              style: const TextStyle(color: AppTheme.textSecondary),
            ),
            const SizedBox(height: 12),
            SizedBox(
              width: double.infinity,
              child: ElevatedButton.icon(
                onPressed: _loadingQR ? null : _fetchQR,
                icon: _loadingQR
                    ? const SizedBox(
                        width: 18,
                        height: 18,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: Colors.white,
                        ),
                      )
                    : const Icon(Icons.qr_code),
                label: Text(_loadingQR ? l.waQRFetching : l.waQRFetchButton),
              ),
            ),
            if (_qrImageBase64 != null) ...[
              const SizedBox(height: 16),
              Center(
                child: Container(
                  decoration: BoxDecoration(
                    border: Border.all(color: AppTheme.border, width: 2),
                    borderRadius: BorderRadius.circular(12),
                  ),
                  padding: const EdgeInsets.all(12),
                  child: Builder(builder: (context) {
                    try {
                      final raw = _qrImageBase64!;
                      final base64Data = raw.contains(',')
                          ? raw.split(',').last
                          : raw;
                      final bytes = base64Decode(base64Data);
                      return Image.memory(
                        Uint8List.fromList(bytes),
                        width: 220,
                        height: 220,
                        fit: BoxFit.contain,
                      );
                    } catch (_) {
                      return Text(l.waQRError);
                    }
                  }),
                ),
              ),
              const SizedBox(height: 8),
              Center(
                child: Text(
                  l.waQRInstructions,
                  textAlign: TextAlign.center,
                  style: const TextStyle(color: AppTheme.textSecondary, fontSize: 13),
                ),
              ),
            ],
          ],
          if (isConnected) ...[
            const SizedBox(height: 12),
            SizedBox(
              width: double.infinity,
              child: OutlinedButton.icon(
                style: OutlinedButton.styleFrom(
                  foregroundColor: AppTheme.danger,
                  side: const BorderSide(color: AppTheme.danger),
                ),
                onPressed: _logoutWhatsapp,
                icon: const Icon(Icons.logout),
                label: Text(l.waDisconnectBtn2),
              ),
            ),
          ],
        ],
      ),
    );
  }

  // ─── Tab 2: Templates ─────────────────────────────────────────────────────

  Widget _buildTemplatesTab() {
    final l = AppLocalizations.of(context);
    if (_loadingTemplates) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_templateError != null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.error_outline, size: 48, color: AppTheme.danger),
            const SizedBox(height: 12),
            Text(_templateError!, textAlign: TextAlign.center),
            const SizedBox(height: 12),
            ElevatedButton.icon(
              onPressed: _loadTemplates,
              icon: const Icon(Icons.refresh),
              label: Text(l.waRetry),
            ),
          ],
        ),
      );
    }

    if (_templates.isEmpty) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.message_outlined, size: 52, color: AppTheme.textSecondary),
            const SizedBox(height: 12),
            Text(
              l.noProfiles,
              style: const TextStyle(color: AppTheme.textSecondary),
            ),
            const SizedBox(height: 12),
            ElevatedButton.icon(
              onPressed: _loadTemplates,
              icon: const Icon(Icons.refresh),
              label: Text(l.refresh),
            ),
          ],
        ),
      );
    }

    return RefreshIndicator(
      onRefresh: _loadTemplates,
      child: ListView.separated(
        padding: const EdgeInsets.all(16),
        itemCount: _templates.length,
        separatorBuilder: (_, __) => const SizedBox(height: 12),
        itemBuilder: (context, i) => _TemplateCard(
          template: _templates[i],
          onSave: _saveTemplate,
        ),
      ),
    );
  }

  // ─── Tab 3: Broadcast ─────────────────────────────────────────────────────

  Widget _buildBroadcastTab() {
    final l = AppLocalizations.of(context);
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        // ── Broadcast
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    const Icon(Icons.campaign, color: AppTheme.primary),
                    const SizedBox(width: 8),
                    Text(
                      l.waBroadcastTitle,
                      style: const TextStyle(
                        fontWeight: FontWeight.w800,
                        fontSize: 16,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 6),
                Text(
                  l.waBroadcastDescription,
                  style: const TextStyle(color: AppTheme.textSecondary, fontSize: 13),
                ),
                const SizedBox(height: 14),
                TextField(
                  controller: _broadcastCtrl,
                  maxLines: 5,
                  decoration: InputDecoration(
                    hintText: l.waBroadcastHint,
                    alignLabelWithHint: true,
                    counter: Text(
                      '${_broadcastCtrl.text.length} ${l.waCharactersCount}',
                      style: TextStyle(
                        color: _broadcastCtrl.text.length > 1000
                            ? AppTheme.danger
                            : AppTheme.textSecondary,
                        fontSize: 12,
                      ),
                    ),
                  ),
                ),
                const SizedBox(height: 12),
                SizedBox(
                  width: double.infinity,
                  child: ElevatedButton.icon(
                    onPressed: _sendingBroadcast ? null : _sendBroadcast,
                    icon: _sendingBroadcast
                        ? const SizedBox(
                            width: 18,
                            height: 18,
                            child: CircularProgressIndicator(
                              strokeWidth: 2,
                              color: Colors.white,
                            ),
                          )
                        : const Icon(Icons.send),
                    label: Text(
                      _sendingBroadcast ? l.waSending : l.waBroadcastSendBtn,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 14),
        // ── Debt Reminder
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    const Icon(Icons.money_off, color: AppTheme.warning),
                    const SizedBox(width: 8),
                    Text(
                      l.waDebtReminderTitle,
                      style: const TextStyle(
                        fontWeight: FontWeight.w800,
                        fontSize: 16,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 6),
                Text(
                  l.waDebtReminderDescription,
                  style: const TextStyle(color: AppTheme.textSecondary, fontSize: 13),
                ),
                const SizedBox(height: 14),
                SizedBox(
                  width: double.infinity,
                  child: ElevatedButton.icon(
                    onPressed: _sendingDebt ? null : _sendDebtReminder,
                    icon: _sendingDebt
                        ? const SizedBox(
                            width: 18,
                            height: 18,
                            child: CircularProgressIndicator(
                              strokeWidth: 2,
                              color: Colors.white,
                            ),
                          )
                        : const Icon(Icons.send),
                    label: Text(
                      _sendingDebt ? l.waSendingDebtReminder : l.waDebtReminderBtn,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 14),
        // ── Test Notification
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    const Icon(Icons.science, color: AppTheme.primary),
                    const SizedBox(width: 8),
                    Text(
                      l.waTestTitle,
                      style: const TextStyle(
                        fontWeight: FontWeight.w800,
                        fontSize: 16,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 6),
                Text(
                  l.waEnterPhone,
                  style: const TextStyle(color: AppTheme.textSecondary, fontSize: 13),
                ),
                const SizedBox(height: 14),
                SizedBox(
                  width: double.infinity,
                  child: OutlinedButton.icon(
                    onPressed: _sendTestNotification,
                    icon: const Icon(Icons.send),
                    label: Text(l.waSendTestBtn),
                  ),
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }
}

// ─── Template Card Widget ───────────────────────────────────────────────────

class _TemplateCard extends StatefulWidget {
  const _TemplateCard({required this.template, required this.onSave});
  final _WATemplate template;
  final Future<void> Function(_WATemplate) onSave;

  @override
  State<_TemplateCard> createState() => _TemplateCardState();
}

class _TemplateCardState extends State<_TemplateCard> {
  late final TextEditingController _ctrl;
  bool _editing = false;
  bool _saving = false;

  @override
  void initState() {
    super.initState();
    _ctrl = TextEditingController(text: widget.template.text);
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    _templateLabels(l)[widget.template.key] ?? widget.template.key,
                    style: const TextStyle(
                      fontWeight: FontWeight.w800,
                      fontSize: 14,
                    ),
                  ),
                ),
                IconButton(
                  icon: Icon(_editing ? Icons.check : Icons.edit, size: 18),
                  tooltip: _editing ? l.save : l.waEdit,
                  onPressed: () async {
                    if (_editing) {
                      setState(() => _saving = true);
                      widget.template.text = _ctrl.text;
                      await widget.onSave(widget.template);
                      setState(() {
                        _saving = false;
                        _editing = false;
                      });
                    } else {
                      setState(() => _editing = true);
                    }
                  },
                ),
              ],
            ),
            const SizedBox(height: 8),
            _editing
                ? TextField(
                    controller: _ctrl,
                    maxLines: 5,
                    decoration: const InputDecoration(
                      border: OutlineInputBorder(),
                      alignLabelWithHint: true,
                    ),
                  )
                : Text(
                    widget.template.text.isEmpty
                        ? '—'
                        : widget.template.text,
                    style: const TextStyle(color: AppTheme.textSecondary),
                  ),
            if (_saving)
              const Padding(
                padding: EdgeInsets.only(top: 8),
                child: LinearProgressIndicator(),
              ),
          ],
        ),
      ),
    );
  }
}