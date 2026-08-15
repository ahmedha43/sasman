import 'dart:convert';

import 'package:flutter/material.dart';

import '../../../config/app_theme.dart';
import '../../../l10n/app_localizations.dart';
import '../../../services/api_service.dart';
import '../../../widgets/notification_helper.dart';
import '../../../widgets/status_badge.dart';

class AgentsScreen extends StatefulWidget {
  const AgentsScreen({super.key, required this.api});

  final ApiService api;

  @override
  State<AgentsScreen> createState() => _AgentsScreenState();
}

class _AgentsScreenState extends State<AgentsScreen> with SingleTickerProviderStateMixin {
  late final TabController _tabController;
  late Future<void> _future = _loadAll();

  List<Map<String, dynamic>> _admins = [];
  List<Map<String, dynamic>> _transactions = [];
  String _query = '';

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  Future<void> _loadAll() async {
    final results = await Future.wait([
      widget.api.get('/radius/api/auth/admins'),
      widget.api.get('/radius/api/auth/admins/transactions').catchError((_) => httpResponseStub()),
    ]);

    _admins = _parseList(results[0].body);
    _transactions = _parseList(results[1].body);
    setState(() {});
  }

  static dynamic httpResponseStub() {
    return Object(); 
  }

  List<Map<String, dynamic>> _parseList(String body) {
    if (body.isEmpty) return [];
    try {
      final decoded = jsonDecode(body);
      if (decoded is List) {
        return decoded.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
      }
      if (decoded is Map) {
        final list = decoded['data'] ?? decoded['items'] ?? decoded['admins'] ?? decoded['transactions'];
        if (list is List) {
          return list.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
        }
      }
    } catch (_) {}
    return [];
  }

  List<Map<String, dynamic>> get _filteredAdmins {
    if (_query.isEmpty) return _admins;
    final q = _query.toLowerCase();
    return _admins.where((admin) {
      final name = '${admin['username'] ?? admin['name'] ?? ''}'.toLowerCase();
      final email = '${admin['email'] ?? ''}'.toLowerCase();
      return name.contains(q) || email.contains(q);
    }).toList();
  }

  Future<void> _registerAgent() async {
    final loc = AppLocalizations.of(context);
    final userCtrl = TextEditingController();
    final nameCtrl = TextEditingController();
    final passCtrl = TextEditingController();
    final emailCtrl = TextEditingController();
    String role = 'agent';
    bool canManageProfiles = false;
    bool canManageNas = false;

    await showDialog<void>(
      context: context,
      builder: (context) {
        return StatefulBuilder(
          builder: (context, setDlg) {
            return AlertDialog(
              title: Text(loc.addNewAgent),
              content: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    TextField(
                      controller: userCtrl,
                      decoration: InputDecoration(labelText: loc.username),
                    ),
                    const SizedBox(height: 10),
                    TextField(
                      controller: nameCtrl,
                      decoration: InputDecoration(labelText: loc.fullName),
                    ),
                    const SizedBox(height: 10),
                    TextField(
                      controller: passCtrl,
                      obscureText: true,
                      decoration: InputDecoration(labelText: loc.password),
                    ),
                    const SizedBox(height: 10),
                    TextField(
                      controller: emailCtrl,
                      keyboardType: TextInputType.emailAddress,
                      decoration: InputDecoration(labelText: loc.emailLabel),
                    ),
                    const SizedBox(height: 10),
                    DropdownButtonFormField<String>(
                      value: role,
                      decoration: InputDecoration(labelText: loc.role),
                      items: [
                        DropdownMenuItem(value: 'agent', child: Text(loc.roleAgent)),
                        DropdownMenuItem(value: 'admin', child: Text(loc.roleSubAdmin)),
                        DropdownMenuItem(value: 'superadmin', child: Text(loc.roleSuperAdmin)),
                      ],
                      onChanged: (val) => setDlg(() => role = val ?? 'agent'),
                    ),
                    const SizedBox(height: 14),
                    const Divider(),
                    const SizedBox(height: 6),
                    Align(
                      alignment: Alignment.centerRight,
                      child: Text(
                        loc.extraPermissions,
                        style: const TextStyle(fontWeight: FontWeight.w800, fontSize: 14),
                      ),
                    ),
                    const SizedBox(height: 8),
                    SwitchListTile(
                      value: canManageProfiles,
                      title: Text(loc.manageProfiles),
                      subtitle: Text(loc.manageProfilesDesc),
                      contentPadding: EdgeInsets.zero,
                      onChanged: (v) => setDlg(() => canManageProfiles = v),
                    ),
                    SwitchListTile(
                      value: canManageNas,
                      title: Text(loc.manageNas),
                      subtitle: Text(loc.manageNasDesc),
                      contentPadding: EdgeInsets.zero,
                      onChanged: (v) => setDlg(() => canManageNas = v),
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
                    if (userCtrl.text.trim().isEmpty || passCtrl.text.isEmpty) {
                      NotificationHelper.showError(context, loc.fillAllFields);
                      return;
                    }
                    try {
                      await widget.api.post('/radius/api/auth/register', body: {
                        'username': userCtrl.text.trim(),
                        'password': passCtrl.text,
                        'name': nameCtrl.text.trim(),
                        'email': emailCtrl.text.trim(),
                        'role': role,
                        'can_manage_profiles': canManageProfiles,
                        'can_manage_nas': canManageNas,
                      });
                      await _loadAll();
                      if (context.mounted) {
                        NotificationHelper.showSuccess(context, loc.agentRegistered);
                        Navigator.of(context).pop();
                      }
                    } on ApiException catch (e) {
                      if (context.mounted) {
                        NotificationHelper.showError(context, e.message);
                      }
                    }
                  },
                  child: Text(loc.registerBtn),
                ),
              ],
            );
          },
        );
      },
    );
  }

  Future<void> _deleteAgent(Map<String, dynamic> admin) async {
    final loc = AppLocalizations.of(context);
    final id = admin['id'] ?? admin['admin_id'];
    final name = admin['username'] ?? admin['name'] ?? '-';
    if (id == null) return;

    final confirmed = await showDialog<bool>(
          context: context,
          builder: (context) => AlertDialog(
            title: Text(loc.deleteAgentLabel),
            content: Text('${loc.deleteAgentConfirm} $name?'),
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

    if (!confirmed) return;

    try {
      await widget.api.delete('/radius/api/auth/admins/$id');
      await _loadAll();
      if (mounted) {
        NotificationHelper.showSuccess(context, loc.agentDeleted);
      }
    } on ApiException catch (e) {
      if (mounted) {
        NotificationHelper.showError(context, e.message);
      }
    }
  }

  Future<void> _walletOperation(Map<String, dynamic> admin, bool isRecharge) async {
    final loc = AppLocalizations.of(context);
    final id = admin['id'] ?? admin['admin_id'];
    final name = admin['username'] ?? admin['name'] ?? '-';
    if (id == null) return;

    final amountCtrl = TextEditingController();

    await showDialog<void>(
      context: context,
      builder: (context) {
        return AlertDialog(
          title: Text(isRecharge ? '${loc.rechargeBalance} $name' : '${loc.withdrawBalance} $name'),
          content: TextField(
            controller: amountCtrl,
            keyboardType: const TextInputType.numberWithOptions(decimal: true),
            decoration: InputDecoration(
              labelText: loc.amount,
              suffixText: loc.balanceCurrency,
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(context).pop(),
              child: Text(loc.cancel),
            ),
            ElevatedButton(
              onPressed: () async {
                final amount = double.tryParse(amountCtrl.text.trim());
                if (amount == null || amount <= 0) {
                  NotificationHelper.showError(context, loc.enterValidAmount2);
                  return;
                }
                try {
                  final endpoint = isRecharge
                      ? '/radius/api/auth/recharge'
                      : '/radius/api/auth/withdraw';

                  await widget.api.post(endpoint, body: {
                    'admin_id': id,
                    'amount': amount,
                  });
                  await _loadAll();
                  if (context.mounted) {
                    NotificationHelper.showSuccess(
                      context,
                      isRecharge ? loc.balanceCharged : loc.balanceWithdrawn,
                    );
                    Navigator.of(context).pop();
                  }
                } on ApiException catch (e) {
                  if (context.mounted) {
                    NotificationHelper.showError(context, e.message);
                  }
                }
              },
              child: Text(isRecharge ? loc.charge : loc.withdrawBtn),
            ),
          ],
        );
      },
    );
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
              Tab(text: loc.agentsTitle),
              Tab(text: loc.transactionsHistory),
            ],
          ),
          body: TabBarView(
            controller: _tabController,
            children: [
              _buildAgentsTab(snapshot, loc),
              _buildTransactionsTab(snapshot, loc),
            ],
          ),
        );
      },
    );
  }

  Widget _buildAgentsTab(AsyncSnapshot<void> snapshot, AppLocalizations loc) {
    final rows = _filteredAdmins;
    return Column(
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
                      loc.agentsTitle,
                      style: Theme.of(context)
                          .textTheme
                          .headlineSmall
                          ?.copyWith(fontWeight: FontWeight.w900),
                    ),
                    const SizedBox(height: 6),
                    Text(loc.agentsSubtitle),
                  ],
                ),
              ),
              ElevatedButton.icon(
                onPressed: _registerAgent,
                icon: const Icon(Icons.add),
                label: Text(loc.addAgent),
              ),
            ],
          ),
        ),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16),
          child: TextField(
            decoration: InputDecoration(
              labelText: loc.searchAgents,
              prefixIcon: const Icon(Icons.search),
            ),
            onChanged: (value) => setState(() => _query = value),
          ),
        ),
        const SizedBox(height: 12),
        Expanded(
          child: RefreshIndicator(
            onRefresh: _loadAll,
            child: snapshot.connectionState == ConnectionState.waiting
                ? const Center(child: CircularProgressIndicator())
                : rows.isEmpty
                    ? Center(child: Text(loc.noAgents))
                    : ListView.builder(
                        padding: const EdgeInsets.all(16),
                        itemCount: rows.length,
                        itemBuilder: (context, index) {
                          final admin = rows[index];
                          final role = '${admin['role'] ?? admin['privilege'] ?? 'agent'}';
                          final balance = admin['balance'] ?? admin['credit'] ?? 0;
                          return Container(
                            margin: const EdgeInsets.only(bottom: 12),
                            decoration: AppTheme.premiumCard(),
                            child: Padding(
                              padding: const EdgeInsets.all(16),
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Row(
                                    children: [
                                      CircleAvatar(
                                        backgroundColor: AppTheme.primary.withValues(alpha: 0.15),
                                        foregroundColor: AppTheme.primaryLight,
                                        child: Text(
                                          '${admin['username'] ?? admin['name'] ?? 'U'}'
                                              .substring(0, 1)
                                              .toUpperCase(),
                                        ),
                                      ),
                                      const SizedBox(width: 12),
                                      Expanded(
                                        child: Column(
                                          crossAxisAlignment: CrossAxisAlignment.start,
                                          children: [
                                            Text(
                                              admin['username'] ?? admin['name'] ?? '-',
                                              style: const TextStyle(
                                                fontSize: 16,
                                                fontWeight: FontWeight.w800,
                                              ),
                                            ),
                                            Text(
                                              admin['email'] ?? '',
                                              style: const TextStyle(
                                                fontSize: 12,
                                                color: AppTheme.textSecondary,
                                              ),
                                            ),
                                          ],
                                        ),
                                      ),
                                      StatusBadge(
                                        label: role == 'superadmin'
                                            ? loc.roleSuperAdmin
                                            : role == 'admin'
                                                ? loc.roleSubAdmin
                                                : loc.roleAgent,
                                        color: role == 'superadmin'
                                            ? AppTheme.accent
                                            : role == 'admin'
                                                ? AppTheme.info
                                                : AppTheme.success,
                                      ),
                                    ],
                                  ),
                                  const SizedBox(height: 14),
                                  const Divider(color: AppTheme.border, thickness: 0.5),
                                  const SizedBox(height: 10),
                                  Row(
                                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                                    children: [
                                      Text(
                                        '${loc.balanceLabel}: $balance ${loc.balanceCurrency}',
                                        style: const TextStyle(
                                          fontSize: 15,
                                          fontWeight: FontWeight.w900,
                                          color: AppTheme.accentLight,
                                        ),
                                      ),
                                      Wrap(
                                        spacing: 6,
                                        children: [
                                          OutlinedButton.icon(
                                            onPressed: () => _walletOperation(admin, true),
                                            icon: const Icon(Icons.add_card, size: 16),
                                            label: Text(loc.charge),
                                            style: OutlinedButton.styleFrom(
                                              minimumSize: const Size(0, 36),
                                              padding: const EdgeInsets.symmetric(horizontal: 10),
                                            ),
                                          ),
                                          OutlinedButton.icon(
                                            onPressed: () => _walletOperation(admin, false),
                                            icon: const Icon(Icons.money_off, size: 16),
                                            label: Text(loc.withdrawBtn),
                                            style: OutlinedButton.styleFrom(
                                              minimumSize: const Size(0, 36),
                                              padding: const EdgeInsets.symmetric(horizontal: 10),
                                            ),
                                          ),
                                          IconButton(
                                            onPressed: () => _deleteAgent(admin),
                                            icon: const Icon(Icons.delete, color: Colors.red),
                                          ),
                                        ],
                                      ),
                                    ],
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

  Widget _buildTransactionsTab(AsyncSnapshot<void> snapshot, AppLocalizations loc) {
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                loc.financialTransactions,
                style: Theme.of(context)
                    .textTheme
                    .headlineSmall
                    ?.copyWith(fontWeight: FontWeight.w900),
              ),
              const SizedBox(height: 6),
              Text(loc.depositWithdrawSales),
            ],
          ),
        ),
        const SizedBox(height: 8),
        Expanded(
          child: RefreshIndicator(
            onRefresh: _loadAll,
            child: snapshot.connectionState == ConnectionState.waiting
                ? const Center(child: CircularProgressIndicator())
                : _transactions.isEmpty
                    ? Center(child: Text(loc.noTransactions))
                    : ListView.builder(
                        padding: const EdgeInsets.all(16),
                        itemCount: _transactions.length,
                        itemBuilder: (context, index) {
                          final tx = _transactions[index];
                          final amount = tx['amount'] ?? 0;
                          final action = '${tx['action'] ?? tx['type'] ?? ''}'.toLowerCase();
                          final isDeposit = action.contains('recharge') ||
                              action.contains('deposit') ||
                              action.contains('add') ||
                              amount > 0;
                          final date = tx['created_at'] ?? tx['timestamp'] ?? '-';
                          final details = tx['details'] ?? tx['note'] ?? tx['description'] ?? '-';

                          return Card(
                            margin: const EdgeInsets.only(bottom: 10),
                            child: ListTile(
                              leading: CircleAvatar(
                                backgroundColor: isDeposit
                                    ? AppTheme.success.withValues(alpha: 0.15)
                                    : AppTheme.danger.withValues(alpha: 0.15),
                                child: Icon(
                                  isDeposit ? Icons.arrow_downward : Icons.arrow_upward,
                                  color: isDeposit ? AppTheme.success : AppTheme.danger,
                                ),
                              ),
                              title: Text(
                                '${tx['admin_username'] ?? tx['username'] ?? tx['admin_id'] ?? '-'}',
                                style: const TextStyle(fontWeight: FontWeight.bold),
                              ),
                              subtitle: Text('$details\n${loc.dateLabel}: $date'),
                              isThreeLine: true,
                              trailing: Text(
                                '${isDeposit ? "+" : "-"}${amount.abs()} ${loc.balanceCurrency}',
                                style: TextStyle(
                                  fontWeight: FontWeight.w900,
                                  fontSize: 15,
                                  color: isDeposit ? AppTheme.success : AppTheme.danger,
                                ),
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