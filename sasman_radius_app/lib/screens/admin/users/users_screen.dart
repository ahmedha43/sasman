import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';

import '../../../config/app_theme.dart';
import '../../../l10n/app_localizations.dart';
import '../../../services/api_service.dart';
import '../../../widgets/notification_helper.dart';


class UsersScreen extends StatefulWidget {
  const UsersScreen({super.key, required this.api});

  final ApiService api;

  @override
  State<UsersScreen> createState() => UsersScreenState();
}

class UsersScreenState extends State<UsersScreen> {
  late Future<void> _future = _load();
  List<Map<String, dynamic>> _users = [];
  List<Map<String, dynamic>> _profiles = [];
  final _searchCtrl = TextEditingController();
  String _query = '';
  String _activeFilter = 'all';
  int _warningHours = 72;
  Timer? _autoRefreshTimer;

  @override
  void initState() {
    super.initState();
    _searchCtrl.addListener(() {
      setState(() {
        _query = _searchCtrl.text;
      });
    });
    _autoRefreshTimer = Timer.periodic(const Duration(seconds: 30), (_) {
      _load(); // Auto-refresh every 30s
    });
  }

  @override
  void dispose() {
    _searchCtrl.dispose();
    _autoRefreshTimer?.cancel();
    super.dispose();
  }

  void setFilter(String filterType) {
    setState(() {
      _activeFilter = filterType;
      _searchCtrl.clear();
      _query = '';
    });
  }

  Future<void> _load() async {
    final usersRes = await widget.api.get('/radius/api/users');
    final profilesRes = await widget.api.get('/radius/api/profiles');
    final users = _parseJsonList(usersRes.body);
    final profiles = _parseJsonList(profilesRes.body);

    int warningHours = 72;
    try {
      final waRes = await widget.api.get('/radius/api/whatsapp/config');
      if (waRes.statusCode == 200) {
        final waData = jsonDecode(waRes.body);
        if (waData is Map && waData['reminder_hours'] != null) {
          warningHours = int.tryParse(waData['reminder_hours'].toString()) ?? 72;
        }
      }
    } catch (e) {
      debugPrint('[Users] Failed to fetch warning hours: $e');
    }

    setState(() {
      _users = users;
      _profiles = profiles;
      _warningHours = warningHours;
    });
  }

  List<Map<String, dynamic>> _parseJsonList(String body) {
    final decoded = jsonDecode(body);
    if (decoded is List) {
      return decoded
          .whereType<Map>()
          .map((e) => Map<String, dynamic>.from(e))
          .toList();
    }
    if (decoded is Map) {
      final list = decoded['data'] ?? decoded['items'] ?? decoded['users'];
      if (list is List) {
        return list
            .whereType<Map>()
            .map((e) => Map<String, dynamic>.from(e))
            .toList();
      }
    }
    return [];
  }

  List<Map<String, dynamic>> get _filteredUsers {
    final now = DateTime.now();
    final warningTimeLimit = now.add(Duration(hours: _warningHours));

    Iterable<Map<String, dynamic>> temp = _users;
    if (_activeFilter == 'active') {
      temp = temp.where((u) => u['enabled'] == true && u['expired'] != true);
    } else if (_activeFilter == 'expired') {
      temp = temp.where((u) {
        if (u['expired'] == true || u['enabled'] != true) return true;
        final session = u['session'];
        if (session is Map) {
          final sessionStatus = '${session['status'] ?? ''}'.toLowerCase();
          return sessionStatus == 'expired' || sessionStatus == 'expired_online';
        }
        return false;
      });
    } else if (_activeFilter == 'online') {
      temp = temp.where((u) {
        final session = u['session'];
        return session is Map && (session['online'] == true || session['status'] == 'online');
      });
    } else if (_activeFilter == 'about-to-expire') {
      temp = temp.where((u) {
        final expiresAtStr = '${u['expires_at'] ?? ''}'.trim();
        if (expiresAtStr.isNotEmpty && u['expired'] != true && u['enabled'] == true) {
          final expiresAt = DateTime.tryParse(expiresAtStr.replaceAll(' ', 'T'));
          return expiresAt != null && expiresAt.isAfter(now) && !expiresAt.isAfter(warningTimeLimit);
        }
        return false;
      });
    }

    if (_query.isEmpty) return temp.toList();
    final q = _query.toLowerCase();
    return temp.where((user) {
      return user.values.any(
        (value) => value != null && value.toString().toLowerCase().contains(q),
      );
    }).toList();
  }

  // ─── User Form (Add / Edit) ────────────────────────────────────────────────

  Future<void> _showUserForm([Map<String, dynamic>? user]) async {
    final loc = AppLocalizations.of(context);
    final isEdit = user != null;
    final usernameCtrl = TextEditingController(text: user?['user'] ?? '');
    final passwordCtrl = TextEditingController(text: user?['pass'] ?? '');
    final fullNameCtrl = TextEditingController(text: user?['full_name'] ?? '');
    final phoneCtrl = TextEditingController(text: user?['phone'] ?? '');
    final expiryCtrl = TextEditingController(text: user?['expires_at'] ?? '');
    String selectedProfile =
        user?['profile'] ??
        (_profiles.isNotEmpty ? _profiles.first['name'] ?? '' : '');

    await showDialog<void>(
      context: context,
      builder: (context) {
        return AlertDialog(
          title: Text(isEdit ? loc.editUserTitle : loc.addUserTitle),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextField(
                  controller: usernameCtrl,
                  decoration: InputDecoration(labelText: loc.username),
                ),
                const SizedBox(height: 10),
                TextField(
                  controller: passwordCtrl,
                  decoration: InputDecoration(labelText: loc.password),
                ),
                const SizedBox(height: 10),
                TextField(
                  controller: fullNameCtrl,
                  decoration: InputDecoration(labelText: loc.fullName),
                ),
                const SizedBox(height: 10),
                TextField(
                  controller: phoneCtrl,
                  decoration: InputDecoration(labelText: loc.phone),
                ),
                const SizedBox(height: 10),
                DropdownButtonFormField<String>(
                  value: selectedProfile.isEmpty ? null : selectedProfile,
                  items: _profiles.map((profile) {
                    final name = profile['name']?.toString() ?? '';
                    return DropdownMenuItem(value: name, child: Text(name));
                  }).toList(),
                  onChanged: (value) => selectedProfile = value ?? '',
                  decoration: InputDecoration(labelText: loc.profile),
                ),
                const SizedBox(height: 10),
                TextField(
                  controller: expiryCtrl,
                  decoration: InputDecoration(
                    labelText: loc.expiryDateHint,
                  ),
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
                  'user': usernameCtrl.text.trim(),
                  'old_user': user?['user'] ?? '',
                  'pass': passwordCtrl.text,
                  'full_name': fullNameCtrl.text,
                  'phone': phoneCtrl.text,
                  'profile': selectedProfile,
                  'expires_at': expiryCtrl.text.trim(),
                  'admin_id': 0,
                };
                try {
                  await widget.api.post('/radius/api/users', body: payload);
                  await _load();
                  if (context.mounted) {
                    NotificationHelper.showSuccess(
                      context,
                      isEdit ? loc.userUpdated : loc.userCreated,
                    );
                    Navigator.of(context).pop();
                  }
                } on ApiException catch (e) {
                  if (context.mounted) {
                    NotificationHelper.showError(context, e.message);
                  }
                }
              },
              child: Text(isEdit ? loc.saveChanges : loc.createUser),
            ),
          ],
        );
      },
    );
  }

  // ─── Actions ───────────────────────────────────────────────────────────────

  Future<void> _performAction(String userName, String action) async {
    final loc = AppLocalizations.of(context);
    final encoded = Uri.encodeComponent(userName);
    late final String path;
    switch (action) {
      case 'disconnect':
        path = '/radius/api/users/$encoded/disconnect';
        break;
      case 'toggle':
        path = '/radius/api/users/$encoded/toggle-status';
        break;
      default:
        return;
    }
    try {
      await widget.api.post(path);
      await _load();
      if (mounted) {
        NotificationHelper.showSuccess(
          context,
          action == 'disconnect'
              ? loc.disconnectSuccess
              : loc.toggleSuccess,
        );
      }
    } on ApiException catch (e) {
      if (mounted) {
        NotificationHelper.showError(context, e.message);
      }
    }
  }

  Future<void> _renewUser(String userName) async {
    final loc = AppLocalizations.of(context);
    final encoded = Uri.encodeComponent(userName);
    String selectedProfile =
        _profiles.isEmpty ? '' : _profiles.first['name'] ?? '';
    bool paid = true;
    await showDialog<void>(
      context: context,
      builder: (context) {
        return StatefulBuilder(
          builder: (context, setState) {
            return AlertDialog(
              title: Text('${loc.renewSubscription} $userName'),
              content: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  DropdownButtonFormField<String>(
                    value: selectedProfile.isEmpty ? null : selectedProfile,
                    items: _profiles.map((profile) {
                      final name = profile['name']?.toString() ?? '';
                      final validity =
                          profile['validity_days']?.toString() ?? '';
                      return DropdownMenuItem(
                        value: name,
                        child: Text(
                          '$name ${validity.isNotEmpty ? '($validity ${loc.dayUnit})' : ''}',
                        ),
                      );
                    }).toList(),
                    onChanged: (value) =>
                        setState(() => selectedProfile = value ?? ''),
                    decoration: InputDecoration(
                      labelText: loc.newProfile,
                    ),
                  ),
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      Checkbox(
                        value: paid,
                        onChanged: (value) =>
                            setState(() => paid = value ?? true),
                      ),
                      Expanded(
                        child: Text(
                          '${loc.paidAmount} (${loc.paidAmountInfo})',
                        ),
                      ),
                    ],
                  ),
                ],
              ),
              actions: [
                TextButton(
                onPressed: () => Navigator.of(context).pop(),
                child: Text(loc.cancel),
              ),
              ElevatedButton(
                onPressed: selectedProfile.isEmpty
                    ? null
                    : () async {
                        try {
                          await widget.api.post(
                            '/radius/api/users/$encoded/renew',
                            body: {'profile': selectedProfile, 'paid': paid},
                          );
                          await _load();
                          if (context.mounted) {
                            NotificationHelper.showSuccess(
                              context,
                              loc.renewSuccess,
                            );
                              Navigator.of(context).pop();
                            }
                          } on ApiException catch (e) {
                            if (mounted) {
                              NotificationHelper.showError(context, e.message);
                            }
                          }
                        },
                    child: Text(loc.renew),
                ),
              ],
            );
          },
        );
      },
    );
  }

  Future<void> _deleteUser(String userName) async {
    final loc = AppLocalizations.of(context);
    final confirmed =
        await showDialog<bool>(
          context: context,
          builder: (context) => AlertDialog(
            title: Text(loc.deleteUserTitle),
            content: Text('${loc.deleteUserConfirm} "$userName"'),
            actions: [
              TextButton(
                onPressed: () => Navigator.of(context).pop(false),
                child: Text(loc.cancel),
              ),
              ElevatedButton(
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppTheme.danger,
                ),
                onPressed: () => Navigator.of(context).pop(true),
                child: Text(loc.delete),
              ),
            ],
          ),
        ) ??
        false;
    if (!confirmed) return;
    try {
      await widget.api.delete(
        '/radius/api/users/${Uri.encodeComponent(userName)}',
      );
      await _load();
      if (mounted) {
        NotificationHelper.showSuccess(context, loc.deleteSuccess);
      }
    } on ApiException catch (e) {
      if (mounted) {
        NotificationHelper.showError(context, e.message);
      }
    }
  }

  // ─── Transaction Dialog ────────────────────────────────────────────────────

  Future<void> _showTransactionDialog(
    String username, {
    required String type,
    VoidCallback? onDone,
  }) async {
    final loc = AppLocalizations.of(context);
    final amountCtrl = TextEditingController();
    final notesCtrl = TextEditingController();
    bool saving = false;
    final isDebt = type == 'debt';
      final title = isDebt ? loc.txAddDebtTitle : loc.txPayDebtTitle;
    final color = isDebt ? AppTheme.warning : AppTheme.success;

    await showDialog<void>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setDlg) => AlertDialog(
          title: Row(
            children: [
              Icon(
                isDebt ? Icons.add_circle : Icons.remove_circle,
                color: color,
              ),
              const SizedBox(width: 8),
              Text(title),
            ],
          ),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: 12,
                  vertical: 8,
                ),
                decoration: BoxDecoration(
                  color: AppTheme.surfaceMuted,
                  borderRadius: BorderRadius.circular(8),
                  border: Border.all(color: AppTheme.border),
                ),
                child: Row(
                  children: [
                    const Icon(
                      Icons.person,
                      size: 16,
                      color: AppTheme.textSecondary,
                    ),
                    const SizedBox(width: 6),
                    Text(
                      '${loc.txSubscriber}: $username',
                      style: const TextStyle(
                        fontWeight: FontWeight.w700,
                        fontSize: 13,
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 14),
              TextField(
                controller: amountCtrl,
                keyboardType:
                    const TextInputType.numberWithOptions(decimal: true),
                decoration: InputDecoration(
                  labelText: loc.amount,
                  prefixIcon: Icon(Icons.monetization_on, color: color),
                  hintText: loc.amountHint,
                ),
              ),
              const SizedBox(height: 10),
              TextField(
                controller: notesCtrl,
                maxLines: 2,
                decoration: InputDecoration(
                  labelText: loc.notes,
                  prefixIcon: const Icon(Icons.note),
                ),
              ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(),
              child: Text(loc.cancel),
            ),
            ElevatedButton(
              style: ElevatedButton.styleFrom(backgroundColor: color),
              onPressed: saving
                  ? null
                  : () async {
                      final amount =
                          double.tryParse(amountCtrl.text.trim()) ?? 0;
                      if (amount <= 0) {
                        NotificationHelper.showError(
                          ctx,
                          loc.enterValidAmount,
                        );
                        return;
                      }
                      setDlg(() => saving = true);
                      try {
                        final encoded = Uri.encodeComponent(username);
                        final res = await widget.api.post(
                          '/radius/api/users/$encoded/transactions',
                          body: {
                            'type': type,
                            'amount': amount,
                            'notes': notesCtrl.text.trim(),
                          },
                        );
                        final decoded = jsonDecode(res.body);
                        final msg = decoded['message']?.toString() ??
                            (isDebt
                                ? loc.debtAdded
                                : loc.debtPaid);
                        await _load();
                        if (ctx.mounted) {
                          NotificationHelper.showSuccess(ctx, msg);
                          Navigator.of(ctx).pop();
                          onDone?.call();
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
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        color: Colors.white,
                      ),
                    )
                  : Text(isDebt ? loc.txAddDebtBtn : loc.txPayDebtBtn),
            ),
          ],
        ),
      ),
    );
  }

  // ─── User Details (full‑screen bottom sheet) ───────────────────────────────

  Future<void> _showDetails(String userName) async {
    try {
      final response = await widget.api.get(
        '/radius/api/users/${Uri.encodeComponent(userName)}/details',
      );
      final decoded = jsonDecode(response.body);
      if (decoded is! Map<String, dynamic>) {
        if (mounted) {
          NotificationHelper.showError(context, AppLocalizations.of(context).invalidData);
        }
        return;
      }
      if (!mounted) return;

    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      backgroundColor: Theme.of(context).scaffoldBackgroundColor,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
        builder: (ctx) => _UserDetailsSheet(
          data: decoded,
          api: widget.api,
          onTransactionAdded: () {
            _load();
          },
        ),
      );
    } on ApiException catch (e) {
      if (mounted) NotificationHelper.showError(context, e.message);
    } catch (_) {
       if (mounted) {
        NotificationHelper.showError(context, AppLocalizations.of(context).subscriberDetailsError);
      }
    }
  }

  // ─── Build ─────────────────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<void>(
      future: _future,
      builder: (context, snapshot) {
        final loc = AppLocalizations.of(context);
        final rows = _filteredUsers;
        return Scaffold(
          body: Column(
            children: [
              Padding(
                padding: const EdgeInsets.all(16),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            loc.usersTitle,
                            style: Theme.of(context)
                                .textTheme
                                .headlineSmall
                                ?.copyWith(fontWeight: FontWeight.w900),
                          ),
                          const SizedBox(height: 6),
                          Text(loc.usersSubtitle),
                        ],
                      ),
                    ),
                    ElevatedButton.icon(
                      onPressed:
                          _profiles.isEmpty ? null : () => _showUserForm(),
                      icon: const Icon(Icons.add),
                      label: Text(loc.addUser),
                    ),
                  ],
                ),
              ),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16),
                child: TextField(
                  controller: _searchCtrl,
                  decoration: InputDecoration(
                    labelText: loc.searchUsers,
                    prefixIcon: const Icon(Icons.search),
                    suffixIcon: _query.isNotEmpty
                        ? IconButton(
                            icon: const Icon(Icons.clear),
                            onPressed: () {
                              _searchCtrl.clear();
                            },
                          )
                        : null,
                  ),
                ),
              ),
              const SizedBox(height: 10),
              SingleChildScrollView(
                scrollDirection: Axis.horizontal,
                padding: const EdgeInsets.symmetric(horizontal: 16),
                child: Row(
                  children: [
                    FilterChip(
                      label: Text(loc.filterAll),
                      selected: _activeFilter == 'all',
                      selectedColor: AppTheme.primary.withValues(alpha: 0.2),
                      checkmarkColor: AppTheme.primary,
                      onSelected: (selected) {
                        setState(() => _activeFilter = 'all');
                      },
                    ),
                    const SizedBox(width: 8),
                    FilterChip(
                      label: Text(loc.filterActive),
                      selected: _activeFilter == 'active',
                      selectedColor: AppTheme.success.withValues(alpha: 0.2),
                      checkmarkColor: AppTheme.success,
                      onSelected: (selected) {
                        setState(() => _activeFilter = 'active');
                      },
                    ),
                    const SizedBox(width: 8),
                    FilterChip(
                      label: Text(loc.filterExpired),
                      selected: _activeFilter == 'expired',
                      selectedColor: AppTheme.danger.withValues(alpha: 0.2),
                      checkmarkColor: AppTheme.danger,
                      onSelected: (selected) {
                        setState(() => _activeFilter = 'expired');
                      },
                    ),
                    const SizedBox(width: 8),
                    FilterChip(
                      label: Text(loc.filterOnline),
                      selected: _activeFilter == 'online',
                      selectedColor: AppTheme.info.withValues(alpha: 0.2),
                      checkmarkColor: AppTheme.info,
                      onSelected: (selected) {
                        setState(() => _activeFilter = 'online');
                      },
                    ),
                    const SizedBox(width: 8),
                    FilterChip(
                      label: Text(loc.filterAboutToExpire),
                      selected: _activeFilter == 'about-to-expire',
                      selectedColor: AppTheme.warning.withValues(alpha: 0.2),
                      checkmarkColor: AppTheme.warning,
                      onSelected: (selected) {
                        setState(() => _activeFilter = 'about-to-expire');
                      },
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 10),
              Expanded(
                child: RefreshIndicator(
                  onRefresh: _load,
                  child: snapshot.connectionState == ConnectionState.waiting
                      ? const Center(child: CircularProgressIndicator())
                      : rows.isEmpty
                          ? Center(
                              child: Text(loc.noResults),
                            )
                          : ListView.builder(
                              padding: const EdgeInsets.all(16),
                              itemCount: rows.length,
                              itemBuilder: (context, index) {
                                final user = rows[index];
                                return _UserCard(
                                  user: user,
                                  onEdit: () => _showUserForm(user),
                                  onRenew: () => _renewUser(
                                    '${user['user'] ?? user['username'] ?? ''}',
                                  ),
                                  onToggle: () => _performAction(
                                    '${user['user'] ?? user['username'] ?? ''}',
                                    'toggle',
                                  ),
                                  onDisconnect: () => _performAction(
                                    '${user['user'] ?? user['username'] ?? ''}',
                                    'disconnect',
                                  ),
                                  onDetails: () => _showDetails(
                                    '${user['user'] ?? user['username'] ?? ''}',
                                  ),
                                  onDelete: () => _deleteUser(
                                    '${user['user'] ?? user['username'] ?? ''}',
                                  ),
                                  onAddDebt: () => _showTransactionDialog(
                                    '${user['user'] ?? user['username'] ?? ''}',
                                    type: 'debt',
                                  ),
                                  onPayment: () => _showTransactionDialog(
                                    '${user['user'] ?? user['username'] ?? ''}',
                                    type: 'payment',
                                  ),
                                );
                              },
                            ),
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

// ═══════════════════════════════════════════════════════════════════════════════
// User Card Widget
// ═══════════════════════════════════════════════════════════════════════════════

class _UserCard extends StatefulWidget {
  const _UserCard({
    required this.user,
    required this.onEdit,
    required this.onRenew,
    required this.onToggle,
    required this.onDisconnect,
    required this.onDetails,
    required this.onDelete,
    required this.onAddDebt,
    required this.onPayment,
  });

  final Map<String, dynamic> user;
  final VoidCallback onEdit;
  final VoidCallback onRenew;
  final VoidCallback onToggle;
  final VoidCallback onDisconnect;
  final VoidCallback onDetails;
  final VoidCallback onDelete;
  final VoidCallback onAddDebt;
  final VoidCallback onPayment;

  @override
  State<_UserCard> createState() => _UserCardState();
}

class _UserCardState extends State<_UserCard> {
  bool _isExpanded = false;

  @override
  Widget build(BuildContext context) {
    final loc = AppLocalizations.of(context);
    final user = widget.user;
    final username = '${user['user'] ?? user['username'] ?? ''}';
    final fullName = '${user['full_name'] ?? '-'}';
    final status = '${user['status'] ?? user['state'] ?? ''}';
    final active =
        user['enabled'] == true || status.toLowerCase().contains('active');
    final balance = (user['balance'] as num?)?.toDouble() ?? 0;
    final hasDebt = balance > 0;
    final expiresAt = '${user['expires_at'] ?? '-'}';

    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: InkWell(
        onTap: () {
          setState(() {
            _isExpanded = !_isExpanded;
          });
        },
        borderRadius: BorderRadius.circular(AppTheme.radiusLg),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // ── Collapsed Info Row
              Row(
                children: [
                  // Status dot indicator
                  Container(
                    width: 8,
                    height: 8,
                    decoration: BoxDecoration(
                      color: active ? AppTheme.success : AppTheme.danger,
                      shape: BoxShape.circle,
                      boxShadow: [
                        BoxShadow(
                          color: (active ? AppTheme.success : AppTheme.danger)
                              .withValues(alpha: 0.4),
                          blurRadius: 4,
                          spreadRadius: 1,
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(width: 10),
                  // Username & Fullname
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(
                          username,
                          style: TextStyle(
                            fontSize: 14,
                            fontWeight: FontWeight.w800,
                            color: Theme.of(context).brightness == Brightness.dark ? AppTheme.textPrimary : AppTheme.textDark,
                          ),
                        ),
                        if (fullName.isNotEmpty && fullName != '-')
                          Text(
                            fullName,
                            style: const TextStyle(
                              fontSize: 11,
                              color: AppTheme.textSecondary,
                            ),
                          ),
                      ],
                    ),
                  ),
                  // Expiry & Debt Info
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          const Icon(
                            Icons.calendar_today_rounded,
                            size: 11,
                            color: AppTheme.textMuted,
                          ),
                          const SizedBox(width: 4),
                          Text(
                            expiresAt.split(' ').first, // Show date part
                            style: const TextStyle(
                              fontSize: 11,
                              color: AppTheme.textSecondary,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ],
                      ),
                      if (hasDebt) ...[
                        const SizedBox(height: 3),
                        Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 6,
                            vertical: 2,
                          ),
                          decoration: BoxDecoration(
                            color: AppTheme.danger.withValues(alpha: 0.15),
                            borderRadius: BorderRadius.circular(4),
                            border: Border.all(
                              color: AppTheme.danger.withValues(alpha: 0.3),
                            ),
                          ),
                          child: Text(
                            '${loc.debtLabel}: ${balance.toStringAsFixed(0)} ${loc.balanceCurrency}',
                            style: const TextStyle(
                              fontSize: 10,
                              fontWeight: FontWeight.w800,
                              color: AppTheme.dangerLight,
                            ),
                          ),
                        ),
                      ],
                    ],
                  ),
                  const SizedBox(width: 8),
                  // Expand arrow indicator
                  Icon(
                    _isExpanded
                        ? Icons.keyboard_arrow_up_rounded
                        : Icons.keyboard_arrow_down_rounded,
                    size: 18,
                    color: AppTheme.textMuted,
                  ),
                ],
              ),

              // ── Expanded content with smooth AnimatedSize transition
              AnimatedSize(
                duration: const Duration(milliseconds: 250),
                curve: Curves.easeInOut,
                child: _isExpanded
                    ? Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          const SizedBox(height: 12),
                          const Divider(color: AppTheme.border, thickness: 0.5),
                          const SizedBox(height: 8),
                          // Expanded Info Chips
                          Wrap(
                            spacing: 8,
                            runSpacing: 6,
                            children: [
                              Chip(
                                label: Text('${loc.cardName}: $fullName'),
                              ),
                              Chip(
                                label: Text('${loc.cardProfile}: ${user['profile'] ?? '-'}'),
                              ),
                              Chip(
                                label: Text('${loc.cardExpiry}: $expiresAt'),
                              ),
                              Chip(
                                avatar: Icon(
                                  hasDebt
                                      ? Icons.warning_amber_rounded
                                      : Icons.check_circle_rounded,
                                  size: 14,
                                  color: hasDebt
                                      ? AppTheme.dangerLight
                                      : AppTheme.successLight,
                                ),
                                label: Text(
                                  '${loc.cardBalance}: ${balance.toStringAsFixed(0)} ${loc.balanceCurrency}',
                                  style: TextStyle(
                                    fontWeight: FontWeight.w700,
                                    color: hasDebt
                                        ? AppTheme.dangerLight
                                        : AppTheme.successLight,
                                  ),
                                ),
                                backgroundColor: hasDebt
                                    ? AppTheme.danger.withValues(alpha: 0.15)
                                    : AppTheme.success.withValues(alpha: 0.15),
                                side: BorderSide(
                                  color: hasDebt
                                      ? AppTheme.danger.withValues(alpha: 0.3)
                                      : AppTheme.success.withValues(alpha: 0.3),
                                ),
                              ),
                            ],
                          ),
                          const SizedBox(height: 12),
                          // Action buttons
                          Wrap(
                            spacing: 8,
                            runSpacing: 8,
                            children: [
                              ElevatedButton.icon(
                                icon: const Icon(Icons.edit_rounded, size: 14),
                                label: Text(loc.editBtn),
                                style: ElevatedButton.styleFrom(
                                  minimumSize: const Size(0, 36),
                                  padding: const EdgeInsets.symmetric(
                                    horizontal: 12,
                                  ),
                                ),
                                onPressed: widget.onEdit,
                              ),
                              OutlinedButton.icon(
                                icon: const Icon(
                                  Icons.refresh_rounded,
                                  size: 14,
                                ),
                                label: Text(loc.renew),
                                style: OutlinedButton.styleFrom(
                                  minimumSize: const Size(0, 36),
                                  padding: const EdgeInsets.symmetric(
                                    horizontal: 12,
                                  ),
                                ),
                                onPressed: widget.onRenew,
                              ),
                              OutlinedButton.icon(
                                icon: const Icon(
                                  Icons.info_outline_rounded,
                                  size: 14,
                                ),
                                label: Text(loc.details),
                                style: OutlinedButton.styleFrom(
                                  minimumSize: const Size(0, 36),
                                  padding: const EdgeInsets.symmetric(
                                    horizontal: 12,
                                  ),
                                ),
                                onPressed: widget.onDetails,
                              ),
                              OutlinedButton.icon(
                                icon: const Icon(
                                  Icons.power_off_rounded,
                                  size: 14,
                                ),
                                label: Text(loc.toggleStatus),
                                style: OutlinedButton.styleFrom(
                                  minimumSize: const Size(0, 36),
                                  padding: const EdgeInsets.symmetric(
                                    horizontal: 12,
                                  ),
                                ),
                                onPressed: widget.onToggle,
                              ),
                              OutlinedButton.icon(
                                icon: const Icon(
                                  Icons.wifi_off_rounded,
                                  size: 14,
                                ),
                                label: Text(loc.disconnectBtn),
                                style: OutlinedButton.styleFrom(
                                  minimumSize: const Size(0, 36),
                                  padding: const EdgeInsets.symmetric(
                                    horizontal: 12,
                                  ),
                                ),
                                onPressed: widget.onDisconnect,
                              ),
                              OutlinedButton.icon(
                                icon: const Icon(
                                  Icons.delete_rounded,
                                  size: 14,
                                ),
                                label: Text(loc.delete),
                                style: OutlinedButton.styleFrom(
                                  foregroundColor: AppTheme.danger,
                                  side:
                                      const BorderSide(color: AppTheme.danger),
                                  minimumSize: const Size(0, 36),
                                  padding: const EdgeInsets.symmetric(
                                    horizontal: 12,
                                  ),
                                ),
                                onPressed: widget.onDelete,
                              ),
                            ],
                          ),
                          const SizedBox(height: 8),
                          // Financial buttons row
                          Row(
                            children: [
                              Expanded(
                                child: OutlinedButton.icon(
                                  icon: const Icon(
                                    Icons.add_circle_outline_rounded,
                                    size: 14,
                                    color: AppTheme.warning,
                                  ),
                                  label: Text(
                                    loc.txAddDebtBtn,
                                    style: TextStyle(
                                      color: AppTheme.warning,
                                      fontSize: 13,
                                    ),
                                  ),
                                  style: OutlinedButton.styleFrom(
                                    side: const BorderSide(
                                      color: AppTheme.warning,
                                    ),
                                    minimumSize: const Size(0, 36),
                                  ),
                                  onPressed: widget.onAddDebt,
                                ),
                              ),
                              const SizedBox(width: 8),
                              Expanded(
                                child: OutlinedButton.icon(
                                  icon: const Icon(
                                    Icons.remove_circle_outline_rounded,
                                    size: 14,
                                    color: AppTheme.success,
                                  ),
                                  label: Text(
                                    loc.txPayDebtBtn,
                                    style: TextStyle(
                                      color: AppTheme.success,
                                      fontSize: 13,
                                    ),
                                  ),
                                  style: OutlinedButton.styleFrom(
                                    side: const BorderSide(
                                      color: AppTheme.success,
                                    ),
                                    minimumSize: const Size(0, 36),
                                  ),
                                  onPressed: widget.onPayment,
                                ),
                              ),
                            ],
                          ),
                        ],
                      )
                    : const SizedBox.shrink(),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════════════════
// User Details Bottom Sheet
// ═══════════════════════════════════════════════════════════════════════════════

class _UserDetailsSheet extends StatefulWidget {
  const _UserDetailsSheet({
    required this.data,
    required this.api,
    required this.onTransactionAdded,
  });

  final Map<String, dynamic> data;
  final ApiService api;
  final VoidCallback onTransactionAdded;

  @override
  State<_UserDetailsSheet> createState() => _UserDetailsSheetState();
}

class _UserDetailsSheetState extends State<_UserDetailsSheet>
    with SingleTickerProviderStateMixin {
  late final TabController _tabs;
  late Map<String, dynamic> _data;

  @override
  void initState() {
    super.initState();
    _tabs = TabController(length: 3, vsync: this);
    _data = widget.data;
  }

  @override
  void dispose() {
    _tabs.dispose();
    super.dispose();
  }

  Future<void> _refreshDetails() async {
    final username = _data['user']?.toString() ?? '';
    if (username.isEmpty) return;
    try {
      final res = await widget.api.get(
        '/radius/api/users/${Uri.encodeComponent(username)}/details',
      );
      final decoded = jsonDecode(res.body);
      if (decoded is Map<String, dynamic> && mounted) {
        setState(() => _data = decoded);
      }
    } catch (_) {}
  }

  Future<void> _showTransactionDialog(String type) async {
    final loc = AppLocalizations.of(context);
    final username = _data['user']?.toString() ?? '';
    if (username.isEmpty) return;

    final amountCtrl = TextEditingController();
    final notesCtrl = TextEditingController();
    bool saving = false;
    final isDebt = type == 'debt';
    final color = isDebt ? AppTheme.warning : AppTheme.success;

    await showDialog<void>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setDlg) => AlertDialog(
          title: Row(
            children: [
              Icon(
                isDebt ? Icons.add_circle : Icons.remove_circle,
                color: color,
              ),
              const SizedBox(width: 8),
              Text(isDebt ? loc.txAddDebtTitle : loc.txPayDebtTitle),
            ],
          ),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: 12,
                  vertical: 8,
                ),
                decoration: BoxDecoration(
                  color: AppTheme.surfaceMuted,
                  borderRadius: BorderRadius.circular(8),
                  border: Border.all(color: AppTheme.border),
                ),
                child: Row(
                  children: [
                    const Icon(
                      Icons.person,
                      size: 16,
                      color: AppTheme.textSecondary,
                    ),
                    const SizedBox(width: 6),
                    Text(
                      '${loc.txSubscriber}: $username',
                      style: const TextStyle(
                        fontWeight: FontWeight.w700,
                        fontSize: 13,
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 14),
              TextField(
                controller: amountCtrl,
                keyboardType:
                    const TextInputType.numberWithOptions(decimal: true),
                decoration: InputDecoration(
                  labelText: loc.amount,
                  prefixIcon: Icon(Icons.monetization_on, color: color),
                  hintText: loc.amountHint,
                ),
              ),
              const SizedBox(height: 10),
              TextField(
                controller: notesCtrl,
                maxLines: 2,
                decoration: InputDecoration(
                  labelText: loc.notes,
                  prefixIcon: const Icon(Icons.note),
                ),
              ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(),
              child: Text(loc.cancel),
            ),
            ElevatedButton(
              style: ElevatedButton.styleFrom(backgroundColor: color),
              onPressed: saving
                  ? null
                  : () async {
                      final amount =
                          double.tryParse(amountCtrl.text.trim()) ?? 0;
                      if (amount <= 0) {
                        NotificationHelper.showError(
                          ctx,
                          loc.enterValidAmount,
                        );
                        return;
                      }
                      setDlg(() => saving = true);
                      try {
                        final encoded = Uri.encodeComponent(username);
                        final res = await widget.api.post(
                          '/radius/api/users/$encoded/transactions',
                          body: {
                            'type': type,
                            'amount': amount,
                            'notes': notesCtrl.text.trim(),
                          },
                        );
                        final decoded = jsonDecode(res.body);
                        final msg = decoded['message']?.toString() ??
                            (isDebt
                                ? loc.debtAdded
                                : loc.debtPaid);
                        await _refreshDetails();
                        widget.onTransactionAdded();
                        if (ctx.mounted) {
                          NotificationHelper.showSuccess(ctx, msg);
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
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        color: Colors.white,
                      ),
                    )
                  : Text(isDebt ? loc.txAddDebtBtn : loc.txPayDebtBtn),
            ),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final loc = AppLocalizations.of(context);
    final username = _data['user']?.toString() ?? '';
    final balance = (_data['balance'] as num?)?.toDouble() ?? 0;
    final hasDebt = balance > 0;
    final session = _data['session'] as Map<String, dynamic>? ?? {};
    final statusStr = session['status']?.toString() ?? 'offline';

    final statusMap = {
      'online': (loc.statusOnline, AppTheme.success),
      'stale': (loc.statusStale, AppTheme.warning),
      'offline': (loc.statusOffline, AppTheme.textSecondary),
      'expired': (loc.statusExpired, AppTheme.danger),
      'expired_online': (loc.statusExpiredOnline, AppTheme.warning),
    };
    final statusInfo = statusMap[statusStr] ?? statusMap['offline']!;

    return DraggableScrollableSheet(
      initialChildSize: 0.92,
      minChildSize: 0.5,
      maxChildSize: 0.95,
      expand: false,
      builder: (context, scrollController) {
        return Column(
          children: [
            // ── Handle
            Container(
              margin: const EdgeInsets.only(top: 10, bottom: 4),
              width: 40,
              height: 4,
              decoration: BoxDecoration(
                color: AppTheme.border,
                borderRadius: BorderRadius.circular(4),
              ),
            ),
            // ── Title
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      '${loc.detailsTitle}: $username',
                      style: const TextStyle(
                        fontWeight: FontWeight.w900,
                        fontSize: 18,
                      ),
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.refresh),
                    tooltip: loc.refresh,
                    onPressed: _refreshDetails,
                  ),
                  IconButton(
                    icon: const Icon(Icons.close),
                    onPressed: () => Navigator.of(context).pop(),
                  ),
                ],
              ),
            ),
            // ── Tabs
            Container(
              margin: const EdgeInsets.fromLTRB(16, 8, 16, 0),
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
                    Tab(text: loc.tabInfo),
                    Tab(text: loc.tabFinancial),
                    Tab(text: loc.tabSessions),
                  ],
              ),
            ),
            const SizedBox(height: 8),
            // ── Tab views
            Expanded(
              child: TabBarView(
                controller: _tabs,
                children: [
                  // ── Tab 1: Info
                  ListView(
                    controller: scrollController,
                    padding: const EdgeInsets.all(16),
                    children: [
                      // status & info card
                      Card(
                        child: Padding(
                          padding: const EdgeInsets.all(16),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                loc.basicInfo,
                                style: const TextStyle(
                                  fontWeight: FontWeight.w800,
                                  fontSize: 16,
                                ),
                              ),
                              const SizedBox(height: 14),
                               _infoRow(loc.infoUser, username),
                              _infoRow(
                                loc.fullName,
                                _data['full_name']?.toString() ?? '-',
                              ),
                              _infoRow(
                                loc.phone,
                                _data['phone']?.toString() ?? '-',
                              ),
                              _infoRow(
                                loc.profile,
                                _data['profile']?.toString() ?? '-',
                              ),
                              _infoRow(
                                loc.expiryDate,
                                _data['expires_at']?.toString() ?? '-',
                              ),
                              _infoRow(
                                loc.infoCreatedAt,
                                _data['created_at']?.toString() ?? '-',
                              ),
                              const SizedBox(height: 8),
                              Row(
                                children: [
                                  SizedBox(
                                    width: 100,
                                    child: Text(
                                      loc.infoStatus,
                                      style: const TextStyle(
                                        color: AppTheme.textSecondary,
                                        fontSize: 13,
                                      ),
                                    ),
                                  ),
                                  Container(
                                    padding: const EdgeInsets.symmetric(
                                      horizontal: 10,
                                      vertical: 4,
                                    ),
                                    decoration: BoxDecoration(
                                      color: statusInfo.$2
                                          .withValues(alpha: 0.12),
                                      borderRadius: BorderRadius.circular(8),
                                    ),
                                    child: Text(
                                      statusInfo.$1,
                                      style: TextStyle(
                                        fontWeight: FontWeight.w700,
                                        color: statusInfo.$2,
                                        fontSize: 13,
                                      ),
                                    ),
                                  ),
                                ],
                              ),
                              const SizedBox(height: 6),
                              Row(
                                children: [
                                  SizedBox(
                                    width: 100,
                                    child: Text(
                                      loc.balance,
                                      style: const TextStyle(
                                        color: AppTheme.textSecondary,
                                        fontSize: 13,
                                      ),
                                    ),
                                  ),
                                  Container(
                                    padding: const EdgeInsets.symmetric(
                                      horizontal: 10,
                                      vertical: 4,
                                    ),
                                    decoration: BoxDecoration(
                                      color: (hasDebt
                                              ? AppTheme.danger
                                              : AppTheme.success)
                                          .withValues(alpha: 0.12),
                                      borderRadius: BorderRadius.circular(8),
                                    ),
                                    child: Text(
                                      '${balance.toStringAsFixed(0)} ${loc.balanceCurrency}',
                                      style: TextStyle(
                                        fontWeight: FontWeight.w800,
                                        color: hasDebt
                                            ? AppTheme.danger
                                            : AppTheme.success,
                                        fontSize: 14,
                                      ),
                                    ),
                                  ),
                                ],
                              ),
                            ],
                          ),
                        ),
                      ),
                      const SizedBox(height: 12),
                      // ── Session info card
                      Card(
                        child: Padding(
                          padding: const EdgeInsets.all(16),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                loc.currentSessionInfo,
                                style: const TextStyle(
                                  fontWeight: FontWeight.w800,
                                  fontSize: 16,
                                ),
                              ),
                              const SizedBox(height: 12),
                              if (session['ip'] != null &&
                                  session['ip'].toString().isNotEmpty) ...[
                                _infoRow(
                                      loc.sessIp, session['ip']?.toString() ?? '-'),
                                  _infoRow(loc.sessDownload,
                                    session['download']?.toString() ?? '0 B'),
                                  _infoRow(loc.sessUpload,
                                    session['upload']?.toString() ?? '0 B'),
                                  _infoRow(
                                    loc.sessDuration,
                                    _formatDuration(
                                      (session['session_seconds'] as num?)
                                              ?.toInt() ??
                                          0),
                                  ),
                                  _infoRow(
                                    loc.sessMac,
                                    session['calling_station']?.toString() ?? '-',
                                  ),
                                  _infoRow(
                                    loc.sessNasIp,
                                    session['nas_ip']?.toString() ?? '-',
                                  ),
                              ] else
                                Text(
                                  loc.noActiveSession,
                                  style: const TextStyle(
                                    color: AppTheme.textSecondary,
                                    fontSize: 14,
                                  ),
                                ),
                            ],
                          ),
                        ),
                      ),
                    ],
                  ),

                  // ── Tab 2: Financial
                  _buildFinancialTab(scrollController),

                  // ── Tab 3: Session History
                  _buildSessionHistoryTab(scrollController),
                ],
              ),
            ),
          ],
        );
      },
    );
  }

  // ─── Tab 2: Financial Records ──────────────────────────────────────────────

  Widget _buildFinancialTab(ScrollController scrollController) {
    final loc = AppLocalizations.of(context);
    final transactions =
        (_data['transactions'] as List?)?.cast<Map<String, dynamic>>() ?? [];
    final balance = (_data['balance'] as num?)?.toDouble() ?? 0;
    final hasDebt = balance > 0;

    return ListView(
      controller: scrollController,
      padding: const EdgeInsets.all(16),
      children: [
        // ── Balance summary card
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Row(
              children: [
                Container(
                  width: 50,
                  height: 50,
                  decoration: BoxDecoration(
                    color: (hasDebt ? AppTheme.danger : AppTheme.success)
                        .withValues(alpha: 0.12),
                    borderRadius: BorderRadius.circular(14),
                  ),
                  child: Icon(
                    hasDebt
                        ? Icons.warning_amber_rounded
                        : Icons.check_circle_rounded,
                    color: hasDebt ? AppTheme.danger : AppTheme.success,
                    size: 28,
                  ),
                ),
                const SizedBox(width: 14),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                       Text(
                         loc.currentBalance,
                         style: const TextStyle(
                          color: AppTheme.textSecondary,
                          fontSize: 13,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        '${balance.toStringAsFixed(0)} ${loc.balanceCurrency}',
                        style: TextStyle(
                          fontWeight: FontWeight.w900,
                          fontSize: 22,
                          color: hasDebt ? AppTheme.danger : AppTheme.success,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 12),
        // ── Action buttons
        Row(
          children: [
            Expanded(
              child: ElevatedButton.icon(
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppTheme.warning,
                ),
                 icon: const Icon(Icons.add_circle, size: 18),
                 label: Text(loc.txAddDebtBtn),
                onPressed: () => _showTransactionDialog('debt'),
              ),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: ElevatedButton.icon(
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppTheme.success,
                ),
                 icon: const Icon(Icons.remove_circle, size: 18),
                 label: Text(loc.txPayDebtBtn),
                onPressed: () => _showTransactionDialog('payment'),
              ),
            ),
          ],
        ),
        const SizedBox(height: 16),
        // ── Transactions list
        Row(
          children: [
            const Icon(Icons.receipt_long, size: 18, color: AppTheme.primary),
            const SizedBox(width: 6),
            Text(
              loc.financialRecords,
              style: const TextStyle(fontWeight: FontWeight.w800, fontSize: 16),
            ),
            const Spacer(),
            Text(
              '${transactions.length} ${loc.txCount}',
              style: const TextStyle(
                color: AppTheme.textSecondary,
                fontSize: 13,
              ),
            ),
          ],
        ),
        const SizedBox(height: 10),
        if (transactions.isEmpty)
          Card(
            child: Padding(
              padding: const EdgeInsets.all(30),
              child: Column(
                children: [
                  Icon(
                    Icons.account_balance_wallet_outlined,
                    size: 48,
                    color: AppTheme.textSecondary.withValues(alpha: 0.5),
                  ),
                  const SizedBox(height: 12),
                  Text(
                    loc.noFinancialRecords,
                    style: const TextStyle(color: AppTheme.textSecondary),
                  ),
                ],
              ),
            ),
          )
        else
          ...transactions.map((t) {
            final isDebt = t['type'] == 'debt';
            final amount = (t['amount'] as num?)?.toDouble() ?? 0;
            final notes = t['notes']?.toString() ?? '';
            final createdAt = t['created_at']?.toString() ?? '';

            return Card(
              margin: const EdgeInsets.only(bottom: 8),
              child: Padding(
                padding: const EdgeInsets.all(14),
                child: Row(
                  children: [
                    // ── Type icon
                    Container(
                      width: 40,
                      height: 40,
                      decoration: BoxDecoration(
                        color: (isDebt ? AppTheme.danger : AppTheme.success)
                            .withValues(alpha: 0.12),
                        borderRadius: BorderRadius.circular(10),
                      ),
                      child: Icon(
                        isDebt
                            ? Icons.arrow_upward_rounded
                            : Icons.arrow_downward_rounded,
                        color: isDebt ? AppTheme.danger : AppTheme.success,
                        size: 22,
                      ),
                    ),
                    const SizedBox(width: 12),
                    // ── Details
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Row(
                            children: [
                              Container(
                                padding: const EdgeInsets.symmetric(
                                  horizontal: 8,
                                  vertical: 2,
                                ),
                                decoration: BoxDecoration(
                                  color: (isDebt
                                          ? AppTheme.danger
                                          : AppTheme.success)
                                      .withValues(alpha: 0.12),
                                  borderRadius: BorderRadius.circular(6),
                                ),
                                 child: Text(
                                   isDebt ? loc.txDebtType : loc.txPayType,
                                  style: TextStyle(
                                    fontWeight: FontWeight.w700,
                                    fontSize: 12,
                                    color: isDebt
                                        ? AppTheme.danger
                                        : AppTheme.success,
                                  ),
                                ),
                              ),
                              const Spacer(),
                              Text(
                                 '${amount.toStringAsFixed(0)} ${loc.balanceCurrency}',
                                style: TextStyle(
                                  fontWeight: FontWeight.w800,
                                  fontSize: 15,
                                  color: isDebt
                                      ? AppTheme.danger
                                      : AppTheme.success,
                                ),
                              ),
                            ],
                          ),
                          if (notes.isNotEmpty) ...[
                            const SizedBox(height: 4),
                            Text(
                              notes,
                              style: const TextStyle(
                                color: AppTheme.textSecondary,
                                fontSize: 12,
                              ),
                              maxLines: 2,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ],
                          const SizedBox(height: 4),
                          Text(
                            createdAt,
                            style: const TextStyle(
                              color: AppTheme.textSecondary,
                              fontSize: 11,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            );
          }),
      ],
    );
  }

  // ─── Tab 3: Session History ────────────────────────────────────────────────

  Widget _buildSessionHistoryTab(ScrollController scrollController) {
    final loc = AppLocalizations.of(context);
    final sessions = (_data['session_history'] as List?)
            ?.cast<Map<String, dynamic>>() ??
        [];

    return ListView(
      controller: scrollController,
      padding: const EdgeInsets.all(16),
      children: [
        Row(
          children: [
            const Icon(Icons.history, size: 18, color: AppTheme.primary),
            const SizedBox(width: 6),
            Text(
              loc.sessionHistory,
              style: const TextStyle(fontWeight: FontWeight.w800, fontSize: 16),
            ),
            const Spacer(),
            Text(
              '${sessions.length} ${loc.sessionCount}',
              style: const TextStyle(
                color: AppTheme.textSecondary,
                fontSize: 13,
              ),
            ),
          ],
        ),
        const SizedBox(height: 10),
        if (sessions.isEmpty)
          Card(
            child: Padding(
              padding: const EdgeInsets.all(30),
              child: Column(
                children: [
                  Icon(
                    Icons.wifi_off,
                    size: 48,
                    color: AppTheme.textSecondary.withValues(alpha: 0.5),
                  ),
                  const SizedBox(height: 12),
                  Text(
                    loc.noSessionHistory,
                    style: const TextStyle(color: AppTheme.textSecondary),
                  ),
                ],
              ),
            ),
          )
        else
          ...sessions.map((s) {
            final startedAt = s['started_at']?.toString() ?? '-';
            final stoppedAt = s['stopped_at']?.toString() ?? '';
            final ip = s['ip']?.toString() ?? '-';
            final download = s['download']?.toString() ?? '-';
            final upload = s['upload']?.toString() ?? '-';
            final sessionTime = (s['session_time'] as num?)?.toInt() ?? 0;
            final mac = s['calling_station']?.toString() ?? '-';
            final isActive = stoppedAt.isEmpty;

            return Card(
              margin: const EdgeInsets.only(bottom: 8),
              child: Padding(
                padding: const EdgeInsets.all(14),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    // ── Header
                    Row(
                      children: [
                        Container(
                          width: 36,
                          height: 36,
                          decoration: BoxDecoration(
                            color: (isActive
                                    ? AppTheme.success
                                    : AppTheme.textSecondary)
                                .withValues(alpha: 0.12),
                            borderRadius: BorderRadius.circular(10),
                          ),
                          child: Icon(
                            isActive ? Icons.wifi : Icons.wifi_off,
                            color: isActive
                                ? AppTheme.success
                                : AppTheme.textSecondary,
                            size: 18,
                          ),
                        ),
                        const SizedBox(width: 10),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                ip,
                                style: const TextStyle(
                                  fontWeight: FontWeight.w700,
                                  fontFamily: 'monospace',
                                  fontSize: 14,
                                ),
                              ),
                               Text(
                                 '${loc.startedAt}: $startedAt',
                                 style: const TextStyle(
                                  color: AppTheme.textSecondary,
                                  fontSize: 11,
                                ),
                              ),
                            ],
                          ),
                        ),
                        if (isActive)
                          Container(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 8,
                              vertical: 3,
                            ),
                            decoration: BoxDecoration(
                              color: AppTheme.success.withValues(alpha: 0.12),
                              borderRadius: BorderRadius.circular(6),
                            ),
                            child: Text(
                              loc.sessionActive,
                              style: const TextStyle(
                                color: AppTheme.success,
                                fontWeight: FontWeight.w700,
                                fontSize: 11,
                              ),
                            ),
                          ),
                      ],
                    ),
                    const SizedBox(height: 10),
                    // ── Info row
                    Wrap(
                      spacing: 14,
                      runSpacing: 6,
                      children: [
                         _sessionChip(Icons.download, loc.sessDownload, download),
                         _sessionChip(Icons.upload, loc.sessUpload, upload),
                         _sessionChip(
                           Icons.timer,
                           loc.sessDuration,
                           _formatDuration(sessionTime),
                         ),
                         _sessionChip(
                           Icons.devices,
                           loc.sessMac,
                           mac,
                         ),
                      ],
                    ),
                    if (!isActive) ...[
                      const SizedBox(height: 6),
                       Text(
                         '${loc.endedAt}: $stoppedAt',
                         style: const TextStyle(
                          color: AppTheme.textSecondary,
                          fontSize: 11,
                        ),
                      ),
                    ],
                  ],
                ),
              ),
            );
          }),
      ],
    );
  }

  // ─── Helpers ───────────────────────────────────────────────────────────────

  Widget _infoRow(String label, String value) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Row(
        children: [
          SizedBox(
            width: 100,
            child: Text(
              label,
              style: const TextStyle(
                color: AppTheme.textSecondary,
                fontSize: 13,
              ),
            ),
          ),
          Expanded(
            child: Text(
              value,
              style: const TextStyle(
                fontWeight: FontWeight.w600,
                fontSize: 13,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _sessionChip(IconData icon, String label, String value) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: 14, color: AppTheme.textSecondary),
        const SizedBox(width: 3),
        Text(
          '$label: ',
          style: const TextStyle(
            color: AppTheme.textSecondary,
            fontSize: 11,
          ),
        ),
        Text(
          value,
          style: const TextStyle(
            fontWeight: FontWeight.w600,
            fontSize: 11,
          ),
        ),
      ],
    );
  }

  String _formatDuration(int seconds) {
    final loc = AppLocalizations.of(context);
    if (seconds <= 0) return '0${loc.durSec}';
    final h = seconds ~/ 3600;
    final m = (seconds % 3600) ~/ 60;
    final s = seconds % 60;
    if (h > 0) return '${h}${loc.durHour} ${m}${loc.durMin} ${s}${loc.durSec}';
    if (m > 0) return '${m}${loc.durMin} ${s}${loc.durSec}';
    return '${s}${loc.durSec}';
  }
}