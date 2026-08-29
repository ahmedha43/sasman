import 'dart:convert';

import 'package:flutter/material.dart';


import '../../../l10n/app_localizations.dart';
import '../../../services/api_service.dart';
import '../../../widgets/notification_helper.dart';

class StreamsScreen extends StatefulWidget {
  const StreamsScreen({super.key, required this.api});

  final ApiService api;

  @override
  State<StreamsScreen> createState() => _StreamsScreenState();
}

class _StreamsScreenState extends State<StreamsScreen> {
  late Future<void> _future = _load();
  List<Map<String, dynamic>> _streams = [];
  String _query = '';

  Future<void> _load() async {
    final res = await widget.api.get('/radius/api/streaming');
    final decoded = jsonDecode(res.body);
    _streams = _parseList(decoded);
    setState(() {});
  }

  List<Map<String, dynamic>> _parseList(dynamic decoded) {
    if (decoded is List) {
      return decoded.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
    }
    if (decoded is Map) {
      final list = decoded['data'] ?? decoded['items'] ?? decoded['streams'];
      if (list is List) {
        return list.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
      }
    }
    return [];
  }

  List<Map<String, dynamic>> get _filteredStreams {
    if (_query.isEmpty) return _streams;
    final q = _query.toLowerCase();
    return _streams.where((item) => item.values.any((value) => value != null && value.toString().toLowerCase().contains(q))).toList();
  }

  Future<void> _toggleStream(Map<String, dynamic> stream) async {
    final loc = AppLocalizations.of(context);
    try {
      await widget.api.post('/radius/api/streaming/${stream['id']}/toggle');
      await _load();
      if (context.mounted) {
        NotificationHelper.showSuccess(context, loc.streamsToggleSuccess);
      }
    } on ApiException catch (e) {
      if (context.mounted) {
        NotificationHelper.showError(context, e.message);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final loc = AppLocalizations.of(context);
    final rows = _filteredStreams;
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
                          Text(loc.streamsTitle, style: Theme.of(context).textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.w900)),
                          const SizedBox(height: 6),
                          Text(loc.streamsSubtitle),
                        ],
                      ),
                    ),
                    ElevatedButton.icon(onPressed: () {}, icon: const Icon(Icons.add), label: Text(loc.streamsAddSource)),
                  ],
                ),
              ),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16),
                child: TextField(
                  decoration: InputDecoration(labelText: loc.streamsSearch, prefixIcon: const Icon(Icons.search)),
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
                          ? Center(child: Text(loc.streamsNoSources))
                          : ListView.builder(
                              padding: const EdgeInsets.all(16),
                              itemCount: rows.length,
                              itemBuilder: (context, index) {
                                final stream = rows[index];
                                return Card(
                                  margin: const EdgeInsets.only(bottom: 12),
                                  child: ListTile(
                                    title: Text(stream['name']?.toString() ?? stream['source']?.toString() ?? '-'),
                                    subtitle: Text('${loc.streamsStatus}: ${stream['active'] == true || stream['status'] == 'active' ? loc.streamsActive : loc.streamsInactive}'),
                                    trailing: ElevatedButton(
                                      onPressed: () => _toggleStream(stream),
                                      child: Text(stream['active'] == true || stream['status'] == 'active' ? loc.streamsStop : loc.streamsActivate),
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
