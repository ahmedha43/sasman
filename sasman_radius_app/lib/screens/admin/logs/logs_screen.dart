import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';

import '../../../l10n/app_localizations.dart';
import '../../../services/api_service.dart';
import '../../../widgets/notification_helper.dart';
import '../../../widgets/structured_details.dart';

class LogsScreen extends StatefulWidget {
  const LogsScreen({super.key, required this.api});

  final ApiService api;

  @override
  State<LogsScreen> createState() => _LogsScreenState();
}

class _LogsScreenState extends State<LogsScreen> {
  late Future<void> _future = _load();
  List<Map<String, dynamic>> _logs = [];
  String _query = '';
  Timer? _timer;
  bool _autoRefresh = true;

  @override
  void initState() {
    super.initState();
    _startTimer();
  }

  void _startTimer() {
    _timer?.cancel();
    _timer = Timer.periodic(const Duration(seconds: 2), (timer) {
      if (_autoRefresh && mounted && _query.isEmpty) {
        _load(quiet: true);
      }
    });
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  Future<void> _load({bool quiet = false}) async {
    try {
      final res = await widget.api.get('/radius/api/logs');
      final list = _parseLogsResponse(res.body);
      if (mounted) {
        setState(() => _logs = list);
      }
    } catch (_) {
      if (!quiet) rethrow;
    }
  }

  List<Map<String, dynamic>> _parseLogsResponse(String body) {
    final trimmed = body.trim();
    if (trimmed.isEmpty) return [];

    try {
      return _parseList(jsonDecode(trimmed));
    } on FormatException {
      return _parsePlainTextLogs(trimmed);
    }
  }

  List<Map<String, dynamic>> _parseList(dynamic decoded) {
    if (decoded is List) {
      return decoded
          .map(_normalizeLogItem)
          .whereType<Map<String, dynamic>>()
          .toList();
    }
    if (decoded is Map) {
      final list = decoded['data'] ?? decoded['items'] ?? decoded['logs'];
      if (list is List) {
        return list
            .map(_normalizeLogItem)
            .whereType<Map<String, dynamic>>()
            .toList();
      }
      return [Map<String, dynamic>.from(decoded)];
    }
    return [];
  }

  Map<String, dynamic>? _normalizeLogItem(dynamic item) {
    if (item is Map) return Map<String, dynamic>.from(item);
    if (item is String && item.trim().isNotEmpty) {
      return _parseLogLine(item.trim());
    }
    return null;
  }

  List<Map<String, dynamic>> _parsePlainTextLogs(String text) {
    return text
        .split(RegExp(r'\r?\n'))
        .map((line) => line.trim())
        .where((line) => line.isNotEmpty)
        .map(_parseLogLine)
        .toList();
  }

  Map<String, dynamic> _parseLogLine(String line) {
    final match = RegExp(
      r'^(\d{4}/\d{2}/\d{2})\s+(\d{2}:\d{2}:\d{2})\s+(\[[^\]]+\])?\s*(.*)$',
    ).firstMatch(line);
    if (match == null) {
      return {'message': line, 'raw': line};
    }

    return {
      'timestamp': '${match.group(1)} ${match.group(2)}',
      if (match.group(3) != null)
        'source': match.group(3)!.replaceAll(RegExp(r'^\[|\]$'), ''),
      'message': match.group(4)?.trim().isNotEmpty == true
          ? match.group(4)!.trim()
          : line,
      'raw': line,
    };
  }

  List<Map<String, dynamic>> get _filteredLogs {
    if (_query.isEmpty) return _logs;
    final q = _query.toLowerCase();
    return _logs
        .where(
          (log) => log.values.any(
            (value) =>
                value != null && value.toString().toLowerCase().contains(q),
          ),
        )
        .toList();
  }

  Future<void> _clearLogs() async {
    final loc = AppLocalizations.of(context);
    final confirmed =
        await showDialog<bool>(
          context: context,
          builder: (context) => AlertDialog(
            title: Text(loc.logsClearTitle),
            content: Text(
              loc.logsClearConfirm,
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.of(context).pop(false),
                child: Text(loc.cancel),
              ),
              ElevatedButton(
                onPressed: () => Navigator.of(context).pop(true),
                child: Text(loc.logsClearTitle),
              ),
            ],
          ),
        ) ??
        false;
    if (!confirmed) return;

    try {
      await widget.api.delete('/radius/api/logs');
      await _load();
      if (mounted) {
        NotificationHelper.showSuccess(context, loc.logsClearSuccess);
      }
    } on ApiException catch (e) {
      if (mounted) {
        NotificationHelper.showError(context, e.message);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final rows = _filteredLogs;
    return FutureBuilder<void>(
      future: _future,
      builder: (context, snapshot) {
        final loc = AppLocalizations.of(context);
        return Scaffold(
          body: Column(
            children: [
              Padding(
                padding: const EdgeInsets.all(16),
                child: LayoutBuilder(
                  builder: (context, constraints) {
                    final title = Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          loc.logsTitle,
                          style: Theme.of(context).textTheme.headlineSmall
                              ?.copyWith(fontWeight: FontWeight.w900),
                        ),
                        const SizedBox(height: 6),
                        Text(
                          loc.logsSubtitle,
                        ),
                      ],
                    );
                    final actions = Wrap(
                      spacing: 8,
                      runSpacing: 6,
                      crossAxisAlignment: WrapCrossAlignment.center,
                      children: [
                        Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Text(
                              loc.logsAutoRefresh,
                              style: TextStyle(fontSize: 12),
                            ),
                            const SizedBox(width: 4),
                            Switch(
                              value: _autoRefresh,
                              onChanged: (val) =>
                                  setState(() => _autoRefresh = val),
                            ),
                          ],
                        ),
                        ElevatedButton.icon(
                          onPressed: () {
                            setState(() {
                              _future = _load();
                            });
                          },
                          icon: const Icon(Icons.refresh),
                          label: Text(loc.refresh),
                        ),
                        OutlinedButton.icon(
                          onPressed: _clearLogs,
                          icon: const Icon(
                            Icons.delete_sweep,
                            color: Colors.red,
                          ),
                          label: Text(loc.logsClear),
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
                      labelText: loc.logsSearch,
                      prefixIcon: Icon(Icons.search),
                    ),
                    onChanged: (value) => setState(() => _query = value),
                  ),
              ),
              const SizedBox(height: 12),
              Expanded(
                child: snapshot.connectionState == ConnectionState.waiting
                    ? const Center(child: CircularProgressIndicator())
                        : rows.isEmpty
                            ? Center(child: Text(loc.logsNoLogs))
                    : ListView.builder(
                        padding: const EdgeInsets.all(16),
                        itemCount: rows.length,
                        itemBuilder: (context, index) {
                          final log = rows[index];
                          return Card(
                            margin: const EdgeInsets.only(bottom: 12),
                            child: Padding(
                              padding: const EdgeInsets.all(14),
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(
                                    log['message']?.toString() ??
                                        log['event']?.toString() ??
                                        loc.logsFallback,
                                    style: const TextStyle(
                                      fontWeight: FontWeight.w800,
                                    ),
                                  ),
                                  const SizedBox(height: 8),
                                  Text(
                                    log['timestamp']?.toString() ??
                                        log['created_at']?.toString() ??
                                        '',
                                  ),
                                  const SizedBox(height: 8),
                                  StructuredDetailsView(
                                    data: log,
                                    compact: true,
                                    showRawJson: false,
                                    priorityKeys: const [
                                      'message',
                                      'event',
                                      'timestamp',
                                      'created_at',
                                      'user',
                                      'username',
                                    ],
                                  ),
                                ],
                              ),
                            ),
                          );
                        },
                      ),
              ),
            ],
          ),
        );
      },
    );
  }
}
