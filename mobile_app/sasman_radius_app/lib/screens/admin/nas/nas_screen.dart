import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;

import '../../../config/app_theme.dart';
import '../../../l10n/app_localizations.dart';
import '../../../services/api_service.dart';
import '../../../widgets/notification_helper.dart';

class NasScreen extends StatefulWidget {
  const NasScreen({super.key, required this.api});

  final ApiService api;

  @override
  State<NasScreen> createState() => _NasScreenState();
}

class _NasScreenState extends State<NasScreen>
    with SingleTickerProviderStateMixin {
  late final TabController _tabController;
  late final Future<void> _future = _load();

  List<Map<String, dynamic>> _nasList = [];
  List<Map<String, dynamic>> _bypassList = [];
  bool _bypassEnabled = false;
  bool _loadingBypassState = false;
  String _query = '';
  String _bypassQuery = '';
  int? _editingId;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 3, vsync: this);
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final List<dynamic> results = await Future.wait([
      widget.api.get('/radius/api/nas'),
      widget.api
          .get('/radius/api/bypass')
          .catchError((_) => httpResponseStub()),
    ]);

    final decodedNas = jsonDecode(results[0].body);
    _nasList = _parseList(decodedNas);

    try {
      if (results[1] is http.Response) {
        final decodedBypass = jsonDecode((results[1] as http.Response).body);
        if (decodedBypass is Map) {
          // Global bypass returns {enabled: bool} or similar
          _bypassEnabled = decodedBypass['enabled'] == true;
          _bypassList = _parseList(decodedBypass);
        } else if (decodedBypass is List) {
          _bypassList = _parseList(decodedBypass);
        }
      }
    } catch (_) {}

    setState(() {});
  }

  static dynamic httpResponseStub() {
    return Object();
  }

  List<Map<String, dynamic>> _parseList(dynamic decoded) {
    if (decoded is List) {
      return decoded
          .whereType<Map>()
          .map((e) => Map<String, dynamic>.from(e))
          .toList();
    }
    if (decoded is Map) {
      final list =
          decoded['data'] ??
          decoded['items'] ??
          decoded['nas'] ??
          decoded['bypass'];
      if (list is List) {
        return list
            .whereType<Map>()
            .map((e) => Map<String, dynamic>.from(e))
            .toList();
      }
    }
    return [];
  }

  List<Map<String, dynamic>> get _filteredNas {
    if (_query.isEmpty) return _nasList;
    final q = _query.toLowerCase();
    return _nasList
        .where(
          (item) => item.values.any(
            (value) =>
                value != null && value.toString().toLowerCase().contains(q),
          ),
        )
        .toList();
  }

  List<Map<String, dynamic>> get _filteredBypass {
    if (_bypassQuery.isEmpty) return _bypassList;
    final q = _bypassQuery.toLowerCase();
    return _bypassList
        .where(
          (item) => item.values.any(
            (value) =>
                value != null && value.toString().toLowerCase().contains(q),
          ),
        )
        .toList();
  }

  Future<void> _showNasForm([Map<String, dynamic>? nas]) async {
    final ipCtrl = TextEditingController(text: nas?['ip'] ?? '');
    final nameCtrl = TextEditingController(text: nas?['name'] ?? '');
    final secretCtrl = TextEditingController(text: nas?['secret'] ?? '');
    final profileCtrl = TextEditingController(
      text: nas?['profile_nas_ip'] ?? '',
    );
    bool isGlobal = nas?['is_global'] == true || nas?['is_global'] == 'true';
    final isEdit = nas != null;
    _editingId = nas?['id'] is int ? nas!['id'] as int : null;

    await showDialog<void>(
      context: context,
      builder: (context) {
        return StatefulBuilder(
          builder: (context, setState) {
            final loc = AppLocalizations.of(context);
            return AlertDialog(
              title: Text(isEdit ? loc.editNas : loc.addNas),
              content: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    TextField(
                      controller: ipCtrl,
                      decoration: InputDecoration(labelText: loc.clientIp),
                    ),
                    const SizedBox(height: 10),
                    TextField(
                      controller: nameCtrl,
                      decoration: InputDecoration(labelText: loc.nasName),
                    ),
                    const SizedBox(height: 10),
                    TextField(
                      controller: secretCtrl,
                      decoration: InputDecoration(
                        labelText: loc.radiusSecret,
                      ),
                    ),
                    const SizedBox(height: 10),
                    TextField(
                      controller: profileCtrl,
                      decoration: InputDecoration(
                        labelText: loc.nasIp,
                      ),
                    ),
                    const SizedBox(height: 10),
                    Row(
                      children: [
                        Checkbox(
                          value: isGlobal,
                          onChanged: (value) =>
                              setState(() => isGlobal = value ?? false),
                        ),
                        Expanded(
                          child: Text(loc.visibleToAllAgents),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.of(context).pop(),
                  child: Text(loc.cancel),
                ),
                ElevatedButton(
                  onPressed: () async {
                    final payload = {
                      'ip': ipCtrl.text.trim(),
                      'name': nameCtrl.text.trim(),
                      'secret': secretCtrl.text.trim(),
                      'profile_nas_ip': profileCtrl.text.trim(),
                      'admin_id': 0,
                      'is_global': isGlobal,
                    };
                    try {
                      if (isEdit && _editingId != null) {
                        await widget.api.post(
                          '/radius/api/nas/$_editingId',
                          body: payload,
                        );
                      } else {
                        await widget.api.post('/radius/api/nas', body: payload);
                      }
                      await _load();
                      if (context.mounted) {
                        NotificationHelper.showSuccess(
                          context,
                          isEdit ? loc.nasUpdated : loc.nasAdded,
                        );
                        Navigator.of(context).pop();
                      }
                    } on ApiException catch (e) {
                      if (context.mounted) {
                        NotificationHelper.showError(context, e.message);
                      }
                    }
                  },
                  child: Text(isEdit ? loc.saveChanges2 : loc.addRouter),
                ),
              ],
            );
          },
        );
      },
    );
  }

  Future<void> _deleteNas(String ip) async {
    final loc = AppLocalizations.of(context);
    final confirmed =
        await showDialog<bool>(
          context: context,
          builder: (context) => AlertDialog(
            title: Text(loc.deleteRouter),
            content: Text('${loc.deleteRouterConfirm} $ip?'),
            actions: [
              TextButton(
                onPressed: () => Navigator.of(context).pop(false),
                child: Text(loc.cancel),
              ),
              ElevatedButton(
                onPressed: () => Navigator.of(context).pop(true),
                child: Text(loc.delete),
              ),
            ],
          ),
        ) ??
        false;
    if (!confirmed) {
      return;
    }
    try {
      await widget.api.delete('/radius/api/nas/${Uri.encodeComponent(ip)}');
      await _load();
      if (mounted) {
        NotificationHelper.showSuccess(context, loc.nasDeleted);
      }
    } on ApiException catch (e) {
      if (mounted) {
        NotificationHelper.showError(context, e.message);
      }
    }
  }

  Future<void> _quickSetup() async {
    final loc = AppLocalizations.of(context);
    final confirmed =
        await showDialog<bool>(
          context: context,
          builder: (context) => AlertDialog(
            title: Text(loc.quickSetup),
            content: Text(loc.quickSetupConfirm),
            actions: [
              TextButton(
                onPressed: () => Navigator.of(context).pop(false),
                child: Text(loc.cancel),
              ),
              ElevatedButton(
                onPressed: () => Navigator.of(context).pop(true),
                child: Text(loc.save),
              ),
            ],
          ),
        ) ??
        false;
    if (!confirmed) {
      return;
    }
    try {
      await widget.api.post('/radius/api/nas/quick-setup');
      await _load();
      if (mounted) {
        NotificationHelper.showSuccess(context, loc.quickSetupSuccess);
      }
    } on ApiException catch (e) {
      if (mounted) {
        NotificationHelper.showError(context, e.message);
      }
    }
  }

  Future<void> _connectRouter() async {
    final loc = AppLocalizations.of(context);
    final confirmed =
        await showDialog<bool>(
          context: context,
          builder: (context) => AlertDialog(
            title: Text(loc.autoConnect),
            content: Text(loc.autoConnectConfirm),
            actions: [
              TextButton(
                onPressed: () => Navigator.of(context).pop(false),
                child: Text(loc.cancel),
              ),
              ElevatedButton(
                onPressed: () => Navigator.of(context).pop(true),
                child: Text(loc.save),
              ),
            ],
          ),
        ) ??
        false;
    if (!confirmed) {
      return;
    }
    try {
      await widget.api.post('/radius/api/router/connect');
      await _load();
      if (mounted) {
        NotificationHelper.showSuccess(context, loc.autoConnectSuccess);
      }
    } on ApiException catch (e) {
      if (mounted) {
        NotificationHelper.showError(context, e.message);
      }
    }
  }

  // Bypass List Actions
  Future<void> _showBypassForm() async {
    final ipCtrl = TextEditingController();
    final macCtrl = TextEditingController();
    final nameCtrl = TextEditingController();

    await showDialog<void>(
      context: context,
      builder: (context) {
        final loc2 = AppLocalizations.of(context);
        return AlertDialog(
          title: Text(loc2.addBypassDevice),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextField(
                  controller: ipCtrl,
                  decoration: InputDecoration(labelText: loc2.deviceIp),
                ),
                const SizedBox(height: 10),
                TextField(
                  controller: macCtrl,
                  decoration: const InputDecoration(labelText: 'MAC Address'),
                ),
                const SizedBox(height: 10),
                TextField(
                  controller: nameCtrl,
                  decoration: InputDecoration(
                    labelText: loc2.deviceName,
                  ),
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(context).pop(),
              child: Text(loc2.cancel),
            ),
            ElevatedButton(
              onPressed: () async {
                final payload = {
                  if (ipCtrl.text.trim().isNotEmpty) 'ip': ipCtrl.text.trim(),
                  if (macCtrl.text.trim().isNotEmpty)
                    'mac': macCtrl.text.trim(),
                  'name': nameCtrl.text.trim(),
                };
                try {
                  await widget.api.post('/radius/api/bypass', body: payload);
                  await _load();
                  if (context.mounted) {
                    NotificationHelper.showSuccess(context, loc2.bypassAdded);
                    Navigator.of(context).pop();
                  }
                } on ApiException catch (e) {
                  if (context.mounted) {
                    NotificationHelper.showError(context, e.message);
                  }
                }
              },
              child: Text(loc2.save),
            ),
          ],
        );
      },
    );
  }

  Future<void> _deleteBypass(Map<String, dynamic> item) async {
    final loc = AppLocalizations.of(context);
    final id = item['id'] ?? item['ip'] ?? item['mac'];
    final name = item['name'] ?? item['ip'] ?? item['mac'] ?? '-';
    if (id == null) return;

    final confirmed =
        await showDialog<bool>(
          context: context,
          builder: (context) => AlertDialog(
            title: Text(loc.deleteBypass),
            content: Text('${loc.deleteBypassConfirm} $name?'),
            actions: [
              TextButton(
                onPressed: () => Navigator.of(context).pop(false),
                child: Text(loc.cancel),
              ),
              ElevatedButton(
                onPressed: () => Navigator.of(context).pop(true),
                child: Text(loc.delete),
              ),
            ],
          ),
        ) ??
        false;
    if (!confirmed) {
      return;
    }
    try {
      await widget.api.delete('/radius/api/bypass/$id');
      await _load();
      if (mounted) {
        NotificationHelper.showSuccess(context, loc.bypassDeleted);
      }
    } on ApiException catch (e) {
      if (mounted) {
        NotificationHelper.showError(context, e.message);
      }
    }
  }

  // ─── Global Bypass Toggle ──────────────────────────────────────────────────

  Future<void> _toggleGlobalBypass(bool enable) async {
    final loc = AppLocalizations.of(context);
    final confirmMsg = enable
        ? loc.bypassEnableWarning
        : loc.bypassDisableConfirm;

    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(enable ? loc.bypassEnableTitle : loc.bypassDisableTitle),
        content: Text(confirmMsg),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(loc.cancel),
          ),
          ElevatedButton(
            style: ElevatedButton.styleFrom(
              backgroundColor: enable ? AppTheme.danger : AppTheme.success,
            ),
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(enable ? loc.enableBypass : loc.disableBypass),
          ),
        ],
      ),
    ) ?? false;

    if (!confirmed) return;

    setState(() => _loadingBypassState = true);
    try {
      await widget.api.post('/radius/api/bypass', body: {'enabled': enable});
      _bypassEnabled = enable;
      if (mounted) {
        NotificationHelper.showSuccess(
          context,
          enable ? loc.bypassEnableSuccess : loc.bypassDisableSuccess,
        );
      }
    } on ApiException catch (e) {
      if (mounted) NotificationHelper.showError(context, e.message);
    } finally {
      if (mounted) setState(() => _loadingBypassState = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final loc = AppLocalizations.of(context);
    return FutureBuilder<void>(
      future: _future,
      builder: (context, snapshot) {
        return Scaffold(
          appBar: TabBar(
            controller: _tabController,
            tabs: [
              Tab(text: loc.tabNasDevices),
              Tab(text: loc.tabGlobalBypass),
              Tab(text: loc.tabBypassList),
            ],
          ),
          body: TabBarView(
            controller: _tabController,
            children: [
              _buildNasTab(snapshot),
              _buildGlobalBypassTab(),
              _buildBypassTab(snapshot),
            ],
          ),
        );
      },
    );
  }

  Widget _buildGlobalBypassTab() {
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        // ── Status Card
        Container(
          padding: const EdgeInsets.all(20),
          decoration: BoxDecoration(
            gradient: _bypassEnabled
                ? const LinearGradient(colors: [Color(0xFFDC2626), Color(0xFFEF4444)])
                : const LinearGradient(colors: [Color(0xFF059669), Color(0xFF10B981)]),
            borderRadius: BorderRadius.circular(16),
            boxShadow: [
              BoxShadow(
                color: (_bypassEnabled ? AppTheme.danger : AppTheme.success)
                    .withValues(alpha: 0.3),
                blurRadius: 12,
                offset: const Offset(0, 4),
              ),
            ],
          ),
          child: Column(
            children: [
              Icon(
                _bypassEnabled ? Icons.gpp_bad_rounded : Icons.gpp_good_rounded,
                size: 64,
                color: Colors.white.withValues(alpha: 0.9),
              ),
              const SizedBox(height: 16),
              Text(
                _bypassEnabled
                    ? AppLocalizations.of(context).globalBypassEnabled
                    : AppLocalizations.of(context).globalBypassDisabled,
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 20,
                  fontWeight: FontWeight.w900,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 8),
              Text(
                _bypassEnabled
                    ? AppLocalizations.of(context).globalBypassDescEnabled
                    : AppLocalizations.of(context).globalBypassDescDisabled,
                style: TextStyle(
                  color: Colors.white.withValues(alpha: 0.85),
                  fontSize: 14,
                ),
                textAlign: TextAlign.center,
              ),
            ],
          ),
        ),
        const SizedBox(height: 20),
        // ── Toggle Button
        SizedBox(
          width: double.infinity,
          child: _loadingBypassState
              ? const Center(child: CircularProgressIndicator())
              : ElevatedButton.icon(
                  style: ElevatedButton.styleFrom(
                    backgroundColor: _bypassEnabled ? const Color(0xFF374151) : AppTheme.danger,
                    foregroundColor: Colors.white,
                    padding: const EdgeInsets.symmetric(vertical: 16),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                  onPressed: () => _toggleGlobalBypass(!_bypassEnabled),
                  icon: Icon(_bypassEnabled ? Icons.lock : Icons.lock_open),
                  label: Text(
                    _bypassEnabled
                        ? AppLocalizations.of(context).disableGlobalBypass
                        : AppLocalizations.of(context).enableGlobalBypass,
                    style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w800),
                  ),
                ),
        ),
        const SizedBox(height: 20),
        // ── Warning Card
        Builder(
          builder: (context) {
            final wl = AppLocalizations.of(context);
            return Container(
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                color: AppTheme.warning.withValues(alpha: 0.08),
                borderRadius: BorderRadius.circular(12),
                border: Border.all(color: AppTheme.warning.withValues(alpha: 0.3)),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      const Icon(Icons.warning_amber_rounded, color: AppTheme.warning, size: 20),
                      const SizedBox(width: 8),
                      Text(
                        wl.bypassWarningImportant,
                        style: const TextStyle(fontWeight: FontWeight.w800, color: AppTheme.warning),
                      ),
                    ],
                  ),
                  const SizedBox(height: 8),
                  Text(
                    wl.bypassWarningText,
                    style: const TextStyle(fontSize: 13, color: AppTheme.textSecondary, height: 1.8),
                  ),
                ],
              ),
            );
          },
        ),
      ],
    );
  }

  Widget _buildNasTab(AsyncSnapshot<void> snapshot) {
    final rows = _filteredNas;
    final loc = AppLocalizations.of(context);
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.all(16),
          child: LayoutBuilder(
            builder: (context, constraints) {
              final title = Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    loc.navNas,
                    style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                      fontWeight: FontWeight.w900,
                    ),
                  ),
                  const SizedBox(height: 6),
                  Text(loc.nasSubtitle),
                ],
              );
              final actions = Wrap(
                spacing: 6,
                runSpacing: 6,
                children: [
                  ElevatedButton.icon(
                    onPressed: () => _showNasForm(),
                    icon: const Icon(Icons.add),
                    label: Text(loc.addNasDevice),
                  ),
                  OutlinedButton.icon(
                    onPressed: _quickSetup,
                    icon: const Icon(Icons.flash_on),
                    label: Text(loc.quickSetupBtn),
                  ),
                  OutlinedButton.icon(
                    onPressed: _connectRouter,
                    icon: const Icon(Icons.link),
                    label: Text(loc.connectRouterBtn),
                  ),
                ],
              );

              if (constraints.maxWidth < 560) {
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [title, const SizedBox(height: 12), actions],
                );
              }

              return Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(child: title),
                  const SizedBox(width: 12),
                  Flexible(child: actions),
                ],
              );
            },
          ),
        ),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16),
          child: TextField(
            decoration: InputDecoration(
              labelText: loc.searchNas,
              prefixIcon: const Icon(Icons.search),
            ),
            onChanged: (value) => setState(() => _query = value),
          ),
        ),
        const SizedBox(height: 12),
        Expanded(
          child: RefreshIndicator(
            onRefresh: _load,
            child: snapshot.connectionState == ConnectionState.waiting
                ? const Center(child: CircularProgressIndicator())
                : rows.isEmpty
                ? Center(child: Text(loc.noNas))
                : ListView.builder(
                    padding: const EdgeInsets.all(16),
                    itemCount: rows.length,
                    itemBuilder: (context, index) {
                      final nas = rows[index];
                      return Card(
                        margin: const EdgeInsets.only(bottom: 12),
                        child: ListTile(
                          title: Text(nas['ip']?.toString() ?? '-'),
                          subtitle: Text(
                            'اسم: ${nas['name'] ?? '-'} • NAS-IP: ${nas['profile_nas_ip'] ?? '-'}',
                          ),
                          trailing: Wrap(
                            spacing: 6,
                            children: [
                              IconButton(
                                onPressed: () => _showNasForm(nas),
                                icon: const Icon(Icons.edit),
                              ),
                              IconButton(
                                onPressed: () =>
                                    _deleteNas(nas['ip']?.toString() ?? ''),
                                icon: const Icon(
                                  Icons.delete,
                                  color: Colors.red,
                                ),
                              ),
                            ],
                          ),
                        ),
                      );
                    },
                  ),
          ),
        ),
      ],
    );
  }

  Widget _buildBypassTab(AsyncSnapshot<void> snapshot) {
    final rows = _filteredBypass;
    final loc = AppLocalizations.of(context);
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.all(16),
          child: LayoutBuilder(
            builder: (context, constraints) {
              final title = Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    loc.bypassDevicesTitle,
                    style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                      fontWeight: FontWeight.w900,
                    ),
                  ),
                  const SizedBox(height: 6),
                  Text(loc.bypassDevicesSubtitle),
                ],
              );
              final action = ElevatedButton.icon(
                onPressed: _showBypassForm,
                icon: const Icon(Icons.add),
                label: Text(loc.addDevice),
              );

              if (constraints.maxWidth < 520) {
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [title, const SizedBox(height: 12), action],
                );
              }

              return Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(child: title),
                  const SizedBox(width: 12),
                  action,
                ],
              );
            },
          ),
        ),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16),
          child: TextField(
            decoration: InputDecoration(
              labelText: loc.searchBypassList,
              prefixIcon: const Icon(Icons.search),
            ),
            onChanged: (value) => setState(() => _bypassQuery = value),
          ),
        ),
        const SizedBox(height: 12),
        Expanded(
          child: RefreshIndicator(
            onRefresh: _load,
            child: snapshot.connectionState == ConnectionState.waiting
                ? const Center(child: CircularProgressIndicator())
                : rows.isEmpty
                ? Center(child: Text(loc.noBypassDevices))
                : ListView.builder(
                    padding: const EdgeInsets.all(16),
                    itemCount: rows.length,
                    itemBuilder: (context, index) {
                      final item = rows[index];
                      return Card(
                        margin: const EdgeInsets.only(bottom: 12),
                        child: ListTile(
                          title: Text(item['name']?.toString() ?? '-'),
                          subtitle: Text(
                            'IP: ${item['ip'] ?? '-'} • MAC: ${item['mac'] ?? '-'}',
                          ),
                          trailing: IconButton(
                            onPressed: () => _deleteBypass(item),
                            icon: const Icon(Icons.delete, color: Colors.red),
                          ),
                        ),
                      );
                    },
                  ),
          ),
        ),
      ],
    );
  }
}
