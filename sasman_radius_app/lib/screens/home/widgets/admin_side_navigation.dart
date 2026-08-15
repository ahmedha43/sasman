import 'package:flutter/material.dart';

import '../../../config/app_theme.dart';
import 'brand_header.dart';
import 'section_model.dart';
import 'side_nav_item.dart';

class AdminSideNavigation extends StatelessWidget {
  const AdminSideNavigation({
    super.key,
    required this.sections,
    required this.selectedIndex,
    required this.onDestinationSelected,
  });

  final List<Section> sections;
  final int selectedIndex;
  final ValueChanged<int> onDestinationSelected;

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Container(
      width: 276,
      color: isDark ? AppTheme.surface : AppTheme.bgSurfaceLight,
      child: SafeArea(
        top: false,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            const BrandHeader(compact: false),
            Expanded(
              child: ListView.separated(
                padding: const EdgeInsets.fromLTRB(12, 4, 12, 16),
                itemCount: sections.length,
                separatorBuilder: (_, __) => const SizedBox(height: 4),
                itemBuilder: (context, index) {
                  final section = sections[index];
                  final selected = index == selectedIndex;
                  return SideNavItem(
                    icon: section.icon,
                    label: section.title,
                    selected: selected,
                    onTap: () => onDestinationSelected(index),
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );
  }
}