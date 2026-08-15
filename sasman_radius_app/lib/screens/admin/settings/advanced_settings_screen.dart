import 'dart:convert';
import 'dart:io';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:path_provider/path_provider.dart';

import '../../../config/app_theme.dart';
import '../../../l10n/app_localizations.dart';
import '../../../services/api_service.dart';
import '../../../widgets/notification_helper.dart';

class AdvancedSettingsScreen extends StatefulWidget {
  const AdvancedSettingsScreen({super.key, required this.api});

  final ApiService api;

  @override
  State<AdvancedSettingsScreen> createState() => _AdvancedSettingsScreenState();
}

class _AdvancedSettingsScreenState extends State<AdvancedSettingsScreen>
    with SingleTickerProviderStateMixin {
  late final TabController _tabController;
  late final Future<void> _future = _loadData();

  // License Status
  bool _licenseValid = false;
  String _licenseStatus = 'inactive';
  String _licenseExpiry = '';
  String _licenseSerial = '';
  final _licenseKeyCtrl = TextEditingController();

  // Cloudflare Tunnel URL
  String _cloudflareUrl = '';

  // Blackout Configuration
  bool _blackoutEnabled = false;
  String _blackoutStart = '00:00';
  String _blackoutEnd = '06:00';
  List<String> _blackoutDates = [];
  List<String> _blackoutExceptions = [];
  final _newExceptionCtrl = TextEditingController();

  // Telegram Backup Config
  bool _telegramEnabled = false;
  final _telegramTokenCtrl = TextEditingController();
  final _telegramChatCtrl = TextEditingController();

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 3, vsync: this);
  }

  @override
  void dispose() {
    _tabController.dispose();
    _licenseKeyCtrl.dispose();
    _newExceptionCtrl.dispose();
    _telegramTokenCtrl.dispose();
    _telegramChatCtrl.dispose();
    super.dispose();
  }

  Future<void> _loadData() async {
    final loc = AppLocalizations.of(context);
    final results = await Future.wait([
      widget.api.get('/radius/api/license/status').catchError((_) => httpResponseStub()),
      widget.api.get('/radius/api/auth/cloudflared/url').catchError((_) => httpResponseStub()),
      widget.api.get('/radius/api/auth/shutdown/config').catchError((_) => httpResponseStub()),
      widget.api.get('/radius/api/auth/backup/telegram').catchError((_) => httpResponseStub()),
    ]);

    // License
    try {
      final license = jsonDecode(results[0].body);
      _licenseValid = license['valid'] == true;
      _licenseStatus = _licenseValid ? 'active' : 'inactive';
      // backend returns 'expires', fallback to 'expires_at'
      _licenseExpiry = license['expires']?.toString() ?? license['expires_at']?.toString() ?? '';
      _licenseSerial = license['serial']?.toString() ?? '';
    } catch (_) {}

    // Cloudflared
    try {
      final cloudflared = jsonDecode(results[1].body);
      _cloudflareUrl = cloudflared['url']?.toString() ?? loc.advNotConnected;
    } catch (_) {
      _cloudflareUrl = loc.advNotConnected;
    }

    // Blackout
    try {
      final blackout = jsonDecode(results[2].body);
      _blackoutEnabled = blackout['enabled'] == true || blackout['enabled'] == 1;
      _blackoutStart = blackout['start_time']?.toString() ?? '00:00';
      _blackoutEnd = blackout['end_time']?.toString() ?? '06:00';
      if (blackout['dates'] is List) {
        _blackoutDates = List<String>.from(blackout['dates']);
      }
      if (blackout['exceptions'] is List) {
        _blackoutExceptions = List<String>.from(blackout['exceptions']);
      }
    } catch (_) {}

    // Telegram
    try {
      final telegram = jsonDecode(results[3].body);
      _telegramEnabled = telegram['enabled'] == true || telegram['enabled'] == 1;
      _telegramTokenCtrl.text = telegram['token']?.toString() ?? '';
      _telegramChatCtrl.text = telegram['chat_id']?.toString() ?? '';
    } catch (_) {}

    setState(() {});
  }

  static dynamic httpResponseStub() {
    return Object(); 
  }

  // Licensing Actions
  Future<void> _activateLicense() async {
    final loc = AppLocalizations.of(context);
    final key = _licenseKeyCtrl.text.trim();
    if (key.isEmpty) {
      NotificationHelper.showError(context, loc.advEnterLicenseKey);
      return;
    }
    try {
      await widget.api.post('/radius/api/license/activate', body: {'key': key});
      if (mounted) NotificationHelper.showSuccess(context, loc.advLicenseActivated);
      _licenseKeyCtrl.clear();
      await _loadData();
    } on ApiException catch (e) {
      if (mounted) NotificationHelper.showError(context, e.message);
    }
  }

  // Blackout Scheduler Actions
  Future<void> _saveBlackoutConfig() async {
    final loc = AppLocalizations.of(context);
    try {
      await widget.api.post('/radius/api/auth/shutdown/config', body: {
        'enabled': _blackoutEnabled,
        'start_time': _blackoutStart,
        'end_time': _blackoutEnd,
        'dates': _blackoutDates,
        'exceptions': _blackoutExceptions,
      });
      if (mounted) NotificationHelper.showSuccess(context, loc.advBlackoutSaved);
    } on ApiException catch (e) {
      if (mounted) NotificationHelper.showError(context, e.message);
    }
  }

  // Backup & Restore
  Future<void> _downloadBackup() async {
    final loc = AppLocalizations.of(context);
    try {
      final res = await widget.api.getFile('/radius/api/auth/backup');
      final bytes = res.bodyBytes;
      if (bytes.isEmpty) {
        if (mounted) NotificationHelper.showError(context, loc.advBackupEmpty);
        return;
      }

      // Build a timestamped filename
      final now = DateTime.now();
      final stamp = '${now.year}${now.month.toString().padLeft(2,'0')}${now.day.toString().padLeft(2,'0')}_${now.hour.toString().padLeft(2,'0')}${now.minute.toString().padLeft(2,'0')}';
      final filename = 'sasman_backup_$stamp.sqlite';

      final savedPath = await _saveFileToDevice(bytes, filename);
      if (!mounted) return;
      if (savedPath != null) {
        NotificationHelper.showSuccess(context, '${loc.advBackupSaved}\n$savedPath');
      } else {
        NotificationHelper.showError(context, loc.advSaveCanceled);
      }
    } on ApiException catch (e) {
      if (mounted) NotificationHelper.showError(context, e.message);
    } catch (e) {
      if (mounted) NotificationHelper.showError(context, '${loc.advDownloadFailed}: $e');
    }
  }

  Future<void> _restoreBackup() async {
    final loc = AppLocalizations.of(context);
    final picked = await FilePicker.platform.pickFiles(type: FileType.any);
    if (picked == null || picked.files.isEmpty) return;

    final file = picked.files.first;
    final bytes = file.bytes;
    if (bytes == null) {
      NotificationHelper.showError(context, loc.advCannotReadFile);
      return;
    }

    try {
      await widget.api.postMultipart(
        '/radius/api/auth/restore',
        'backup',
        bytes,
        file.name,
      );
      if (mounted) NotificationHelper.showSuccess(context, loc.advRestoreSuccess);
      await _loadData();
    } on ApiException catch (e) {
      if (mounted) NotificationHelper.showError(context, e.message);
    }
  }

  // Telegram settings
  Future<void> _saveTelegramConfig() async {
    final loc = AppLocalizations.of(context);
    try {
      await widget.api.post('/radius/api/auth/backup/telegram', body: {
        'enabled': _telegramEnabled,
        'token': _telegramTokenCtrl.text.trim(),
        'chat_id': _telegramChatCtrl.text.trim(),
      });
      if (mounted) NotificationHelper.showSuccess(context, loc.advTelegramSaved);
    } on ApiException catch (e) {
      if (mounted) NotificationHelper.showError(context, e.message);
    }
  }

  Future<void> _testTelegramBackup() async {
    final loc = AppLocalizations.of(context);
    try {
      await widget.api.post('/radius/api/auth/backup/telegram/test');
      if (mounted) NotificationHelper.showSuccess(context, loc.advTelegramTestSent);
    } on ApiException catch (e) {
      if (mounted) NotificationHelper.showError(context, e.message);
    }
  }

  // Migration & Excel
  Future<void> _migrateFromSAS4() async {
    final loc = AppLocalizations.of(context);
    final confirmed = await showDialog<bool>(
          context: context,
          builder: (context) {
            final loc = AppLocalizations.of(context);
            return AlertDialog(
            title: Text(loc.advMigrateDialogTitle),
            content: Text(
              loc.advMigrateConfirm,
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.of(context).pop(false),
                child: Text(loc.cancel),
              ),
              ElevatedButton(
                onPressed: () => Navigator.of(context).pop(true),
                child: Text(loc.advStartMigration),
              ),
            ],
          );
          },
        ) ??
        false;
    if (!confirmed) return;

    try {
      await widget.api.post('/radius/api/import/sas4');
      if (mounted) NotificationHelper.showSuccess(context, loc.advMigrationStarted);
    } on ApiException catch (e) {
      if (mounted) NotificationHelper.showError(context, e.message);
    }
  }

  Future<void> _importExcel() async {
    final loc = AppLocalizations.of(context);
    final picked = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: ['xlsx', 'xls'],
    );
    if (picked == null || picked.files.isEmpty) return;

    final file = picked.files.first;
    final bytes = file.bytes;
    if (bytes == null) {
      NotificationHelper.showError(context, loc.advCannotReadExcel);
      return;
    }

    try {
      await widget.api.postMultipart(
        '/radius/api/import/excel',
        'excel',
        bytes,
        file.name,
      );
      if (mounted) NotificationHelper.showSuccess(context, loc.advExcelImported);
      await _loadData();
    } on ApiException catch (e) {
      if (mounted) NotificationHelper.showError(context, e.message);
    }
  }

  Future<void> _exportExcel() async {
    final loc = AppLocalizations.of(context);
    try {
      final res = await widget.api.getFile('/radius/api/export/excel');
      final bytes = res.bodyBytes;
      if (bytes.isEmpty) {
        if (mounted) NotificationHelper.showError(context, loc.advExportEmpty);
        return;
      }

      final now = DateTime.now();
      final stamp = '${now.year}${now.month.toString().padLeft(2,'0')}${now.day.toString().padLeft(2,'0')}_${now.hour.toString().padLeft(2,'0')}${now.minute.toString().padLeft(2,'0')}';
      final filename = 'sasman_users_$stamp.xlsx';

      final savedPath = await _saveFileToDevice(bytes, filename);
      if (!mounted) return;
      if (savedPath != null) {
        NotificationHelper.showSuccess(context, '${loc.advExcelSaved}\n$savedPath');
      } else {
        NotificationHelper.showError(context, loc.advSaveCanceled);
      }
    } on ApiException catch (e) {
      if (mounted) NotificationHelper.showError(context, e.message);
    } catch (e) {
      if (mounted) NotificationHelper.showError(context, '${loc.advExportFailed}: $e');
    }
  }

  /// Saves [bytes] to the device. On Android/iOS saves to the Downloads or
  /// Documents folder automatically. On desktop opens a save-file dialog.
  Future<String?> _saveFileToDevice(List<int> bytes, String filename) async {
    final loc = AppLocalizations.of(context);
    try {
      // Desktop (Windows / Linux / macOS): show save dialog
      if (Platform.isWindows || Platform.isLinux || Platform.isMacOS) {
        final outputPath = await FilePicker.platform.saveFile(
          dialogTitle: loc.advSaveFileDialog,
          fileName: filename,
        );
        if (outputPath == null) return null;
        final file = File(outputPath);
        await file.writeAsBytes(bytes, flush: true);
        return outputPath;
      }

      // Mobile (Android / iOS): save to Downloads or Documents
      Directory dir;
      if (Platform.isAndroid) {
        // getExternalStorageDirectory points to app-specific external storage;
        // prefer the public Downloads folder when available.
        final extDirs = await getExternalStorageDirectories(
          type: StorageDirectory.downloads,
        );
        dir = extDirs != null && extDirs.isNotEmpty
            ? extDirs.first
            : await getApplicationDocumentsDirectory();
      } else {
        dir = await getApplicationDocumentsDirectory();
      }

      final file = File('${dir.path}/$filename');
      await file.writeAsBytes(bytes, flush: true);
      return file.path;
    } catch (e) {
      if (mounted) NotificationHelper.showError(context, '${loc.advSaveError}: $e');
      return null;
    }
  }

  // System Reset
  Future<void> _resetSystem() async {
    final loc = AppLocalizations.of(context);
    final confirmCtrl = TextEditingController();
    final isConfirmValid = await showDialog<bool>(
          context: context,
          builder: (context) {
            final loc = AppLocalizations.of(context);
            return StatefulBuilder(
              builder: (context, setState) {
                return AlertDialog(
                  title: Text(loc.advResetDialogTitle),
                  content: Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        loc.advResetWarning,
                        style: TextStyle(color: AppTheme.danger, fontWeight: FontWeight.bold),
                      ),
                      const SizedBox(height: 12),
                      Text(
                        loc.advResetConfirmHint.replaceAll('{kw}', loc.advResetKeyword),
                      ),
                      const SizedBox(height: 6),
                      TextField(
                        controller: confirmCtrl,
                        onChanged: (_) => setState(() {}),
                        decoration: InputDecoration(hintText: loc.advResetKeywordHint),
                      ),
                    ],
                  ),
                  actions: [
                    TextButton(
                      onPressed: () => Navigator.of(context).pop(false),
                      child: Text(loc.cancel),
                    ),
                    ElevatedButton(
                      style: ElevatedButton.styleFrom(backgroundColor: AppTheme.danger),
                      onPressed: confirmCtrl.text.trim() == loc.advResetKeyword
                          ? () => Navigator.of(context).pop(true)
                          : null,
                      child: Text(loc.advResetNow),
                    ),
                  ],
                );
              },
            );
          },
        ) ??
        false;

    if (!isConfirmValid) return;

    try {
      await widget.api.post('/radius/api/system/reset');
      if (mounted) NotificationHelper.showSuccess(context, loc.advResetSuccess);
      await _loadData();
    } on ApiException catch (e) {
      if (mounted) NotificationHelper.showError(context, e.message);
    }
  }

  // Date selection dialog
  Future<void> _selectDate() async {
    final date = await showDatePicker(
      context: context,
      initialDate: DateTime.now(),
      firstDate: DateTime.now(),
      lastDate: DateTime.now().add(const Duration(days: 365)),
    );
    if (date == null) return;
    final formatted = "${date.year}-${date.month.toString().padLeft(2, '0')}-${date.day.toString().padLeft(2, '0')}";
    if (!_blackoutDates.contains(formatted)) {
      setState(() => _blackoutDates.add(formatted));
    }
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<void>(
      future: _future,
      builder: (context, snapshot) {
        final loc = AppLocalizations.of(context);
        return Scaffold(
          appBar: TabBar(
            controller: _tabController,
            tabs: [
              Tab(text: loc.advTabLicense),
              Tab(text: loc.advTabBackup),
              Tab(text: loc.advTabMigration),
            ],
          ),
          body: TabBarView(
            controller: _tabController,
            children: [
              _buildLicenseBlackoutTab(snapshot),
              _buildBackupTelegramTab(snapshot),
              _buildMigrationResetTab(snapshot),
            ],
          ),
        );
      },
    );
  }

  Widget _buildLicenseBlackoutTab(AsyncSnapshot<void> snapshot) {
    final loc = AppLocalizations.of(context);
    if (snapshot.connectionState == ConnectionState.waiting) {
      return const Center(child: CircularProgressIndicator());
    }

    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        // Cloudflare active URL
        Container(
          padding: const EdgeInsets.all(16),
          decoration: AppTheme.premiumCard(),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Icon(Icons.link_rounded, color: AppTheme.accentLight),
                  SizedBox(width: 8),
                  Text(
                    loc.advCloudflareUrlTitle,
                    style: TextStyle(fontWeight: FontWeight.w900, fontSize: 15),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              SelectableText(
                _cloudflareUrl,
                style: const TextStyle(
                  color: AppTheme.accentLight,
                  fontWeight: FontWeight.bold,
                  fontSize: 14,
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 16),

        // Licensing section
        Text(
          loc.advLicenseSectionTitle,
          style: Theme.of(context)
              .textTheme
              .titleMedium
              ?.copyWith(fontWeight: FontWeight.w900),
        ),
        const SizedBox(height: 8),
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text(loc.licenseCurrentStatus),
                    Text(
                      _licenseStatus == 'active' ? loc.licenseActive : loc.licenseInactive,
                      style: TextStyle(
                        fontWeight: FontWeight.w900,
                        color: _licenseValid ? AppTheme.success : AppTheme.danger,
                      ),
                    ),
                  ],
                ),
                if (_licenseExpiry.isNotEmpty) ...[
                  const SizedBox(height: 6),
                  Text('${loc.advExpiryPrefix} $_licenseExpiry', style: const TextStyle(color: AppTheme.textSecondary)),
                ],
                if (_licenseSerial.isNotEmpty) ...[
                  const SizedBox(height: 4),
                    SelectableText(
                      '${loc.advSerialPrefix} $_licenseSerial',
                      style: const TextStyle(color: AppTheme.textSecondary, fontSize: 12),
                  ),
                ],
                const SizedBox(height: 12),
                Divider(color: AppTheme.border, thickness: 0.5),
                const SizedBox(height: 10),
                Row(
                  children: [
                    Expanded(
                      child: TextField(
                        controller: _licenseKeyCtrl,
                        decoration: InputDecoration(
                          hintText: loc.advLicenseKeyHint,
                        ),
                      ),
                    ),
                    const SizedBox(width: 10),
                    ElevatedButton(
                      onPressed: _activateLicense,
                      child: Text(loc.advActivate),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 20),

        // Scheduled Blackout
        Text(
          loc.advBlackoutTitle,
          style: Theme.of(context)
              .textTheme
              .titleMedium
              ?.copyWith(fontWeight: FontWeight.w900),
        ),
        const SizedBox(height: 8),
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text(loc.advBlackoutEnableLabel),
                    Switch(
                      value: _blackoutEnabled,
                      onChanged: (val) => setState(() => _blackoutEnabled = val),
                    ),
                  ],
                ),
                const SizedBox(height: 10),
                Row(
                  children: [
                    Expanded(
                      child: TextField(
                        controller: TextEditingController(text: _blackoutStart),
                        decoration: InputDecoration(labelText: loc.advBlackoutStartLabel),
                        onTap: () async {
                          final time = await showTimePicker(
                            context: context,
                            initialTime: const TimeOfDay(hour: 0, minute: 0),
                          );
                          if (time != null) {
                            setState(() => _blackoutStart = "${time.hour.toString().padLeft(2, '0')}:${time.minute.toString().padLeft(2, '0')}");
                          }
                        },
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: TextField(
                        controller: TextEditingController(text: _blackoutEnd),
                        decoration: InputDecoration(labelText: loc.advBlackoutEndLabel),
                        onTap: () async {
                          final time = await showTimePicker(
                            context: context,
                            initialTime: const TimeOfDay(hour: 6, minute: 0),
                          );
                          if (time != null) {
                            setState(() => _blackoutEnd = "${time.hour.toString().padLeft(2, '0')}:${time.minute.toString().padLeft(2, '0')}");
                          }
                        },
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 16),
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                     Text(
                      loc.advBlackoutDatesLabel,
                      style: TextStyle(fontWeight: FontWeight.bold),
                    ),
                    TextButton.icon(
                      onPressed: _selectDate,
                      icon: const Icon(Icons.calendar_today, size: 16),
                      label: Text(loc.advAddDate),
                    ),
                  ],
                ),
                if (_blackoutDates.isEmpty)
                  Text(loc.advNoBlackoutDates, style: TextStyle(fontSize: 12, color: AppTheme.textSecondary))
                else
                  Wrap(
                    spacing: 6,
                    runSpacing: 6,
                    children: _blackoutDates.map((date) {
                      return Chip(
                        label: Text(date),
                        onDeleted: () => setState(() => _blackoutDates.remove(date)),
                      );
                    }).toList(),
                  ),
                const SizedBox(height: 16),
                 Text(
                   loc.advBlackoutExceptionsLabel,
                   style: TextStyle(fontWeight: FontWeight.bold),
                 ),
                const SizedBox(height: 6),
                Row(
                  children: [
                    Expanded(
                      child: TextField(
                        controller: _newExceptionCtrl,
                        decoration: InputDecoration(hintText: loc.advExceptionHint),
                      ),
                    ),
                    const SizedBox(width: 8),
                    ElevatedButton(
                      onPressed: () {
                        final val = _newExceptionCtrl.text.trim();
                        if (val.isNotEmpty && !_blackoutExceptions.contains(val)) {
                          setState(() {
                            _blackoutExceptions.add(val);
                            _newExceptionCtrl.clear();
                          });
                        }
                      },
                      child: Text(loc.advAdd),
                    ),
                  ],
                ),
                const SizedBox(height: 10),
                if (_blackoutExceptions.isEmpty)
                  Text(loc.advNoExceptions, style: TextStyle(fontSize: 12, color: AppTheme.textSecondary))
                else
                  Wrap(
                    spacing: 6,
                    runSpacing: 6,
                    children: _blackoutExceptions.map((ex) {
                      return Chip(
                        label: Text(ex),
                        onDeleted: () => setState(() => _blackoutExceptions.remove(ex)),
                      );
                    }).toList(),
                  ),
                const SizedBox(height: 20),
                SizedBox(
                  width: double.infinity,
                  child: ElevatedButton.icon(
                    onPressed: _saveBlackoutConfig,
                    icon: const Icon(Icons.save),
                    label: Text(loc.advSaveBlackout),
                  ),
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildBackupTelegramTab(AsyncSnapshot<void> snapshot) {
    final loc = AppLocalizations.of(context);
    if (snapshot.connectionState == ConnectionState.waiting) {
      return const Center(child: CircularProgressIndicator());
    }

    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        Text(
          loc.advBackupTitle,
          style: Theme.of(context)
              .textTheme
              .titleMedium
              ?.copyWith(fontWeight: FontWeight.w900),
        ),
        const SizedBox(height: 8),
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              children: [
                Row(
                  children: [
                    Expanded(
                      child: OutlinedButton.icon(
                        onPressed: _downloadBackup,
                        icon: const Icon(Icons.download),
                        label: Text(loc.advBackupDownload),
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: ElevatedButton.icon(
                        onPressed: _restoreBackup,
                        icon: const Icon(Icons.upload),
                        label: Text(loc.advRestore),
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 20),
        Text(
          loc.advTelegramTitle,
          style: Theme.of(context)
              .textTheme
              .titleMedium
              ?.copyWith(fontWeight: FontWeight.w900),
        ),
        const SizedBox(height: 8),
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text(loc.advTelegramEnableLabel),
                    Switch(
                      value: _telegramEnabled,
                      onChanged: (val) => setState(() => _telegramEnabled = val),
                    ),
                  ],
                ),
                const SizedBox(height: 10),
                TextField(
                  controller: _telegramTokenCtrl,
                  decoration: InputDecoration(labelText: loc.advTelegramTokenLabel),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _telegramChatCtrl,
                  decoration: InputDecoration(labelText: loc.advTelegramChatLabel),
                ),
                const SizedBox(height: 18),
                Row(
                  children: [
                    Expanded(
                      child: ElevatedButton.icon(
                        onPressed: _saveTelegramConfig,
                        icon: const Icon(Icons.save),
                        label: Text(loc.advSaveSettings),
                      ),
                    ),
                    const SizedBox(width: 12),
                    OutlinedButton.icon(
                      onPressed: _testTelegramBackup,
                      icon: const Icon(Icons.send),
                      label: Text(loc.advTestBackup),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildMigrationResetTab(AsyncSnapshot<void> snapshot) {
    final loc = AppLocalizations.of(context);
    if (snapshot.connectionState == ConnectionState.waiting) {
      return const Center(child: CircularProgressIndicator());
    }

    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        Text(
          loc.advMigrationTitle,
          style: Theme.of(context)
              .textTheme
              .titleMedium
              ?.copyWith(fontWeight: FontWeight.w900),
        ),
        const SizedBox(height: 8),
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              children: [
                SizedBox(
                  width: double.infinity,
                  child: OutlinedButton.icon(
                    onPressed: _migrateFromSAS4,
                    icon: const Icon(Icons.swap_horiz),
                    label: Text(loc.advMigrateSas4),
                  ),
                ),
                const SizedBox(height: 12),
                Row(
                  children: [
                    Expanded(
                      child: OutlinedButton.icon(
                        onPressed: _importExcel,
                        icon: const Icon(Icons.file_open),
                        label: Text(loc.advImportExcel),
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: OutlinedButton.icon(
                        onPressed: _exportExcel,
                        icon: const Icon(Icons.download_for_offline),
                        label: Text(loc.advExportExcel),
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 24),
        Text(
          loc.advDangerZoneTitle,
          style: Theme.of(context)
              .textTheme
              .titleMedium
              ?.copyWith(color: AppTheme.danger, fontWeight: FontWeight.w900),
        ),
        const SizedBox(height: 8),
        Card(
          color: AppTheme.bgDark,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppTheme.radiusLg),
            side: const BorderSide(color: AppTheme.danger, width: 1.5),
          ),
          child: Padding(
            padding: const EdgeInsets.all(20),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                 Text(
                  loc.advResetTitle,
                  style: TextStyle(
                    fontWeight: FontWeight.w900,
                    fontSize: 16,
                    color: Colors.white,
                  ),
                ),
                const SizedBox(height: 8),
                 Text(
                  loc.advResetDesc,
                  style: TextStyle(color: AppTheme.textSecondary, fontSize: 13),
                ),
                const SizedBox(height: 16),
                SizedBox(
                  width: double.infinity,
                  child: ElevatedButton.icon(
                    style: ElevatedButton.styleFrom(
                      backgroundColor: AppTheme.danger,
                      foregroundColor: Colors.white,
                    ),
                    onPressed: _resetSystem,
                    icon: const Icon(Icons.delete_forever),
                    label: Text(loc.advResetButton),
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