import 'dart:convert';

import 'package:flutter/material.dart';

import '../config/app_theme.dart';

class StructuredDetailsView extends StatelessWidget {
  const StructuredDetailsView({
    super.key,
    required this.data,
    this.title,
    this.priorityKeys = const [],
    this.compact = false,
    this.showRawJson = true,
  });

  final Map<String, dynamic> data;
  final String? title;
  final List<String> priorityKeys;
  final bool compact;
  final bool showRawJson;

  static const _sensitiveKeys = {
    'pass',
    'password',
    'secret',
    'token',
    'api_token',
    'access_token',
    'refresh_token',
  };

  static const _labels = {
    'user': 'اسم المستخدم',
    'username': 'اسم المستخدم',
    'full_name': 'الاسم الكامل',
    'name': 'الاسم',
    'phone': 'الهاتف',
    'profile': 'الباقة',
    'profile_name': 'الباقة',
    'status': 'الحالة',
    'state': 'الحالة',
    'enabled': 'مفعل',
    'disabled': 'معطل',
    'online': 'متصل',
    'expires_at': 'تاريخ الانتهاء',
    'expiration': 'تاريخ الانتهاء',
    'created_at': 'تاريخ الإنشاء',
    'updated_at': 'آخر تحديث',
    'balance': 'الرصيد',
    'credit': 'الرصيد',
    'debt': 'الدين',
    'download': 'التحميل',
    'upload': 'الرفع',
    'validity': 'الصلاحية',
    'validity_days': 'الصلاحية بالأيام',
    'nas_ip': 'NAS IP',
    'ip': 'IP',
    'mac': 'MAC',
    'last_login': 'آخر دخول',
    'last_seen': 'آخر ظهور',
    'session': 'الجلسة',
    'message': 'الرسالة',
    'event': 'الحدث',
    'timestamp': 'الوقت',
  };

  @override
  Widget build(BuildContext context) {
    final entries = _orderedEntries(data);
    if (entries.isEmpty) {
      return _emptyState(context);
    }

    final children = <Widget>[
      if (title != null) ...[
        Text(
          title!,
          style: Theme.of(
            context,
          ).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w900),
        ),
        const SizedBox(height: 12),
      ],
      _DetailsGrid(entries: entries, compact: compact),
    ];

    if (showRawJson) {
      children.addAll([const SizedBox(height: 12), _RawJsonPanel(data: data)]);
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: children,
    );
  }

  Widget _emptyState(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(18),
      decoration: BoxDecoration(
        color: AppTheme.bgSurface,
        borderRadius: BorderRadius.circular(AppTheme.radiusMd),
        border: Border.all(color: AppTheme.border.withValues(alpha: 0.3)),
      ),
      child: Text(
        'لا توجد تفاصيل قابلة للعرض',
        textAlign: TextAlign.center,
        style: Theme.of(context).textTheme.bodyMedium?.copyWith(
          color: AppTheme.textSecondary,
          fontWeight: FontWeight.w700,
        ),
      ),
    );
  }

  List<MapEntry<String, dynamic>> _orderedEntries(Map<String, dynamic> source) {
    final visible = source.entries.where((entry) => !_isNoise(entry)).toList();
    final preferred = <MapEntry<String, dynamic>>[];
    final rest = <MapEntry<String, dynamic>>[];

    for (final key in priorityKeys) {
      final match = visible.where((entry) => entry.key == key);
      preferred.addAll(match);
    }

    final preferredKeys = preferred.map((entry) => entry.key).toSet();
    for (final entry in visible) {
      if (!preferredKeys.contains(entry.key)) rest.add(entry);
    }
    rest.sort((a, b) => _labelFor(a.key).compareTo(_labelFor(b.key)));
    return [...preferred, ...rest];
  }

  bool _isNoise(MapEntry<String, dynamic> entry) {
    final value = entry.value;
    if (value == null) return true;
    if (value is String && value.trim().isEmpty) return true;
    if (value is Iterable && value.isEmpty) return true;
    if (value is Map && value.isEmpty) return true;
    return false;
  }

  static String _labelFor(String key) {
    return _labels[key] ?? key.replaceAll('_', ' ');
  }

  static String _safeValue(String key, dynamic value) {
    if (_sensitiveKeys.contains(key.toLowerCase())) {
      final text = value?.toString() ?? '';
      return text.isEmpty ? 'غير محدد' : 'مخفي';
    }
    if (value is bool) return value ? 'نعم' : 'لا';
    if (value is num) return value.toString();
    if (value is Iterable) {
      return value.map((item) => item.toString()).join('، ');
    }
    if (value is Map) {
      return value.entries
          .take(4)
          .map((entry) => '${_labelFor(entry.key.toString())}: ${entry.value}')
          .join('، ');
    }

    final text = value?.toString().trim() ?? '-';
    if (text.length <= 160) return text;
    return '${text.substring(0, 157)}...';
  }
}

class _DetailsGrid extends StatelessWidget {
  const _DetailsGrid({required this.entries, required this.compact});

  final List<MapEntry<String, dynamic>> entries;
  final bool compact;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final columns = constraints.maxWidth >= 680
            ? 3
            : constraints.maxWidth >= 460
            ? 2
            : 1;
        return GridView.builder(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          itemCount: entries.length,
          gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
            crossAxisCount: columns,
            crossAxisSpacing: 10,
            mainAxisSpacing: 10,
            mainAxisExtent: compact ? 76 : 86,
          ),
          itemBuilder: (context, index) {
            final entry = entries[index];
            return _DetailTile(
              label: StructuredDetailsView._labelFor(entry.key),
              value: StructuredDetailsView._safeValue(entry.key, entry.value),
            );
          },
        );
      },
    );
  }
}

class _DetailTile extends StatelessWidget {
  const _DetailTile({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: AppTheme.bgSurface,
        borderRadius: BorderRadius.circular(AppTheme.radiusMd),
        border: Border.all(color: AppTheme.border.withValues(alpha: 0.3)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            label,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: Theme.of(context).textTheme.labelMedium?.copyWith(
              color: AppTheme.textMuted,
              fontWeight: FontWeight.w700,
              fontSize: 11,
            ),
          ),
          const SizedBox(height: 7),
          Expanded(
            child: SelectableText(
              value,
              maxLines: 2,
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                color: AppTheme.textPrimary,
                fontWeight: FontWeight.w800,
                height: 1.25,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _RawJsonPanel extends StatelessWidget {
  const _RawJsonPanel({required this.data});

  final Map<String, dynamic> data;

  @override
  Widget build(BuildContext context) {
    return ExpansionTile(
      tilePadding: EdgeInsets.zero,
      childrenPadding: EdgeInsets.zero,
      iconColor: AppTheme.textSecondary,
      collapsedIconColor: AppTheme.textMuted,
      title: Text(
        'البيانات الخام',
        style: Theme.of(
          context,
        ).textTheme.titleSmall?.copyWith(
          fontWeight: FontWeight.w900,
          color: AppTheme.textSecondary,
        ),
      ),
      children: [
        Container(
          width: double.infinity,
          constraints: const BoxConstraints(maxHeight: 260),
          padding: const EdgeInsets.all(14),
          decoration: BoxDecoration(
            color: AppTheme.bgDark,
            borderRadius: BorderRadius.circular(AppTheme.radiusMd),
            border: Border.all(color: AppTheme.border.withValues(alpha: 0.3)),
          ),
          child: SingleChildScrollView(
            child: SelectableText(
              const JsonEncoder.withIndent('  ').convert(data),
              textDirection: TextDirection.ltr,
              style: const TextStyle(
                color: AppTheme.accent,
                fontFamily: 'monospace',
                fontSize: 12,
              ),
            ),
          ),
        ),
      ],
    );
  }
}

class DetailsDialog extends StatelessWidget {
  const DetailsDialog({
    super.key,
    required this.title,
    required this.data,
    this.priorityKeys = const [],
  });

  final String title;
  final Map<String, dynamic> data;
  final List<String> priorityKeys;

  @override
  Widget build(BuildContext context) {
    final size = MediaQuery.sizeOf(context);
    return AlertDialog(
      titlePadding: const EdgeInsets.fromLTRB(24, 22, 24, 0),
      contentPadding: const EdgeInsets.fromLTRB(24, 16, 24, 8),
      actionsPadding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
      title: Text(title),
      content: SizedBox(
        width: size.width < 640 ? size.width * 0.92 : 760,
        child: SingleChildScrollView(
          child: StructuredDetailsView(data: data, priorityKeys: priorityKeys),
        ),
      ),
      actions: [
        TextButton.icon(
          onPressed: () => Navigator.of(context).pop(),
          icon: const Icon(Icons.close),
          label: const Text('إغلاق'),
        ),
      ],
    );
  }
}
