class DashboardStats {
  const DashboardStats({
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

  factory DashboardStats.empty() {
    return const DashboardStats(
      totalUsers: 0,
      activeUsers: 0,
      onlineUsers: 0,
      expiredUsers: 0,
      aboutToExpire: 0,
      balance: 0,
    );
  }
}
