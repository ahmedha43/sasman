import 'dart:convert';

import 'package:flutter/material.dart';

import '../../config/app_theme.dart';
import '../../services/api_service.dart';

class DashboardScreen extends StatefulWidget {
  const DashboardScreen({super.key, required this.api});

  final ApiService api;

  @override
  State<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends State<DashboardScreen> {
  late Future<_DashboardStats> _future = _load();

  Future<_DashboardStats> _load() async {
    final usersRes = await widget.api.get('/radius/api/users');
    final meRes = await widget.api.get('/radius/api/auth/me');
    final users = _decodeList(usersRes.body);
    final me = _decodeMap(meRes.body);

    int active = 0;
    int online = 0;
    int expired = 0;
    int aboutToExpire = 0;
    final now = DateTime.now();

    for (final item in users) {
      final status = '${item['status'] ?? item['state'] ?? ''}'.toLowerCase();
      if (status.contains('active') ||
          item['disabled'] == false ||
          item['enabled'] == true)
        active++;
      if (status.contains('online') ||
          item['online'] == true ||
          item['session'] != null)
        online++;
      final expiresAt = DateTime.tryParse(
        '${item['expires_at'] ?? item['expiration'] ?? ''}',
      );
      if (expiresAt != null) {
        if (expiresAt.isBefore(now)) {
          expired++;
        } else if (expiresAt.difference(now).inDays <= 3) {
          aboutToExpire++;
        }
      }
    }

    final balance = num.tryParse('${me['balance'] ?? me['credit'] ?? 0}') ?? 0;
    return _DashboardStats(
      totalUsers: users.length,
      activeUsers: active,
      onlineUsers: online,
      expiredUsers: expired,
      aboutToExpire: aboutToExpire,
      balance: balance,
    );
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<_DashboardStats>(
      future: _future,
      builder: (context, snapshot) {
        if (snapshot.connectionState == ConnectionState.waiting) {
          return const Center(child: CircularProgressIndicator());
        }
        if (snapshot.hasError) {
          return Center(
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: Container(
                padding: const EdgeInsets.all(20),
                decoration: AppTheme.premiumCard(),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Icon(
                      Icons.error_outline_rounded,
                      color: AppTheme.danger,
                      size: 48,
                    ),
                    const SizedBox(height: 16),
                    Text(
                      'تعذر تحميل لوحة القيادة',
                      style: Theme.of(context).textTheme.titleLarge?.copyWith(
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                    const SizedBox(height: 12),
                    Text(
                      snapshot.error.toString(),
                      textAlign: TextAlign.center,
                      style: const TextStyle(color: AppTheme.textSecondary),
                    ),
                    const SizedBox(height: 20),
                    ElevatedButton.icon(
                      onPressed: () {
                        setState(() {
                          _future = _load();
                        });
                      },
                      icon: const Icon(Icons.refresh_rounded),
                      label: const Text('أعد المحاولة'),
                    ),
                  ],
                ),
              ),
            ),
          );
        }

        final data = snapshot.data!;
        return RefreshIndicator(
          onRefresh: () {
            final future = _load();
            setState(() {
              _future = future;
            });
            return future;
          },
          child: ListView(
            padding: const EdgeInsets.all(20),
            children: [
              Container(
                padding: const EdgeInsets.all(20),
                decoration: BoxDecoration(
                  gradient: AppTheme.gradientDark,
                  borderRadius: BorderRadius.circular(AppTheme.radiusLg),
                  border: Border.all(
                    color: AppTheme.border.withValues(alpha: 0.3),
                  ),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'لوحة تحكم المدير',
                      style: Theme.of(context).textTheme.headlineMedium
                          ?.copyWith(
                            fontWeight: FontWeight.w900,
                            color: Colors.white,
                          ),
                    ),
                    const SizedBox(height: 8),
                    const Text(
                      'عرض سريع لحالة النظام والإحصائيات الرئيعية لراديوس SASMAN.',
                      style: TextStyle(color: AppTheme.textSecondary),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 24),
              Wrap(
                spacing: 16,
                runSpacing: 16,
                children: [
                  _StatCard(
                    title: 'إجمالي المشتركين',
                    value: data.totalUsers.toString(),
                    icon: Icons.group_rounded,
                    gradient: AppTheme.gradientPurple,
                  ),
                  _StatCard(
                    title: 'المشتركين النشطين',
                    value: data.activeUsers.toString(),
                    icon: Icons.verified_user_rounded,
                    gradient: AppTheme.gradientGreen,
                  ),
                  _StatCard(
                    title: 'المتصلين الآن',
                    value: data.onlineUsers.toString(),
                    icon: Icons.wifi_rounded,
                    gradient: AppTheme.gradientCyan,
                  ),
                  _StatCard(
                    title: 'المنتهين',
                    value: data.expiredUsers.toString(),
                    icon: Icons.warning_rounded,
                    gradient: AppTheme.gradientRed,
                  ),
                  _StatCard(
                    title: 'قريبين من الانتهاء',
                    value: data.aboutToExpire.toString(),
                    icon: Icons.event_note_rounded,
                    gradient: AppTheme.gradientAmber,
                  ),
                  _StatCard(
                    title: 'الرصيد',
                    value: '${data.balance}',
                    icon: Icons.account_balance_wallet_rounded,
                    gradient: AppTheme.gradientBlue,
                  ),
                ],
              ),
            ],
          ),
        );
      },
    );
  }
}

class _StatCard extends StatelessWidget {
  const _StatCard({
    required this.title,
    required this.value,
    required this.icon,
    required this.gradient,
  });

  final String title;
  final String value;
  final IconData icon;
  final Gradient gradient;

  @override
  Widget build(BuildContext context) {
    final width = MediaQuery.sizeOf(context).width;
    final isMobile = width < 640;

    return Container(
      width: isMobile
          ? double.infinity
          : (width - 76) / 2 > 260
              ? 260
              : (width - 76) / 2,
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        gradient: gradient,
        borderRadius: BorderRadius.circular(AppTheme.radiusLg),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.15),
            blurRadius: 10,
            offset: const Offset(0, 4),
          ),
        ],
      ),
      child: Stack(
        children: [
          Positioned(
            left: -10,
            bottom: -10,
            child: Icon(
              icon,
              size: 72,
              color: Colors.white.withValues(alpha: 0.12),
            ),
          ),
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                padding: const EdgeInsets.all(8),
                decoration: BoxDecoration(
                  color: Colors.white.withValues(alpha: 0.2),
                  borderRadius: BorderRadius.circular(AppTheme.radiusMd),
                ),
                child: Icon(icon, color: Colors.white, size: 24),
              ),
              const SizedBox(height: 16),
              Text(
                value,
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 32,
                  fontWeight: FontWeight.w900,
                  height: 1.1,
                ),
              ),
              const SizedBox(height: 8),
              Text(
                title,
                style: TextStyle(
                  color: Colors.white.withValues(alpha: 0.8),
                  fontWeight: FontWeight.w700,
                  fontSize: 14,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

List<Map<String, dynamic>> _decodeList(String body) {
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
  return <Map<String, dynamic>>[];
}

Map<String, dynamic> _decodeMap(String body) {
  final decoded = jsonDecode(body);
  if (decoded is Map) {
    return Map<String, dynamic>.from(decoded);
  }
  return {};
}

class _DashboardStats {
  const _DashboardStats({
    required this.totalUsers,
    required this.activeUsers,
    required this.onlineUsers,
    required this.expiredUsers,
    required this.aboutToExpire,
    required this.balance,
  });

  final int totalUsers;
  final int activeUsers;
  final int onlineUsers;
  final int expiredUsers;
  final int aboutToExpire;
  final num balance;
}