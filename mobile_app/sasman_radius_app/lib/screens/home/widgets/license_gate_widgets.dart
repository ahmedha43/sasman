import 'package:flutter/material.dart';

import '../../../config/app_theme.dart';
import '../../../l10n/app_localizations.dart';

class LicenseGate extends StatelessWidget {
  const LicenseGate({
    super.key,
    required this.message,
    required this.serial,
    required this.expires,
    required this.routerConnected,
    required this.busy,
    required this.licenseKeyController,
    required this.routerAddressController,
    required this.routerUserController,
    required this.routerPassController,
    required this.onRefresh,
    required this.onActivate,
    required this.onConnectRouter,
    required this.onCopy,
    required this.onOpenUrl,
  });

  static const _payments = [
    ('Zain Cash', '07819597948', Icons.account_balance_wallet_rounded),
    ('SuperKey', '9596119421', Icons.key_rounded),
    (
      'USDT TRC20',
      'TUnDec4ULJPCzcQwVyQg4ASRuNwvL5H4mq',
      Icons.currency_exchange_rounded,
    ),
    ('Binance UID', '500747355', Icons.badge_rounded),
  ];

  final String message;
  final String serial;
  final String expires;
  final bool routerConnected;
  final bool busy;
  final TextEditingController licenseKeyController;
  final TextEditingController routerAddressController;
  final TextEditingController routerUserController;
  final TextEditingController routerPassController;
  final Future<void> Function() onRefresh;
  final Future<void> Function() onActivate;
  final Future<void> Function() onConnectRouter;
  final Future<void> Function(String label, String value) onCopy;
  final Future<void> Function(String url) onOpenUrl;

  @override
  Widget build(BuildContext context) {
    final loc = AppLocalizations.of(context);
    return RefreshIndicator(
      onRefresh: onRefresh,
      child: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          Container(
            padding: const EdgeInsets.all(20),
            decoration: AppTheme.premiumCard(
              gradient: const LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                colors: [Color(0xFF3B0B12), Color(0xFF111827)],
              ),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    TweenAnimationBuilder<double>(
                      tween: Tween(begin: 0.9, end: 1),
                      duration: const Duration(milliseconds: 900),
                      curve: Curves.easeInOut,
                      builder: (context, scale, child) =>
                          Transform.scale(scale: scale, child: child),
                      child: Container(
                        width: 58,
                        height: 58,
                        decoration: BoxDecoration(
                          color: AppTheme.danger.withValues(alpha: 0.18),
                          borderRadius: BorderRadius.circular(16),
                          border: Border.all(
                            color: AppTheme.danger.withValues(alpha: 0.5),
                          ),
                        ),
                        child: const Icon(
                          Icons.gpp_bad_rounded,
                          color: AppTheme.dangerLight,
                          size: 34,
                        ),
                      ),
                    ),
                    const SizedBox(width: 14),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                  Text(
                    loc.systemNotActivated,
                    style: Theme.of(context)
                        .textTheme
                        .headlineSmall
                        ?.copyWith(
                          fontWeight: FontWeight.w900,
                          color: Theme.of(context).brightness == Brightness.dark ? Colors.white : AppTheme.textDark,
                        ),
                  ),
                          const SizedBox(height: 4),
                          Text(
                            message.isEmpty
                                ? loc.activateLicenseInfo
                                : message,
                            style: const TextStyle(
                              color: AppTheme.textSecondary,
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 16),
                Wrap(
                  spacing: 10,
                  runSpacing: 10,
                  children: [
                    _InfoChip(
                      icon: Icons.router_rounded,
                      label: routerConnected
                          ? loc.routerConnected
                          : loc.routerNotConnected,
                    ),
                    if (serial.isNotEmpty)
                      _InfoChip(
                        icon: Icons.fingerprint_rounded,
                        label: 'Serial: $serial',
                      ),
                    if (expires.isNotEmpty)
                      _InfoChip(
                        icon: Icons.event_rounded,
                        label: 'Expires: $expires',
                      ),
                  ],
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          _GateCard(
            title: loc.contactDeveloper,
            icon: Icons.support_agent_rounded,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  loc.developerName,
                  style: const TextStyle(
                    fontWeight: FontWeight.w900,
                    fontSize: 15,
                  ),
                ),
                const SizedBox(height: 12),
                Row(
                  children: [
                    Expanded(
                      child: ElevatedButton.icon(
                        style: ElevatedButton.styleFrom(
                          backgroundColor: const Color(0xFF25D366),
                        ),
                        onPressed: () =>
                            onOpenUrl('https://wa.me/qr/U22CICF7BZKDN1'),
                        icon: const Icon(Icons.chat_rounded),
                        label: Text(loc.whatsapp),
                      ),
                    ),
                    const SizedBox(width: 10),
                    Expanded(
                      child: ElevatedButton.icon(
                        style: ElevatedButton.styleFrom(
                          backgroundColor: const Color(0xFF0088CC),
                        ),
                        onPressed: () => onOpenUrl('https://t.me/Aa79n'),
                        icon: const Icon(Icons.send_rounded),
                        label: Text(loc.telegram),
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          _GateCard(
            title: loc.paymentMethods,
            icon: Icons.payments_rounded,
            child: Column(
              children: [
                for (final payment in _payments) ...[
                  _PaymentRow(
                    title: payment.$1,
                    value: payment.$2,
                    icon: payment.$3,
                    onCopy: () => onCopy(payment.$1, payment.$2),
                  ),
                  if (payment != _payments.last) const Divider(height: 18),
                ],
              ],
            ),
          ),
          if (!routerConnected) ...[
            const SizedBox(height: 16),
            _GateCard(
              title: loc.connectRouter,
              icon: Icons.router_rounded,
              child: Column(
                children: [
                  TextField(
                    controller: routerAddressController,
                    decoration: InputDecoration(
                      labelText: loc.routerIp,
                      hintText: loc.routerIpHint,
                    ),
                  ),
                  const SizedBox(height: 10),
                  TextField(
                    controller: routerUserController,
                    decoration: InputDecoration(
                      labelText: loc.username,
                    ),
                  ),
                  const SizedBox(height: 10),
                  TextField(
                    controller: routerPassController,
                    obscureText: true,
                    decoration: InputDecoration(
                      labelText: loc.password,
                    ),
                  ),
                  const SizedBox(height: 12),
                  SizedBox(
                    width: double.infinity,
                    child: OutlinedButton.icon(
                      onPressed: busy ? null : onConnectRouter,
                      icon: const Icon(Icons.settings_ethernet_rounded),
                      label: Text(loc.connectAndFetchSerial),
                    ),
                  ),
                ],
              ),
            ),
          ],
          const SizedBox(height: 16),
          _GateCard(
            title: loc.activateLicense,
            icon: Icons.verified_user_rounded,
            child: Column(
              children: [
                TextField(
                  controller: licenseKeyController,
                  minLines: 3,
                  maxLines: 6,
                  decoration: InputDecoration(
                    labelText: loc.activationKey,
                    hintText: loc.activationKeyHint,
                  ),
                ),
                const SizedBox(height: 12),
                SizedBox(
                  width: double.infinity,
                  child: ElevatedButton.icon(
                    onPressed: busy ? null : onActivate,
                    icon: busy
                        ? const SizedBox(
                            width: 18,
                            height: 18,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Icon(Icons.lock_open_rounded),
                    label: Text(busy ? loc.activating : loc.activateLicense),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _GateCard extends StatelessWidget {
  const _GateCard({
    required this.title,
    required this.icon,
    required this.child,
  });

  final String title;
  final IconData icon;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: AppTheme.premiumCard(),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(icon, color: AppTheme.primaryLight),
              const SizedBox(width: 8),
              Text(
                title,
                style: const TextStyle(
                  fontWeight: FontWeight.w900,
                  fontSize: 16,
                ),
              ),
            ],
          ),
          const SizedBox(height: 14),
          child,
        ],
      ),
    );
  }
}

class _InfoChip extends StatelessWidget {
  const _InfoChip({required this.icon, required this.label});

  final IconData icon;
  final String label;

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
      decoration: BoxDecoration(
        color: isDark ? Colors.white.withValues(alpha: 0.08) : AppTheme.bgSurfaceLight,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: isDark ? Colors.white.withValues(alpha: 0.08) : AppTheme.borderLightTheme),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 16, color: AppTheme.textSecondary),
          const SizedBox(width: 6),
          Text(
            label,
            style: const TextStyle(
              color: AppTheme.textSecondary,
              fontWeight: FontWeight.w800,
            ),
          ),
        ],
      ),
    );
  }
}

class _PaymentRow extends StatelessWidget {
  const _PaymentRow({
    required this.title,
    required this.value,
    required this.icon,
    required this.onCopy,
  });

  final String title;
  final String value;
  final IconData icon;
  final VoidCallback onCopy;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Icon(icon, color: AppTheme.accentLight),
        const SizedBox(width: 10),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(title, style: const TextStyle(fontWeight: FontWeight.w900)),
              const SizedBox(height: 2),
              SelectableText(
                value,
                style: const TextStyle(
                  color: AppTheme.textSecondary,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ],
          ),
        ),
        IconButton(
          onPressed: onCopy,
          icon: const Icon(Icons.copy_rounded, size: 18),
          tooltip: 'Copy',
          color: AppTheme.textSecondary,
        ),
      ],
    );
  }
}