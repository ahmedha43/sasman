import 'package:flutter/material.dart';

class Section {
  const Section(this.title, this.icon, this.child);

  final String title;
  final IconData icon;
  final Widget child;
}