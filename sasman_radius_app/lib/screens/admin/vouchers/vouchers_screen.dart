import 'dart:convert';

import 'package:flutter/material.dart';

import '../../../l10n/app_localizations.dart';
import '../../../services/api_service.dart';
import '../../../widgets/notification_helper.dart';

class VouchersScreen extends StatefulWidget {
  const VouchersScreen({super.key, required this.api});

  final ApiService api;

  @override
  State<VouchersScreen> createState() => _VouchersScreenState();
}

class _VouchersScreenState extends State<VouchersScreen> {
  late final Future<void> _future = _load();
  List<Map<String, dynamic>> _vouchers = [];
  List<Map<String, dynamic>> _profiles = [];
  String _query = '';

  Future<void> _load() async {
    final vouchersRes = await widget.api.get('/radius/api/vouchers');
    final profilesRes = await widget.api.get('/radius/api/profiles');
    _vouchers = _parseList(vouchersRes.body);
    _profiles = _parseList(profilesRes.body);
    setState(() {});
  }

  List<Map<String, dynamic>> _parseList(String body) {
    final decoded = jsonDecode(body);
    if (decoded is List) {
      return decoded.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
    }
    if (decoded is Map) {
      final list = decoded['data'] ?? decoded['items'] ?? decoded['vouchers'];
      if (list is List) {
        return list.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
      }
    }
    return [];
  }

  List<Map<String, dynamic>> get _filteredVouchers {
    if (_query.isEmpty) return _vouchers;
    final q = _query.toLowerCase();
    return _vouchers.where((voucher) {
      return voucher.values.any((value) => value != null && value.toString().toLowerCase().contains(q));
    }).toList();
  }

  Future<void> _generateVouchers() async {
    String profile = _profiles.isNotEmpty ? _profiles.first['name']?.toString() ?? '' : '';
    final countCtrl = TextEditingController(text: '10');
    final priceCtrl = TextEditingController(text: '0');
    String codeType = 'alphanumeric';
    final lengthCtrl = TextEditingController(text: '10');

    await showDialog<void>(
      context: context,
      builder: (context) {
        return StatefulBuilder(builder: (context, setState) {
          final dialogLoc = AppLocalizations.of(context);
          return AlertDialog(
            title: Text(dialogLoc.generateVouchersTitle),
            content: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  DropdownButtonFormField<String>(
                    value: profile.isEmpty ? null : profile,
                    decoration: InputDecoration(labelText: dialogLoc.voucherProfile),
                    items: _profiles.map((profile) {
                      return DropdownMenuItem(value: profile['name']?.toString() ?? '', child: Text(profile['name']?.toString() ?? ''));
                    }).toList(),
                    onChanged: (value) => setState(() => profile = value ?? ''),
                  ),
                  const SizedBox(height: 10),
                  TextField(controller: countCtrl, keyboardType: TextInputType.number, decoration: InputDecoration(labelText: dialogLoc.voucherCount)),
                  const SizedBox(height: 10),
                  TextField(controller: priceCtrl, keyboardType: const TextInputType.numberWithOptions(decimal: false), decoration: InputDecoration(labelText: dialogLoc.voucherPrice)),
                  const SizedBox(height: 10),
                  DropdownButtonFormField<String>(
                    value: codeType,
                    decoration: InputDecoration(labelText: dialogLoc.voucherCodeType),
                    items: [
                      DropdownMenuItem(value: 'alphanumeric', child: Text(dialogLoc.voucherAlphaNumeric)),
                      DropdownMenuItem(value: 'numbers', child: Text(dialogLoc.voucherNumbersOnly)),
                      DropdownMenuItem(value: 'letters', child: Text(dialogLoc.voucherLettersOnly)),
                    ],
                    onChanged: (value) => setState(() => codeType = value ?? 'alphanumeric'),
                  ),
                  const SizedBox(height: 10),
                  TextField(controller: lengthCtrl, keyboardType: TextInputType.number, decoration: InputDecoration(labelText: dialogLoc.voucherCodeLength)),
                ],
              ),
            ),
            actions: [
              TextButton(onPressed: () => Navigator.of(context).pop(), child: Text(dialogLoc.cancel)),
              ElevatedButton(
                onPressed: profile.isEmpty
                    ? null
                    : () async {
                        try {
                          await widget.api.post('/radius/api/vouchers/generate', body: {
                            'profile_name': profile,
                            'count': int.tryParse(countCtrl.text.trim()) ?? 1,
                            'price': double.tryParse(priceCtrl.text.trim()) ?? 0,
                            'code_type': codeType,
                            'code_length': int.tryParse(lengthCtrl.text.trim()) ?? 10,
                          });
                          await _load();
                          if (context.mounted) {
                            NotificationHelper.showSuccess(context, dialogLoc.vouchersGenerated);
                            Navigator.of(context).pop();
                          }
                        } on ApiException catch (e) {
                          if (mounted) {
                            NotificationHelper.showError(context, e.message);
                          }
                        }
                      },
                child: Text(dialogLoc.generateBtn),
              ),
            ],
          );
        });
      },
    );
  }

  Future<void> _deleteVoucher(int id) async {
    final loc = AppLocalizations.of(context);
    final confirmed = await showDialog<bool>(
          context: context,
          builder: (context) => AlertDialog(
            title: Text(loc.deleteVoucher),
            content: Text(loc.deleteVoucherConfirm),
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
      await widget.api.delete('/radius/api/vouchers/$id');
      await _load();
      if (mounted) {
        NotificationHelper.showSuccess(context, loc.voucherDeleted);
      }
    } on ApiException catch (e) {
      if (mounted) {
        NotificationHelper.showError(context, e.message);
      }
    }
  }

  Future<void> _deleteBatch(dynamic batchId) async {
    final loc = AppLocalizations.of(context);
    final confirmed = await showDialog<bool>(
          context: context,
          builder: (context) => AlertDialog(
            title: Text(loc.deleteBatchLabel),
            content: Text('${loc.deleteBatchConfirm} $batchId؟'),
            actions: [
              TextButton(onPressed: () => Navigator.of(context).pop(false), child: Text(loc.cancel)),
              ElevatedButton(onPressed: () => Navigator.of(context).pop(true), child: Text(loc.deleteBatchLabel)),
            ],
          ),
        ) ?? false;
    if (!confirmed) {
      return;
    }
    try {
      await widget.api.delete('/radius/api/vouchers/batch/$batchId');
      await _load();
      if (mounted) {
        NotificationHelper.showSuccess(context, loc.batchDeleted);
      }
    } on ApiException catch (e) {
      if (mounted) {
        NotificationHelper.showError(context, e.message);
      }
    }
  }

  Future<void> _clearAllVouchers() async {
    final loc = AppLocalizations.of(context);
    final confirmed = await showDialog<bool>(
          context: context,
          builder: (context) => AlertDialog(
            title: Text(loc.clearAllVouchers),
            content: Text(loc.clearAllVouchersConfirm),
            actions: [
              TextButton(onPressed: () => Navigator.of(context).pop(false), child: Text(loc.cancel)),
              ElevatedButton(onPressed: () => Navigator.of(context).pop(true), child: Text(loc.clearAllLabel)),
            ],
          ),
        ) ?? false;
    if (!confirmed) {
      return;
    }
    try {
      await widget.api.delete('/radius/api/vouchers/all/clear');
      await _load();
      if (mounted) {
        NotificationHelper.showSuccess(context, loc.allVouchersCleared);
      }
    } on ApiException catch (e) {
      if (mounted) {
        NotificationHelper.showError(context, e.message);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final rows = _filteredVouchers;
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
                          Text(loc.vouchersTitle, style: Theme.of(context).textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.w900)),
                          const SizedBox(height: 6),
                          Text(loc.vouchersSubtitle),
                        ],
                      ),
                    ),
                    ElevatedButton.icon(onPressed: _generateVouchers, icon: const Icon(Icons.add), label: Text(loc.generateVouchers)),
                    const SizedBox(width: 8),
                    PopupMenuButton<String>(
                      onSelected: (value) {
                        if (value == 'clear_all') {
                          _clearAllVouchers();
                        }
                      },
                      itemBuilder: (context) => [
                        PopupMenuItem(
                          value: 'clear_all',
                          child: Row(
                            children: [
                              const Icon(Icons.delete_sweep, color: Colors.red),
                              const SizedBox(width: 8),
                              Text(loc.clearAllVouchers),
                            ],
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16),
                child: TextField(
                  decoration: InputDecoration(labelText: loc.searchVouchers, prefixIcon: const Icon(Icons.search)),
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
                          ? Center(child: Text(loc.noVouchers))
                          : ListView.builder(
                              padding: const EdgeInsets.all(16),
                              itemCount: rows.length,
                              itemBuilder: (context, index) {
                                final voucher = rows[index];
                                final used = voucher['is_used'] == 1 || voucher['is_used'] == true || voucher['status'] == 'used';
                                final batchId = voucher['batch_id'] ?? voucher['batch'] ?? '-';
                                return Card(
                                  margin: const EdgeInsets.only(bottom: 12),
                                  child: ListTile(
                                    title: Text(voucher['code']?.toString() ?? '-'),
                                    subtitle: Text('${loc.voucherProfileLabel}${voucher['profile_name'] ?? voucher['profile'] ?? '-'} • ${voucher['validity_days'] ?? voucher['validity'] ?? '-'} ${loc.voucherDay} • ${loc.voucherBatchLabel}$batchId'),
                                    trailing: Row(
                                      mainAxisSize: MainAxisSize.min,
                                      children: [
                                        Text(used ? loc.voucherUsed : loc.voucherAvailable, style: TextStyle(color: used ? Colors.red : Colors.green)),
                                        const SizedBox(width: 8),
                                        PopupMenuButton<String>(
                                          onSelected: (value) {
                                            if (value == 'delete_single') {
                                              _deleteVoucher(int.tryParse(voucher['id']?.toString() ?? '') ?? 0);
                                            } else if (value == 'delete_batch') {
                                              if (batchId != '-') {
                                                _deleteBatch(batchId);
                                              } else {
                                                NotificationHelper.showError(context, loc.noBatchId);
                                              }
                                            }
                                          },
                                          itemBuilder: (context) => [
                                            PopupMenuItem(
                                              value: 'delete_single',
                                              child: Row(
                                                children: [
                                                  const Icon(Icons.delete, color: Colors.red),
                                                  const SizedBox(width: 8),
                                                  Text(loc.deleteThisVoucher),
                                                ],
                                              ),
                                            ),
                                            PopupMenuItem(
                                              value: 'delete_batch',
                                              child: Row(
                                                children: [
                                                  const Icon(Icons.delete_sweep, color: Colors.amber),
                                                  const SizedBox(width: 8),
                                                  Text(loc.deleteEntireBatch),
                                                ],
                                              ),
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
          ),
        );
      },
    );
  }
}