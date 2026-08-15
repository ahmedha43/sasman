import 'package:flutter/material.dart';
import 'app_strings.dart';
import 'app_strings_model.dart';

class AppLocalizations {
  final Locale locale;
  late final AppStrings _strings;

  AppLocalizations(this.locale) {
    // Populate the [AppStrings] registry (idempotent) so every locale can be
    // resolved. Called here instead of at top level because Dart only allows
    // declarations — not statements — at the top level of a library.
    registerAllAppStrings();
    _strings = AppStrings.fromLocale(locale.languageCode);
  }

  static AppLocalizations of(BuildContext context) {

    return Localizations.of<AppLocalizations>(context, AppLocalizations)!;
  }

  static const LocalizationsDelegate<AppLocalizations> delegate =
      _AppLocalizationsDelegate();

  String get appName => _strings.appName;
  String get appSubtitle => _strings.appSubtitle;

  // Splash
  String get splashTitle => _strings.splashTitle;
  String get splashSubtitle => _strings.splashSubtitle;

  // Auth
  String get loginTitle => _strings.loginTitle;
  String get loginButton => _strings.loginButton;
  String get changeServer => _strings.changeServer;
  String get username => _strings.username;
  String get password => _strings.password;
  String get loginSuccess => _strings.loginSuccess;
  String get loginFailed => _strings.loginFailed;
  String get serverSetupTitle => _strings.serverSetupTitle;
  String get serverAddress => _strings.serverAddress;
  String get serverAddressHint => _strings.serverAddressHint;
  String get cloudflareLink => _strings.cloudflareLink;
  String get cloudflareHint => _strings.cloudflareHint;
  String get cloudflareInfo => _strings.cloudflareInfo;
  String get testConnection => _strings.testConnection;
  String get saveAndContinue => _strings.saveAndContinue;
  String get serverSaveSuccess => _strings.serverSaveSuccess;
  String get connectionSuccess => _strings.connectionSuccess;
  String get serverInfo => _strings.serverInfo;

  // Navigation
  String get navDashboard => _strings.navDashboard;
  String get navUsers => _strings.navUsers;
  String get navAgents => _strings.navAgents;
  String get navProfiles => _strings.navProfiles;
  String get navVouchers => _strings.navVouchers;
  String get navNas => _strings.navNas;
  String get navStreams => _strings.navStreams;
  String get navWhatsApp => _strings.navWhatsApp;
  String get navLogs => _strings.navLogs;
  String get navAdvancedSettings => _strings.navAdvancedSettings;
  String get navSettings => _strings.navSettings;

  // AppBar
  String get refresh => _strings.refresh;
  String get logout => _strings.logout;
  String get logoutSuccess => _strings.logoutSuccess;
  String get superAdminBadge => _strings.superAdminBadge;
  String get balanceCurrency => _strings.balanceCurrency;

  // Dashboard
  String get dashboardTitle => _strings.dashboardTitle;
  String get dashboardSubtitle => _strings.dashboardSubtitle;
  String get totalUsers => _strings.totalUsers;
  String get activeUsers => _strings.activeUsers;
  String get onlineUsers => _strings.onlineUsers;
  String get expiredUsers => _strings.expiredUsers;
  String get aboutToExpire => _strings.aboutToExpire;
  String get balance => _strings.balance;
  String get dashboardLoadError => _strings.dashboardLoadError;
  String get retry => _strings.retry;

  // Users
  String get usersTitle => _strings.usersTitle;
  String get usersSubtitle => _strings.usersSubtitle;
  String get addUser => _strings.addUser;
  String get searchUsers => _strings.searchUsers;
  String get filterAll => _strings.filterAll;
  String get filterActive => _strings.filterActive;
  String get filterExpired => _strings.filterExpired;
  String get filterOnline => _strings.filterOnline;
  String get filterAboutToExpire => _strings.filterAboutToExpire;
  String get noResults => _strings.noResults;
  String get addUserTitle => _strings.addUserTitle;
  String get editUserTitle => _strings.editUserTitle;
  String get fullName => _strings.fullName;
  String get phone => _strings.phone;
  String get profile => _strings.profile;
  String get expiryDate => _strings.expiryDate;
  String get expiryDateHint => _strings.expiryDateHint;
  String get cancel => _strings.cancel;
  String get save => _strings.save;
  String get createUser => _strings.createUser;
  String get saveChanges => _strings.saveChanges;
  String get userCreated => _strings.userCreated;
  String get userUpdated => _strings.userUpdated;
  String get disconnectUser => _strings.disconnectUser;
  String get disconnectSuccess => _strings.disconnectSuccess;
  String get toggleStatus => _strings.toggleStatus;
  String get toggleSuccess => _strings.toggleSuccess;
  String get renewSubscription => _strings.renewSubscription;
  String get newProfile => _strings.newProfile;
  String get paidAmount => _strings.paidAmount;
  String get paidAmountInfo => _strings.paidAmountInfo;
  String get renew => _strings.renew;
  String get renewSuccess => _strings.renewSuccess;
  String get deleteUserTitle => _strings.deleteUserTitle;
  String get deleteUserConfirm => _strings.deleteUserConfirm;
  String get delete => _strings.delete;
  String get deleteSuccess => _strings.deleteSuccess;
  String get enableUser => _strings.enableUser;
  String get disableUser => _strings.disableUser;
  String get addDebt => _strings.addDebt;
  String get payDebt => _strings.payDebt;
  String get amount => _strings.amount;
  String get amountHint => _strings.amountHint;
  String get notes => _strings.notes;
  String get notesHint => _strings.notesHint;
  String get enterValidAmount => _strings.enterValidAmount;
  String get debtAdded => _strings.debtAdded;
  String get debtPaid => _strings.debtPaid;
  String get debtLabel => _strings.debtLabel;

  // ── Users (details/cards/transactions) ──
  String get txAddDebtTitle => _strings.txAddDebtTitle;
  String get txPayDebtTitle => _strings.txPayDebtTitle;
  String get txSubscriber => _strings.txSubscriber;
  String get txAddDebtBtn => _strings.txAddDebtBtn;
  String get txPayDebtBtn => _strings.txPayDebtBtn;
  String get cardName => _strings.cardName;
  String get cardProfile => _strings.cardProfile;
  String get cardExpiry => _strings.cardExpiry;
  String get cardBalance => _strings.cardBalance;
  String get editBtn => _strings.editBtn;
  String get details => _strings.details;
  String get disconnectBtn => _strings.disconnectBtn;
  String get detailsTitle => _strings.detailsTitle;
  String get tabInfo => _strings.tabInfo;
  String get tabFinancial => _strings.tabFinancial;
  String get tabSessions => _strings.tabSessions;
  String get basicInfo => _strings.basicInfo;
  String get infoUser => _strings.infoUser;
  String get infoCreatedAt => _strings.infoCreatedAt;
  String get infoStatus => _strings.infoStatus;
  String get statusOnline => _strings.statusOnline;
  String get statusStale => _strings.statusStale;
  String get statusOffline => _strings.statusOffline;
  String get statusExpired => _strings.statusExpired;
  String get statusExpiredOnline => _strings.statusExpiredOnline;
  String get currentSessionInfo => _strings.currentSessionInfo;
  String get sessIp => _strings.sessIp;
  String get sessDownload => _strings.sessDownload;
  String get sessUpload => _strings.sessUpload;
  String get sessDuration => _strings.sessDuration;
  String get sessMac => _strings.sessMac;
  String get sessNasIp => _strings.sessNasIp;
  String get noActiveSession => _strings.noActiveSession;
  String get currentBalance => _strings.currentBalance;
  String get financialRecords => _strings.financialRecords;
  String get txCount => _strings.txCount;
  String get noFinancialRecords => _strings.noFinancialRecords;
  String get txDebtType => _strings.txDebtType;
  String get txPayType => _strings.txPayType;
  String get sessionHistory => _strings.sessionHistory;
  String get sessionCount => _strings.sessionCount;
  String get noSessionHistory => _strings.noSessionHistory;
  String get startedAt => _strings.startedAt;
  String get sessionActive => _strings.sessionActive;
  String get endedAt => _strings.endedAt;
  String get dayUnit => _strings.dayUnit;
  String get durSec => _strings.durSec;
  String get durMin => _strings.durMin;
  String get durHour => _strings.durHour;
  String get invalidData => _strings.invalidData;
  String get subscriberDetailsError => _strings.subscriberDetailsError;

  // License
  String get licenseActivation => _strings.licenseActivation;
  String get systemNotActivated => _strings.systemNotActivated;
  String get activateLicenseInfo => _strings.activateLicenseInfo;
  String get routerConnected => _strings.routerConnected;
  String get routerNotConnected => _strings.routerNotConnected;
  String get contactDeveloper => _strings.contactDeveloper;
  String get developerName => _strings.developerName;
  String get whatsapp => _strings.whatsapp;
  String get telegram => _strings.telegram;
  String get paymentMethods => _strings.paymentMethods;
  String get connectRouter => _strings.connectRouter;
  String get routerIp => _strings.routerIp;
  String get routerIpHint => _strings.routerIpHint;
  String get connectAndFetchSerial => _strings.connectAndFetchSerial;
  String get connectRouterSuccess => _strings.connectRouterSuccess;
  String get activationKey => _strings.activationKey;
  String get activationKeyHint => _strings.activationKeyHint;
  String get activateLicense => _strings.activateLicense;
  String get activating => _strings.activating;
  String get activateSuccess => _strings.activateSuccess;
  String get enterLicenseKey => _strings.enterLicenseKey;
  String get enterRouterAddress => _strings.enterRouterAddress;
  String get copiedLabel => _strings.copiedLabel;
  String get failedOpenUrl => _strings.failedOpenUrl;

  // Common
  String get loading => _strings.loading;
  String get error => _strings.error;
  String get success => _strings.success;
  String get currency => _strings.currency;

  // ── Agents ──
  String get agentsTitle => _strings.agentsTitle;
  String get agentsSubtitle => _strings.agentsSubtitle;
  String get addAgent => _strings.addAgent;
  String get searchAgents => _strings.searchAgents;
  String get noAgents => _strings.noAgents;
  String get addNewAgent => _strings.addNewAgent;
  String get role => _strings.role;
  String get roleAgent => _strings.roleAgent;
  String get roleSubAdmin => _strings.roleSubAdmin;
  String get roleSuperAdmin => _strings.roleSuperAdmin;
  String get extraPermissions => _strings.extraPermissions;
  String get manageProfiles => _strings.manageProfiles;
  String get manageProfilesDesc => _strings.manageProfilesDesc;
  String get manageNas => _strings.manageNas;
  String get manageNasDesc => _strings.manageNasDesc;
  String get emailLabel => _strings.email;
  String get registerBtn => _strings.register;
  String get agentRegistered => _strings.agentRegistered;
  String get deleteAgentLabel => _strings.deleteAgent;
  String get deleteAgentConfirm => _strings.deleteAgentConfirm;
  String get agentDeleted => _strings.agentDeleted;
  String get rechargeBalance => _strings.rechargeBalance;
  String get withdrawBalance => _strings.withdrawBalance;
  String get charge => _strings.charge;
  String get withdrawBtn => _strings.withdraw;
  String get balanceCharged => _strings.balanceCharged;
  String get balanceWithdrawn => _strings.balanceWithdrawn;
  String get enterValidAmount2 => _strings.enterValidAmount2;
  String get financialTransactions => _strings.financialTransactions;
  String get transactionsHistory => _strings.transactionsHistory;
  String get depositWithdrawSales => _strings.depositWithdrawSales;
  String get noTransactions => _strings.noTransactions;
  String get balanceLabel => _strings.balanceLabel;
  String get dateLabel => _strings.date;
  String get fillAllFields => _strings.fillAllFields;

  // ── NAS ──
  String get editNas => _strings.editNas;
  String get addNas => _strings.addNas;
  String get clientIp => _strings.clientIp;
  String get nasName => _strings.nasName;
  String get radiusSecret => _strings.radiusSecret;
  String get nasIp => _strings.nasIp;
  String get visibleToAllAgents => _strings.visibleToAllAgents;
  String get nasAdded => _strings.nasAdded;
  String get nasUpdated => _strings.nasUpdated;
  String get saveChanges2 => _strings.saveChanges2;
  String get addRouter => _strings.addRouter;
  String get deleteRouter => _strings.deleteRouter;
  String get deleteRouterConfirm => _strings.deleteRouterConfirm;
  String get nasDeleted => _strings.nasDeleted;
  String get quickSetup => _strings.quickSetup;
  String get quickSetupConfirm => _strings.quickSetupConfirm;
  String get quickSetupSuccess => _strings.quickSetupSuccess;
  String get autoConnect => _strings.autoConnect;
  String get autoConnectConfirm => _strings.autoConnectConfirm;
  String get autoConnectSuccess => _strings.autoConnectSuccess;
  String get addBypassDevice => _strings.addBypassDevice;
  String get deviceIp => _strings.deviceIp;
  String get deviceName => _strings.deviceName;
  String get bypassAdded => _strings.bypassAdded;
  String get deleteBypass => _strings.deleteBypass;
  String get deleteBypassConfirm => _strings.deleteBypassConfirm;
  String get bypassDeleted => _strings.bypassDeleted;
  String get tabNasDevices => _strings.tabNasDevices;
  String get tabGlobalBypass => _strings.tabGlobalBypass;
  String get tabBypassList => _strings.tabBypassList;
  String get nasSubtitle => _strings.nasSubtitle;
  String get searchNas => _strings.searchNas;
  String get noNas => _strings.noNas;
  String get addNasDevice => _strings.addNasDevice;
  String get quickSetupBtn => _strings.quickSetupBtn;
  String get connectRouterBtn => _strings.connectRouterBtn;
  String get bypassDevicesTitle => _strings.bypassDevicesTitle;
  String get bypassDevicesSubtitle => _strings.bypassDevicesSubtitle;
  String get searchBypassList => _strings.searchBypassList;
  String get noBypassDevices => _strings.noBypassDevices;
  String get globalBypassEnabled => _strings.globalBypassEnabled;
  String get globalBypassDisabled => _strings.globalBypassDisabled;
  String get globalBypassDescEnabled => _strings.globalBypassDescEnabled;
  String get globalBypassDescDisabled => _strings.globalBypassDescDisabled;
  String get enableBypass => _strings.enableBypass;
  String get disableBypass => _strings.disableBypass;
  String get bypassEnableSuccess => _strings.bypassEnableSuccess;
  String get bypassDisableSuccess => _strings.bypassDisableSuccess;
  String get bypassWarningTitle => _strings.bypassWarningTitle;
  String get bypassWarningText => _strings.bypassWarningText;
  String get disableGlobalBypass => _strings.disableGlobalBypass;
  String get enableGlobalBypass => _strings.enableGlobalBypass;
  String get bypassWarningImportant => _strings.bypassWarningImportant;
  String get addDevice => _strings.addDevice;
  String get nasNameLabel => _strings.nasNameLabel;
  String get nasIpLabel => _strings.nasIpLabel;
  String get bypassEnableWarning => _strings.bypassWarningText;
  String get bypassDisableConfirm => _strings.bypassDisableConfirm;
  String get bypassEnableTitle => _strings.bypassWarningTitle;
  String get bypassDisableTitle => _strings.disableGlobalBypass;

  // ── Profiles ──
  String get profilesTitle => _strings.profilesTitle;
  String get profilesSubtitle => _strings.profilesSubtitle;
  String get addProfile => _strings.addProfile;
  String get editProfile => _strings.editProfile;
  String get addNewProfile => _strings.addNewProfile;
  String get profileName => _strings.profileName;
  String get downloadSpeed => _strings.downloadSpeed;
  String get uploadSpeed => _strings.uploadSpeed;
  String get mikrotikLinkType => _strings.mikrotikLinkType;
  String get noLink => _strings.noLink;
  String get poolName => _strings.poolName;
  String get groupName => _strings.groupName;
  String get validityDays => _strings.validityDays;
  String get userPrice => _strings.userPrice;
  String get agentPrice => _strings.agentPrice;
  String get simultaneousDevices => _strings.simultaneousDevices;
  String get expiredTransfer => _strings.expiredTransfer;
  String get noTransfer => _strings.noTransfer;
  String get expiredPoolLabel => _strings.expiredPoolLabel;
  String get expiredPoolName => _strings.expiredPoolName;
  String get expiredProfileLabel => _strings.expiredProfileLabel;
  String get profileNameRequired => _strings.profileNameRequired;
  String get profileCreated => _strings.profileCreated;
  String get createProfile => _strings.createProfile;
  String get deleteProfile => _strings.deleteProfile;
  String get deleteProfileConfirm => _strings.deleteProfileConfirm;
  String get profileDeleted => _strings.profileDeleted;
  String get noProfiles => _strings.noProfiles;
  String get searchProfiles => _strings.searchProfiles;
  String get speedLabel => _strings.speedLabel;
  String get validityLabel => _strings.validityLabel;
  String get usersLabel => _strings.usersLabel;

  // ── WhatsApp ──
  String get waTitle => _strings.waTitle;
  String get waSubtitle => _strings.waSubtitle;
  String get waConfigTitle => _strings.waConfigTitle;
  String get waEnableService => _strings.waEnableService;
  String get waPhoneNumberLabel => _strings.waPhoneNumberLabel;
  String get waReminderSettingsTitle => _strings.waReminderSettingsTitle;
  String get waEnableReminder => _strings.waEnableReminder;
  String get waReminderHoursLabel => _strings.waReminderHoursLabel;
  String get waSaveConfig => _strings.waSaveConfig;
  String get waConfigSaved => _strings.waConfigSaved;
  String get waAlreadyConnected => _strings.waAlreadyConnected;
  String get waWaitingLink => _strings.waWaitingLink;
  String get waQRFetchFailed => _strings.waQRFetchFailed;
  String get waUnexpectedError => _strings.waUnexpectedError;
  String get waDisconnectConfirmTitle => _strings.waDisconnectConfirmTitle;
  String get waDisconnectConfirmMessage => _strings.waDisconnectConfirmMessage;
  String get waDisconnectBtn => _strings.waDisconnectBtn;
  String get waDisconnected => _strings.waDisconnected;
  String get waTemplateSaved => _strings.waTemplateSaved;
  String get waWriteMessageFirst => _strings.waWriteMessageFirst;
  String get waBroadcastConfirmTitle => _strings.waBroadcastConfirmTitle;
  String get waBroadcastConfirmMessage => _strings.waBroadcastConfirmMessage;
  String get waSendNow => _strings.waSendNow;
  String get waBroadcastSent => _strings.waBroadcastSent;
  String get waTestTitle => _strings.waTestTitle;
  String get waEnterPhone => _strings.waEnterPhone;
  String get waTestMessage => _strings.waTestMessage;
  String get waTestSent => _strings.waTestSent;
  String get waSendTestBtn => _strings.waSendTestBtn;
  String get waDebtReminderConfirmTitle => _strings.waDebtReminderConfirmTitle;
  String get waDebtReminderConfirmMessage => _strings.waDebtReminderConfirmMessage;
  String get waSendBtn => _strings.waSendBtn;
  String get waDebtReminderSent => _strings.waDebtReminderSent;
  String get waTabSettings => _strings.waTabSettings;
  String get waTabTemplates => _strings.waTabTemplates;
  String get waTabBroadcast => _strings.waTabBroadcast;
  String get waRetry => _strings.waRetry;
  String get waStatusConnected => _strings.waStatusConnected;
  String get waStatusWaiting => _strings.waStatusWaiting;
  String get waStatusDisconnected => _strings.waStatusDisconnected;
  String get waEdit => _strings.waEdit;
  String get waAutoReminder => _strings.waAutoReminder;
  String get waReminderEnabledText => _strings.waReminderEnabledText;
  String get waReminderDisabled => _strings.waReminderDisabled;
  String get waQRLinkTitle => _strings.waQRLinkTitle;
  String get waQRLinkDescription => _strings.waQRLinkDescription;
  String get waQRFetching => _strings.waQRFetching;
  String get waQRFetchButton => _strings.waQRFetchButton;
  String get waQRError => _strings.waQRError;
  String get waQRInstructions => _strings.waQRInstructions;
  String get waDisconnectBtn2 => _strings.waDisconnectBtn2;
  String get waBroadcastTitle => _strings.waBroadcastTitle;
  String get waBroadcastDescription => _strings.waBroadcastDescription;
  String get waBroadcastHint => _strings.waBroadcastHint;
  String get waCharactersCount => _strings.waCharactersCount;
  String get waSending => _strings.waSending;
  String get waBroadcastSendBtn => _strings.waBroadcastSendBtn;
  String get waDebtReminderTitle => _strings.waDebtReminderTitle;
  String get waDebtReminderDescription => _strings.waDebtReminderDescription;
  String get waDebtReminderBtn => _strings.waDebtReminderBtn;
  String get waSendingDebtReminder => _strings.waSendingDebtReminder;

  // ── Vouchers ──
  String get vouchersTitle => _strings.vouchersTitle;
  String get vouchersSubtitle => _strings.vouchersSubtitle;
  String get generateVouchers => _strings.generateVouchers;
  String get generateVouchersTitle => _strings.generateVouchersTitle;
  String get voucherProfile => _strings.voucherProfile;
  String get voucherCount => _strings.voucherCount;
  String get voucherPrice => _strings.voucherPrice;
  String get voucherCodeType => _strings.voucherCodeType;
  String get voucherAlphaNumeric => _strings.voucherAlphaNumeric;
  String get voucherNumbersOnly => _strings.voucherNumbersOnly;
  String get voucherLettersOnly => _strings.voucherLettersOnly;
  String get voucherCodeLength => _strings.voucherCodeLength;
  String get generateBtn => _strings.generate;
  String get vouchersGenerated => _strings.vouchersGenerated;
  String get deleteVoucher => _strings.deleteVoucher;
  String get deleteVoucherConfirm => _strings.deleteVoucherConfirm;
  String get voucherDeleted => _strings.voucherDeleted;
  String get deleteBatchLabel => _strings.deleteBatch;
  String get deleteBatchConfirm => _strings.deleteBatchConfirm;
  String get batchDeleted => _strings.batchDeleted;
  String get clearAllVouchers => _strings.clearAllVouchers;
  String get clearAllVouchersConfirm => _strings.clearAllVouchersConfirm;
  String get clearAllLabel => _strings.clearAll;
  String get allVouchersCleared => _strings.allVouchersCleared;
  String get searchVouchers => _strings.searchVouchers;
  String get noVouchers => _strings.noVouchers;
  String get voucherProfileLabel => _strings.voucherProfileLabel;
  String get voucherDay => _strings.voucherDay;
  String get voucherBatchLabel => _strings.voucherBatchLabel;
  String get voucherUsed => _strings.voucherUsed;
  String get voucherAvailable => _strings.voucherAvailable;
  String get noBatchId => _strings.noBatchId;
  String get deleteThisVoucher => _strings.deleteThisVoucher;
  String get deleteEntireBatch => _strings.deleteEntireBatch;

  // ── Streams (Broadcast) ──
  String get streamsTitle => _strings.streamsTitle;
  String get streamsSubtitle => _strings.streamsSubtitle;
  String get streamsAddSource => _strings.streamsAddSource;
  String get streamsSearch => _strings.streamsSearch;
  String get streamsNoSources => _strings.streamsNoSources;
  String get streamsStatus => _strings.streamsStatus;
  String get streamsActive => _strings.streamsActive;
  String get streamsInactive => _strings.streamsInactive;
  String get streamsStop => _strings.streamsStop;
  String get streamsActivate => _strings.streamsActivate;
  String get streamsToggleSuccess => _strings.streamsToggleSuccess;

  // ── Logs ──
  String get logsTitle => _strings.logsTitle;
  String get logsSubtitle => _strings.logsSubtitle;
  String get logsAutoRefresh => _strings.logsAutoRefresh;
  String get logsSearch => _strings.logsSearch;
  String get logsNoLogs => _strings.logsNoLogs;
  String get logsClear => _strings.logsClear;
  String get logsClearTitle => _strings.logsClearTitle;
  String get logsClearConfirm => _strings.logsClearConfirm;
  String get logsClearSuccess => _strings.logsClearSuccess;
  String get logsFallback => _strings.logsFallback;

  // ── WhatsApp (new) ──
  String get waInvalidServerData => _strings.waInvalidServerData;
  String get waConfigLoadError => _strings.waConfigLoadError;
  String get waTemplatesLoadError => _strings.waTemplatesLoadError;
  String get waReminderActive => _strings.waReminderActive;

  // ── WhatsApp Template Labels ──
  String get waTemplateRenewPaid => _strings.waTemplateRenewPaid;
  String get waTemplateRenewDebt => _strings.waTemplateRenewDebt;
  String get waTemplateAddDebt => _strings.waTemplateAddDebt;
  String get waTemplatePayment => _strings.waTemplatePayment;
  String get waTemplateExpiryReminder => _strings.waTemplateExpiryReminder;
  String get waTemplateDebtReminder => _strings.waTemplateDebtReminder;

  // ── Advanced Settings ──
  String get licenseActive => _strings.licenseActive;
  String get licenseInactive => _strings.licenseInactive;
  String get licenseCurrentStatus => _strings.licenseCurrentStatus;
  String get advNotConnected => _strings.advNotConnected;
  String get advCloudflareUrlTitle => _strings.advCloudflareUrlTitle;
  String get advLicenseSectionTitle => _strings.advLicenseSectionTitle;
  String get advLicenseKeyHint => _strings.advLicenseKeyHint;
  String get advActivate => _strings.advActivate;
  String get advEnterLicenseKey => _strings.advEnterLicenseKey;
  String get advLicenseActivated => _strings.advLicenseActivated;
  String get advExpiryPrefix => _strings.advExpiryPrefix;
  String get advSerialPrefix => _strings.advSerialPrefix;
  String get advBlackoutTitle => _strings.advBlackoutTitle;
  String get advBlackoutEnableLabel => _strings.advBlackoutEnableLabel;
  String get advBlackoutStartLabel => _strings.advBlackoutStartLabel;
  String get advBlackoutEndLabel => _strings.advBlackoutEndLabel;
  String get advBlackoutDatesLabel => _strings.advBlackoutDatesLabel;
  String get advAddDate => _strings.advAddDate;
  String get advNoBlackoutDates => _strings.advNoBlackoutDates;
  String get advBlackoutExceptionsLabel => _strings.advBlackoutExceptionsLabel;
  String get advNoExceptions => _strings.advNoExceptions;
  String get advExceptionHint => _strings.advExceptionHint;
  String get advAdd => _strings.advAdd;
  String get advSaveBlackout => _strings.advSaveBlackout;
  String get advBlackoutSaved => _strings.advBlackoutSaved;
  String get advBackupTitle => _strings.advBackupTitle;
  String get advBackupDownload => _strings.advBackupDownload;
  String get advRestore => _strings.advRestore;
  String get advTelegramTitle => _strings.advTelegramTitle;
  String get advTelegramEnableLabel => _strings.advTelegramEnableLabel;
  String get advTelegramTokenLabel => _strings.advTelegramTokenLabel;
  String get advTelegramChatLabel => _strings.advTelegramChatLabel;
  String get advSaveSettings => _strings.advSaveSettings;
  String get advTestBackup => _strings.advTestBackup;
  String get advBackupEmpty => _strings.advBackupEmpty;
  String get advBackupSaved => _strings.advBackupSaved;
  String get advSaveCanceled => _strings.advSaveCanceled;
  String get advDownloadFailed => _strings.advDownloadFailed;
  String get advExportFailed => _strings.advExportFailed;
  String get advCannotReadFile => _strings.advCannotReadFile;
  String get advRestoreSuccess => _strings.advRestoreSuccess;
  String get advTelegramSaved => _strings.advTelegramSaved;
  String get advTelegramTestSent => _strings.advTelegramTestSent;
  String get advMigrationTitle => _strings.advMigrationTitle;
  String get advMigrateDialogTitle => _strings.advMigrateDialogTitle;
  String get advMigrateConfirm => _strings.advMigrateConfirm;
  String get advStartMigration => _strings.advStartMigration;
  String get advMigrationStarted => _strings.advMigrationStarted;
  String get advMigrateSas4 => _strings.advMigrateSas4;
  String get advImportExcel => _strings.advImportExcel;
  String get advExportExcel => _strings.advExportExcel;
  String get advCannotReadExcel => _strings.advCannotReadExcel;
  String get advExcelImported => _strings.advExcelImported;
  String get advExportEmpty => _strings.advExportEmpty;
  String get advExcelSaved => _strings.advExcelSaved;
  String get advSaveFileDialog => _strings.advSaveFileDialog;
  String get advSaveError => _strings.advSaveError;
  String get advDangerZoneTitle => _strings.advDangerZoneTitle;
  String get advResetTitle => _strings.advResetTitle;
  String get advResetDesc => _strings.advResetDesc;
  String get advResetButton => _strings.advResetButton;
  String get advResetDialogTitle => _strings.advResetDialogTitle;
  String get advResetWarning => _strings.advResetWarning;
  String get advResetConfirmHint => _strings.advResetConfirmHint;
  String get advResetKeyword => _strings.advResetKeyword;
  String get advResetKeywordHint => _strings.advResetKeywordHint;
  String get advResetNow => _strings.advResetNow;
  String get advResetSuccess => _strings.advResetSuccess;
  String get advTabLicense => _strings.advTabLicense;
  String get advTabBackup => _strings.advTabBackup;
  String get advTabMigration => _strings.advTabMigration;
}

class _AppLocalizationsDelegate
    extends LocalizationsDelegate<AppLocalizations> {
  const _AppLocalizationsDelegate();

  @override
  bool isSupported(Locale locale) {
    return supportedLanguageCodes.contains(locale.languageCode);
  }

  @override
  Future<AppLocalizations> load(Locale locale) async {
    return AppLocalizations(locale);
  }

  @override
  bool shouldReload(covariant LocalizationsDelegate<AppLocalizations> old) {
    return false;
  }
}