import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sasman_radius_app/screens/auth/server_setup_screen.dart';

void main() {
  testWidgets('shows server setup form', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Directionality(
          textDirection: TextDirection.rtl,
          child: ServerSetupScreen(),
        ),
      ),
    );

    expect(find.text('إعداد اتصال SASMAN'), findsOneWidget);
    expect(find.byIcon(Icons.router), findsOneWidget);
  });
}
