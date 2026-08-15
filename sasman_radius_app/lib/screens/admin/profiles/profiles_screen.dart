import 'dart:convert';

import 'package:flutter/material.dart';

import '../../../l10n/app_localizations.dart';
import '../../../services/api_service.dart';
import '../../../widgets/notification_helper.dart';

class ProfilesScreen extends StatefulWidget {
  const ProfilesScreen({super.key, required this.api});

  final ApiService api;

  @override
  State<ProfilesScreen> createState() => _ProfilesScreenState();
}

class _ProfilesScreenState extends State<ProfilesScreen> {
  late final Future<void> _future = _load();
  List<Map<String, dynamic>> _profiles = [];
  String _query = '';

  Future<void> _load() async {
    final res = await widget.api.get('/radius/api/profiles');
    final decoded = jsonDecode(res.body);
    final list = _parseListResponse(decoded);
    setState(() => _profiles = list);
  }

  List<Map<String, dynamic>> _parseListResponse(dynamic decoded) {
    if (decoded is List) {
      return decoded.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
    }
    if (decoded is Map) {
      final list = decoded['data'] ?? decoded['items'] ?? decoded['profiles'];
      if (list is List) {
        return list.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
      }
    }
    return [];
  }

  List<Map<String, dynamic>> get _filteredProfiles {
    if (_query.isEmpty) return _profiles;
    final q = _query.toLowerCase();
    return _profiles.where((profile) {
      return profile.values.any((value) => value != null && value.toString().toLowerCase().contains(q));
    }).toList();
  }

  Future<void> _showProfileForm([Map<String, dynamic>? profile]) async {
    final isEdit = profile != null;
    final nameCtrl = TextEditingController(text: profile?['name'] ?? '');
    final downloadCtrl = TextEditingController(text: profile?['download']?.toString() ?? '');
    final uploadCtrl = TextEditingController(text: profile?['upload']?.toString() ?? '');
    final validityCtrl = TextEditingController(text: profile?['validity_days']?.toString() ?? profile?['validity']?.toString() ?? '30');
    final priceCtrl = TextEditingController(text: profile?['price']?.toString() ?? '0');
    final agentPriceCtrl = TextEditingController(text: profile?['agent_price']?.toString() ?? '0');
    final nasCtrl = TextEditingController(text: profile?['nas_ip'] ?? 'ALL');
    final simCtrl = TextEditingController(text: profile?['simultaneous']?.toString() ?? '1');
    final expiredPoolCtrl = TextEditingController(text: profile?['expired_pool'] ?? '');
    final expiredProfileCtrl = TextEditingController(text: profile?['expired_profile'] ?? '');
    final poolCtrl = TextEditingController(text: profile?['pool'] ?? '');
    final groupCtrl = TextEditingController(text: profile?['mikrotik_group'] ?? '');
    final expiredLinkType = (profile != null && profile['expired_pool'] != null && profile['expired_pool'].toString().isNotEmpty)
        ? 'pool'
        : (profile != null && profile['expired_profile'] != null && profile['expired_profile'].toString().isNotEmpty)
            ? 'group'
            : 'none';
    final linkType = (profile != null && profile['pool'] != null && profile['pool'].toString().isNotEmpty)
        ? 'pool'
        : (profile != null && profile['mikrotik_group'] != null && profile['mikrotik_group'].toString().isNotEmpty)
            ? 'group'
            : 'none';
    String selectedLinkType = linkType;
    String selectedExpiredLinkType = expiredLinkType;

    await showDialog<void>(
      context: context,
      builder: (context) {
        return StatefulBuilder(builder: (context, setState) {
          final loc = AppLocalizations.of(context);
          return AlertDialog(
            title: Text(isEdit ? loc.editProfile : loc.addNewProfile),
            content: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  TextField(controller: nameCtrl, decoration: InputDecoration(labelText: loc.profileName)),
                  const SizedBox(height: 10),
                  Row(
                    children: [
                      Expanded(child: TextField(controller: downloadCtrl, decoration: InputDecoration(labelText: loc.downloadSpeed))),
                      const SizedBox(width: 10),
                      Expanded(child: TextField(controller: uploadCtrl, decoration: InputDecoration(labelText: loc.uploadSpeed))),
                    ],
                  ),
                  const SizedBox(height: 10),
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(loc.mikrotikLinkType),
                      RadioListTile<String>(
                        value: 'none',
                        groupValue: selectedLinkType,
                        title: Text(loc.noLink),
                        onChanged: (value) => setState(() => selectedLinkType = value!),
                      ),
                      RadioListTile<String>(
                        value: 'pool',
                        groupValue: selectedLinkType,
                        title: const Text('IP Pool'),
                        onChanged: (value) => setState(() => selectedLinkType = value!),
                      ),
                      if (selectedLinkType == 'pool')
                        TextField(controller: poolCtrl, decoration: InputDecoration(labelText: loc.poolName)),
                      RadioListTile<String>(
                        value: 'group',
                        groupValue: selectedLinkType,
                        title: const Text('MikroTik Group'),
                        onChanged: (value) => setState(() => selectedLinkType = value!),
                      ),
                      if (selectedLinkType == 'group')
                        TextField(controller: groupCtrl, decoration: InputDecoration(labelText: loc.groupName)),
                    ],
                  ),
                  const SizedBox(height: 10),
                  TextField(controller: validityCtrl, decoration: InputDecoration(labelText: loc.validityDays)),
                  const SizedBox(height: 10),
                  TextField(controller: priceCtrl, decoration: InputDecoration(labelText: loc.userPrice)),
                  const SizedBox(height: 10),
                  TextField(controller: agentPriceCtrl, decoration: InputDecoration(labelText: loc.agentPrice)),
                  const SizedBox(height: 10),
                  TextField(controller: nasCtrl, decoration: const InputDecoration(labelText: 'NAS-IP-Address')),
                  const SizedBox(height: 10),
                  TextField(controller: simCtrl, decoration: InputDecoration(labelText: loc.simultaneousDevices)),
                  const SizedBox(height: 10),
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(loc.expiredTransfer),
                      RadioListTile<String>(
                        value: 'none',
                        groupValue: selectedExpiredLinkType,
                        title: Text(loc.noTransfer),
                        onChanged: (value) => setState(() => selectedExpiredLinkType = value!),
                      ),
                      RadioListTile<String>(
                        value: 'pool',
                        groupValue: selectedExpiredLinkType,
                        title: Text(loc.expiredPoolLabel),
                        onChanged: (value) => setState(() => selectedExpiredLinkType = value!),
                      ),
                      if (selectedExpiredLinkType == 'pool')
                        TextField(controller: expiredPoolCtrl, decoration: InputDecoration(labelText: loc.expiredPoolName)),
                      RadioListTile<String>(
                        value: 'group',
                        groupValue: selectedExpiredLinkType,
                        title: Text(loc.expiredProfileLabel),
                        onChanged: (value) => setState(() => selectedExpiredLinkType = value!),
                      ),
                      if (selectedExpiredLinkType == 'group')
                        TextField(controller: expiredProfileCtrl, decoration: const InputDecoration(labelText: 'expired profile')),
                    ],
                  ),
                ],
              ),
            ),
            actions: [
              TextButton(onPressed: () => Navigator.of(context).pop(), child: Text(loc.cancel)),
              ElevatedButton(
                onPressed: () async {
                  if (nameCtrl.text.trim().isEmpty) {
                    if (mounted) {
                      NotificationHelper.showError(context, loc.profileNameRequired);
                    }
                    return;
                  }
                  final payload = {
                    'original_name': profile?['name'] ?? '',
                    'name': nameCtrl.text.trim(),
                    'download': downloadCtrl.text.trim(),
                    'upload': uploadCtrl.text.trim(),
                    'pool': selectedLinkType == 'pool' ? poolCtrl.text.trim() : '',
                    'mikrotik_group': selectedLinkType == 'group' ? groupCtrl.text.trim() : '',
                    'validity': validityCtrl.text.trim(),
                    'price': double.tryParse(priceCtrl.text.trim()) ?? 0,
                    'agent_price': double.tryParse(agentPriceCtrl.text.trim()) ?? 0,
                    'nas_ip': nasCtrl.text.trim().isEmpty ? 'ALL' : nasCtrl.text.trim(),
                    'simultaneous': simCtrl.text.trim().isEmpty ? '1' : simCtrl.text.trim(),
                    'expired_pool': selectedExpiredLinkType == 'pool' ? expiredPoolCtrl.text.trim() : '',
                    'expired_profile': selectedExpiredLinkType == 'group' ? expiredProfileCtrl.text.trim() : '',
                    'admin_id': 0,
                  };
                  try {
                    await widget.api.post('/radius/api/profiles', body: payload);
                    await _load();
                    if (context.mounted) {
                      NotificationHelper.showSuccess(
                        context,
                        loc.profileCreated,
                      );
                      Navigator.of(context).pop();
                    }
                  } on ApiException catch (e) {
                    if (mounted) {
                      NotificationHelper.showError(context, e.message);
                    }
                  } finally {
                  }
                },
                child: Text(isEdit ? loc.saveChanges : loc.createProfile),
              ),
            ],
          );
        });
      },
    );
  }

  Future<void> _deleteProfile(String name) async {
    final loc = AppLocalizations.of(context);
    final confirmed = await showDialog<bool>(
          context: context,
          builder: (context) => AlertDialog(
            title: Text(loc.deleteProfile),
            content: Text('${loc.deleteProfileConfirm} "$name"؟'),
            actions: [
              TextButton(onPressed: () => Navigator.of(context).pop(false), child: Text(loc.cancel)),
              ElevatedButton(onPressed: () => Navigator.of(context).pop(true), child: Text(loc.delete)),
            ],
          ),
        ) ?? false;
    if (!confirmed) {
      return;
    }
    try {
      await widget.api.delete('/radius/api/profiles/${Uri.encodeComponent(name)}');
      await _load();
      if (mounted) {
        NotificationHelper.showSuccess(context, loc.profileDeleted);
      }
    } on ApiException catch (e) {
      if (mounted) {
        NotificationHelper.showError(context, e.message);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final rows = _filteredProfiles;
    final loc = AppLocalizations.of(context);
    return FutureBuilder<void>(
      future: _future,
      builder: (context, snapshot) {
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
                          Text(loc.profilesTitle, style: Theme.of(context).textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.w900)),
                          const SizedBox(height: 6),
                          Text(loc.profilesSubtitle),
                        ],
                      ),
                    ),
                    ElevatedButton.icon(onPressed: () => _showProfileForm(), icon: const Icon(Icons.add), label: Text(loc.addProfile)),
                  ],
                ),
              ),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16),
                child: TextField(
                  decoration: InputDecoration(labelText: loc.searchProfiles, prefixIcon: const Icon(Icons.search)),
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
                          ? Center(child: Text(loc.noProfiles))
                          : ListView.builder(
                              padding: const EdgeInsets.all(16),
                              itemCount: rows.length,
                              itemBuilder: (context, index) {
                                final profile = rows[index];
                                return Card(
                                  margin: const EdgeInsets.only(bottom: 12),
                                  child: Padding(
                                    padding: const EdgeInsets.all(14),
                                    child: Column(
                                      crossAxisAlignment: CrossAxisAlignment.start,
                                      children: [
                                        Row(
                                          children: [
                                            Expanded(child: Text(profile['name'] ?? '-', style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w800))),
                                            Text(profile['price']?.toString() ?? '0', style: const TextStyle(fontWeight: FontWeight.w700)),
                                          ],
                                        ),
                                        const SizedBox(height: 8),
                                        Wrap(spacing: 10, runSpacing: 6, children: [
                                          Chip(label: Text('${loc.speedLabel}${profile['download'] ?? '-'} / ${profile['upload'] ?? '-'}')),
                                          Chip(label: Text('${loc.validityLabel}${profile['validity_days'] ?? profile['validity'] ?? '-'}')),
                                          Chip(label: Text('NAS: ${profile['nas_ip'] ?? 'ALL'}')),
                                          Chip(label: Text('${loc.usersLabel}${profile['simultaneous'] ?? '1'}')),
                                        ]),
                                        const SizedBox(height: 10),
                                        Wrap(spacing: 8, runSpacing: 8, children: [
                                          ElevatedButton.icon(onPressed: () => _showProfileForm(profile), icon: const Icon(Icons.edit), label: Text(loc.editProfile)),
                                          OutlinedButton.icon(onPressed: () => _deleteProfile(profile['name']?.toString() ?? ''), icon: const Icon(Icons.delete), label: Text(loc.deleteProfile)),
                                        ]),
                                      ],
                                    ),
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