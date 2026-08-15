/// Central aggregator for all translations.
///
/// This file contains NO translation strings. It only imports each
/// per-language file and registers it with [AppStrings], so adding a
/// new language is as simple as creating `strings/app_strings_<code>.dart`
/// and adding a single `register` line below.
import 'app_strings_model.dart';
import 'strings/app_strings_ar.dart';
import 'strings/app_strings_en.dart';
import 'strings/app_strings_tr.dart';
import 'strings/app_strings_ku.dart';
import 'strings/app_strings_fa.dart';
import 'strings/app_strings_fr.dart';
import 'strings/app_strings_de.dart';
import 'strings/app_strings_es.dart';
import 'strings/app_strings_ru.dart';

void registerAllAppStrings() {
  AppStrings.registerLocale('ar', arAppStrings);
  AppStrings.registerLocale('en', enAppStrings);
  AppStrings.registerLocale('tr', trAppStrings);
  AppStrings.registerLocale('ku', kuAppStrings);
  AppStrings.registerLocale('fa', faAppStrings);
  AppStrings.registerLocale('fr', frAppStrings);
  AppStrings.registerLocale('de', deAppStrings);
  AppStrings.registerLocale('es', esAppStrings);
  AppStrings.registerLocale('ru', ruAppStrings);
}

const List<String> supportedLanguageCodes = ['ar', 'en', 'tr', 'ku', 'fa', 'fr', 'de', 'es', 'ru'];
