/// Data model for all translatable strings.
///
/// This file contains NO translations. It only defines the shape of
/// [AppStrings] and a registry used by [AppLocalizations].
/// Translations live in `strings/app_strings_<lang>.dart`.
class AppStrings {
  final String appName;
  final String appSubtitle;
  final String splashTitle;
  final String splashSubtitle;
  final String loginTitle;
  final String loginButton;
  final String changeServer;
  final String username;
  final String password;
  final String loginSuccess;
  final String loginFailed;
  final String serverSetupTitle;
  final String serverAddress;
  final String serverAddressHint;
  final String cloudflareLink;
  final String cloudflareHint;
  final String cloudflareInfo;
  final String testConnection;
  final String saveAndContinue;
  final String serverSaveSuccess;
  final String connectionSuccess;
  final String serverInfo;
  final String navDashboard;
  final String navUsers;
  final String navAgents;
  final String navProfiles;
  final String navVouchers;
  final String navNas;
  final String navStreams;
  final String navWhatsApp;
  final String navLogs;
  final String navAdvancedSettings;
  final String navSettings;
  final String refresh;
  final String logout;
  final String logoutSuccess;
  final String superAdminBadge;
  final String balanceCurrency;
  final String dashboardTitle;
  final String dashboardSubtitle;
  final String totalUsers;
  final String activeUsers;
  final String onlineUsers;
  final String expiredUsers;
  final String aboutToExpire;
  final String balance;
  final String dashboardLoadError;
  final String retry;
  final String usersTitle;
  final String usersSubtitle;
  final String addUser;
  final String searchUsers;
  final String filterAll;
  final String filterActive;
  final String filterExpired;
  final String filterOnline;
  final String filterAboutToExpire;
  final String noResults;
  final String addUserTitle;
  final String editUserTitle;
  final String fullName;
  final String phone;
  final String profile;
  final String expiryDate;
  final String expiryDateHint;
  final String cancel;
  final String save;
  final String createUser;
  final String saveChanges;
  final String userCreated;
  final String userUpdated;
  final String disconnectUser;
  final String disconnectSuccess;
  final String toggleStatus;
  final String toggleSuccess;
  final String renewSubscription;
  final String newProfile;
  final String paidAmount;
  final String paidAmountInfo;
  final String renew;
  final String renewSuccess;
  final String deleteUserTitle;
  final String deleteUserConfirm;
  final String delete;
  final String deleteSuccess;
  final String addDebt;
  final String payDebt;
  final String amount;
  final String amountHint;
  final String notes;
  final String notesHint;
  final String enterValidAmount;
  final String debtAdded;
  final String debtPaid;
  final String debtLabel;
  final String txAddDebtTitle;
  final String txPayDebtTitle;
  final String txSubscriber;
  final String txAddDebtBtn;
  final String txPayDebtBtn;
  final String cardName;
  final String cardProfile;
  final String cardExpiry;
  final String cardBalance;
  final String editBtn;
  final String details;
  final String disconnectBtn;
  final String detailsTitle;
  final String tabInfo;
  final String tabFinancial;
  final String tabSessions;
  final String basicInfo;
  final String infoUser;
  final String infoCreatedAt;
  final String infoStatus;
  final String statusOnline;
  final String statusStale;
  final String statusOffline;
  final String statusExpired;
  final String statusExpiredOnline;
  final String currentSessionInfo;
  final String sessIp;
  final String sessDownload;
  final String sessUpload;
  final String sessDuration;
  final String sessMac;
  final String sessNasIp;
  final String noActiveSession;
  final String currentBalance;
  final String financialRecords;
  final String txCount;
  final String noFinancialRecords;
  final String txDebtType;
  final String txPayType;
  final String sessionHistory;
  final String sessionCount;
  final String noSessionHistory;
  final String startedAt;
  final String sessionActive;
  final String endedAt;
  final String dayUnit;
  final String durSec;
  final String durMin;
  final String durHour;
  final String invalidData;
  final String subscriberDetailsError;
  final String licenseActivation;
  final String systemNotActivated;
  final String activateLicenseInfo;
  final String routerConnected;
  final String routerNotConnected;
  final String contactDeveloper;
  final String developerName;
  final String whatsapp;
  final String telegram;
  final String paymentMethods;
  final String connectRouter;
  final String routerIp;
  final String routerIpHint;
  final String connectAndFetchSerial;
  final String connectRouterSuccess;
  final String activationKey;
  final String activationKeyHint;
  final String activateLicense;
  final String activating;
  final String activateSuccess;
  final String enterLicenseKey;
  final String enterRouterAddress;
  final String copiedLabel;
  final String failedOpenUrl;
  final String loading;
  final String error;
  final String success;
  final String currency;
  final String agentsTitle;
  final String agentsSubtitle;
  final String addAgent;
  final String searchAgents;
  final String noAgents;
  final String addNewAgent;
  final String role;
  final String roleAgent;
  final String roleSubAdmin;
  final String roleSuperAdmin;
  final String extraPermissions;
  final String manageProfiles;
  final String manageProfilesDesc;
  final String manageNas;
  final String manageNasDesc;
  final String email;
  final String register;
  final String agentRegistered;
  final String deleteAgent;
  final String deleteAgentConfirm;
  final String agentDeleted;
  final String rechargeBalance;
  final String withdrawBalance;
  final String charge;
  final String withdraw;
  final String balanceCharged;
  final String balanceWithdrawn;
  final String enterValidAmount2;
  final String financialTransactions;
  final String transactionsHistory;
  final String depositWithdrawSales;
  final String noTransactions;
  final String balanceLabel;
  final String date;
  final String fillAllFields;
  final String editNas;
  final String addNas;
  final String clientIp;
  final String nasName;
  final String radiusSecret;
  final String nasIp;
  final String visibleToAllAgents;
  final String nasAdded;
  final String nasUpdated;
  final String saveChanges2;
  final String addRouter;
  final String deleteRouter;
  final String deleteRouterConfirm;
  final String nasDeleted;
  final String quickSetup;
  final String quickSetupConfirm;
  final String quickSetupSuccess;
  final String autoConnect;
  final String autoConnectConfirm;
  final String autoConnectSuccess;
  final String addBypassDevice;
  final String deviceIp;
  final String deviceName;
  final String bypassAdded;
  final String deleteBypass;
  final String deleteBypassConfirm;
  final String bypassDeleted;
  final String tabNasDevices;
  final String tabGlobalBypass;
  final String tabBypassList;
  final String nasSubtitle;
  final String searchNas;
  final String noNas;
  final String addNasDevice;
  final String quickSetupBtn;
  final String connectRouterBtn;
  final String bypassDevicesTitle;
  final String bypassDevicesSubtitle;
  final String searchBypassList;
  final String noBypassDevices;
  final String globalBypassEnabled;
  final String globalBypassDisabled;
  final String globalBypassDescEnabled;
  final String globalBypassDescDisabled;
  final String enableBypass;
  final String disableBypass;
  final String enableUser;
  final String disableUser;
  final String bypassEnableSuccess;
  final String bypassDisableSuccess;
  final String bypassWarningTitle;
  final String bypassWarningText;
  final String disableGlobalBypass;
  final String enableGlobalBypass;
  final String bypassWarningImportant;
  final String addDevice;
  final String nasNameLabel;
  final String nasIpLabel;
  final String bypassDisableConfirm;
  final String profilesTitle;
  final String profilesSubtitle;
  final String addProfile;
  final String editProfile;
  final String addNewProfile;
  final String profileName;
  final String downloadSpeed;
  final String uploadSpeed;
  final String mikrotikLinkType;
  final String noLink;
  final String poolName;
  final String groupName;
  final String validityDays;
  final String userPrice;
  final String agentPrice;
  final String simultaneousDevices;
  final String expiredTransfer;
  final String noTransfer;
  final String expiredPoolLabel;
  final String expiredPoolName;
  final String expiredProfileLabel;
  final String profileNameRequired;
  final String profileCreated;
  final String createProfile;
  final String deleteProfile;
  final String deleteProfileConfirm;
  final String profileDeleted;
  final String noProfiles;
  final String searchProfiles;
  final String speedLabel;
  final String validityLabel;
  final String usersLabel;
  final String waTitle;
  final String waSubtitle;
  final String waConfigTitle;
  final String waEnableService;
  final String waPhoneNumberLabel;
  final String waReminderSettingsTitle;
  final String waEnableReminder;
  final String waReminderHoursLabel;
  final String waSaveConfig;
  final String waConfigSaved;
  final String waAlreadyConnected;
  final String waWaitingLink;
  final String waQRFetchFailed;
  final String waUnexpectedError;
  final String waDisconnectConfirmTitle;
  final String waDisconnectConfirmMessage;
  final String waDisconnectBtn;
  final String waDisconnected;
  final String waTemplateSaved;
  final String waWriteMessageFirst;
  final String waBroadcastConfirmTitle;
  final String waBroadcastConfirmMessage;
  final String waSendNow;
  final String waBroadcastSent;
  final String waTestTitle;
  final String waEnterPhone;
  final String waTestMessage;
  final String waTestSent;
  final String waSendTestBtn;
  final String waDebtReminderConfirmTitle;
  final String waDebtReminderConfirmMessage;
  final String waSendBtn;
  final String waDebtReminderSent;
  final String waTabSettings;
  final String waTabTemplates;
  final String waTabBroadcast;
  final String waRetry;
  final String waStatusConnected;
  final String waStatusWaiting;
  final String waStatusDisconnected;
  final String waEdit;
  final String waAutoReminder;
  final String waReminderEnabledText;
  final String waReminderDisabled;
  final String waQRLinkTitle;
  final String waQRLinkDescription;
  final String waQRFetching;
  final String waQRFetchButton;
  final String waQRError;
  final String waQRInstructions;
  final String waDisconnectBtn2;
  final String waBroadcastTitle;
  final String waBroadcastDescription;
  final String waBroadcastHint;
  final String waCharactersCount;
  final String waSending;
  final String waBroadcastSendBtn;
  final String waDebtReminderTitle;
  final String waDebtReminderDescription;
  final String waDebtReminderBtn;
  final String waSendingDebtReminder;
  final String waInvalidServerData;
  final String waConfigLoadError;
  final String waTemplatesLoadError;
  final String waReminderActive;
  final String waTemplateRenewPaid;
  final String waTemplateRenewDebt;
  final String waTemplateAddDebt;
  final String waTemplatePayment;
  final String waTemplateExpiryReminder;
  final String waTemplateDebtReminder;
  final String vouchersTitle;
  final String vouchersSubtitle;
  final String generateVouchers;
  final String generateVouchersTitle;
  final String voucherProfile;
  final String voucherCount;
  final String voucherPrice;
  final String voucherCodeType;
  final String voucherAlphaNumeric;
  final String voucherNumbersOnly;
  final String voucherLettersOnly;
  final String voucherCodeLength;
  final String generate;
  final String vouchersGenerated;
  final String deleteVoucher;
  final String deleteVoucherConfirm;
  final String voucherDeleted;
  final String deleteBatch;
  final String deleteBatchConfirm;
  final String batchDeleted;
  final String clearAllVouchers;
  final String clearAllVouchersConfirm;
  final String clearAll;
  final String allVouchersCleared;
  final String searchVouchers;
  final String noVouchers;
  final String voucherProfileLabel;
  final String voucherDay;
  final String voucherBatchLabel;
  final String voucherUsed;
  final String voucherAvailable;
  final String noBatchId;
  final String deleteThisVoucher;
  final String deleteEntireBatch;
  final String streamsTitle;
  final String streamsSubtitle;
  final String streamsAddSource;
  final String streamsSearch;
  final String streamsNoSources;
  final String streamsStatus;
  final String streamsActive;
  final String streamsInactive;
  final String streamsStop;
  final String streamsActivate;
  final String streamsToggleSuccess;
  final String logsTitle;
  final String logsSubtitle;
  final String logsAutoRefresh;
  final String logsSearch;
  final String logsNoLogs;
  final String logsClear;
  final String logsClearTitle;
  final String logsClearConfirm;
  final String logsClearSuccess;
  final String logsFallback;
  final String licenseActive;
  final String licenseInactive;
  final String licenseCurrentStatus;
  final String advNotConnected;
  final String advCloudflareUrlTitle;
  final String advLicenseSectionTitle;
  final String advLicenseKeyHint;
  final String advActivate;
  final String advEnterLicenseKey;
  final String advLicenseActivated;
  final String advExpiryPrefix;
  final String advSerialPrefix;
  final String advBlackoutTitle;
  final String advBlackoutEnableLabel;
  final String advBlackoutStartLabel;
  final String advBlackoutEndLabel;
  final String advBlackoutDatesLabel;
  final String advAddDate;
  final String advNoBlackoutDates;
  final String advBlackoutExceptionsLabel;
  final String advNoExceptions;
  final String advExceptionHint;
  final String advAdd;
  final String advSaveBlackout;
  final String advBlackoutSaved;
  final String advBackupTitle;
  final String advBackupDownload;
  final String advRestore;
  final String advTelegramTitle;
  final String advTelegramEnableLabel;
  final String advTelegramTokenLabel;
  final String advTelegramChatLabel;
  final String advSaveSettings;
  final String advTestBackup;
  final String advBackupEmpty;
  final String advBackupSaved;
  final String advSaveCanceled;
  final String advDownloadFailed;
  final String advExportFailed;
  final String advCannotReadFile;
  final String advRestoreSuccess;
  final String advTelegramSaved;
  final String advTelegramTestSent;
  final String advMigrationTitle;
  final String advMigrateDialogTitle;
  final String advMigrateConfirm;
  final String advStartMigration;
  final String advMigrationStarted;
  final String advMigrateSas4;
  final String advImportExcel;
  final String advExportExcel;
  final String advCannotReadExcel;
  final String advExcelImported;
  final String advExportEmpty;
  final String advExcelSaved;
  final String advSaveFileDialog;
  final String advSaveError;
  final String advDangerZoneTitle;
  final String advResetTitle;
  final String advResetDesc;
  final String advResetButton;
  final String advResetDialogTitle;
  final String advResetWarning;
  final String advResetConfirmHint;
  final String advResetKeyword;
  final String advResetKeywordHint;
  final String advResetNow;
  final String advResetSuccess;
  final String advTabLicense;
  final String advTabBackup;
  final String advTabMigration;

  const AppStrings({
    required this.appName,
    required this.appSubtitle,
    required this.splashTitle,
    required this.splashSubtitle,
    required this.loginTitle,
    required this.loginButton,
    required this.changeServer,
    required this.username,
    required this.password,
    required this.loginSuccess,
    required this.loginFailed,
    required this.serverSetupTitle,
    required this.serverAddress,
    required this.serverAddressHint,
    required this.cloudflareLink,
    required this.cloudflareHint,
    required this.cloudflareInfo,
    required this.testConnection,
    required this.saveAndContinue,
    required this.serverSaveSuccess,
    required this.connectionSuccess,
    required this.serverInfo,
    required this.navDashboard,
    required this.navUsers,
    required this.navAgents,
    required this.navProfiles,
    required this.navVouchers,
    required this.navNas,
    required this.navStreams,
    required this.navWhatsApp,
    required this.navLogs,
    required this.navAdvancedSettings,
    required this.navSettings,
    required this.refresh,
    required this.logout,
    required this.logoutSuccess,
    required this.superAdminBadge,
    required this.balanceCurrency,
    required this.dashboardTitle,
    required this.dashboardSubtitle,
    required this.totalUsers,
    required this.activeUsers,
    required this.onlineUsers,
    required this.expiredUsers,
    required this.aboutToExpire,
    required this.balance,
    required this.dashboardLoadError,
    required this.retry,
    required this.usersTitle,
    required this.usersSubtitle,
    required this.addUser,
    required this.searchUsers,
    required this.filterAll,
    required this.filterActive,
    required this.filterExpired,
    required this.filterOnline,
    required this.filterAboutToExpire,
    required this.noResults,
    required this.addUserTitle,
    required this.editUserTitle,
    required this.fullName,
    required this.phone,
    required this.profile,
    required this.expiryDate,
    required this.expiryDateHint,
    required this.cancel,
    required this.save,
    required this.createUser,
    required this.saveChanges,
    required this.userCreated,
    required this.userUpdated,
    required this.disconnectUser,
    required this.disconnectSuccess,
    required this.toggleStatus,
    required this.toggleSuccess,
    required this.renewSubscription,
    required this.newProfile,
    required this.paidAmount,
    required this.paidAmountInfo,
    required this.renew,
    required this.renewSuccess,
    required this.deleteUserTitle,
    required this.deleteUserConfirm,
    required this.delete,
    required this.deleteSuccess,
    required this.addDebt,
    required this.payDebt,
    required this.amount,
    required this.amountHint,
    required this.notes,
    required this.notesHint,
    required this.enterValidAmount,
    required this.debtAdded,
    required this.debtPaid,
    required this.debtLabel,
    required this.txAddDebtTitle,
    required this.txPayDebtTitle,
    required this.txSubscriber,
    required this.txAddDebtBtn,
    required this.txPayDebtBtn,
    required this.cardName,
    required this.cardProfile,
    required this.cardExpiry,
    required this.cardBalance,
    required this.editBtn,
    required this.details,
    required this.disconnectBtn,
    required this.detailsTitle,
    required this.tabInfo,
    required this.tabFinancial,
    required this.tabSessions,
    required this.basicInfo,
    required this.infoUser,
    required this.infoCreatedAt,
    required this.infoStatus,
    required this.statusOnline,
    required this.statusStale,
    required this.statusOffline,
    required this.statusExpired,
    required this.statusExpiredOnline,
    required this.currentSessionInfo,
    required this.sessIp,
    required this.sessDownload,
    required this.sessUpload,
    required this.sessDuration,
    required this.sessMac,
    required this.sessNasIp,
    required this.noActiveSession,
    required this.currentBalance,
    required this.financialRecords,
    required this.txCount,
    required this.noFinancialRecords,
    required this.txDebtType,
    required this.txPayType,
    required this.sessionHistory,
    required this.sessionCount,
    required this.noSessionHistory,
    required this.startedAt,
    required this.sessionActive,
    required this.endedAt,
    required this.dayUnit,
    required this.durSec,
    required this.durMin,
    required this.durHour,
    required this.invalidData,
    required this.subscriberDetailsError,
    required this.licenseActivation,
    required this.systemNotActivated,
    required this.activateLicenseInfo,
    required this.routerConnected,
    required this.routerNotConnected,
    required this.contactDeveloper,
    required this.developerName,
    required this.whatsapp,
    required this.telegram,
    required this.paymentMethods,
    required this.connectRouter,
    required this.routerIp,
    required this.routerIpHint,
    required this.connectAndFetchSerial,
    required this.connectRouterSuccess,
    required this.activationKey,
    required this.activationKeyHint,
    required this.activateLicense,
    required this.activating,
    required this.activateSuccess,
    required this.enterLicenseKey,
    required this.enterRouterAddress,
    required this.copiedLabel,
    required this.failedOpenUrl,
    required this.loading,
    required this.error,
    required this.success,
    required this.currency,
    required this.agentsTitle,
    required this.agentsSubtitle,
    required this.addAgent,
    required this.searchAgents,
    required this.noAgents,
    required this.addNewAgent,
    required this.role,
    required this.roleAgent,
    required this.roleSubAdmin,
    required this.roleSuperAdmin,
    required this.extraPermissions,
    required this.manageProfiles,
    required this.manageProfilesDesc,
    required this.manageNas,
    required this.manageNasDesc,
    required this.email,
    required this.register,
    required this.agentRegistered,
    required this.deleteAgent,
    required this.deleteAgentConfirm,
    required this.agentDeleted,
    required this.rechargeBalance,
    required this.withdrawBalance,
    required this.charge,
    required this.withdraw,
    required this.balanceCharged,
    required this.balanceWithdrawn,
    required this.enterValidAmount2,
    required this.financialTransactions,
    required this.transactionsHistory,
    required this.depositWithdrawSales,
    required this.noTransactions,
    required this.balanceLabel,
    required this.date,
    required this.fillAllFields,
    required this.editNas,
    required this.addNas,
    required this.clientIp,
    required this.nasName,
    required this.radiusSecret,
    required this.nasIp,
    required this.visibleToAllAgents,
    required this.nasAdded,
    required this.nasUpdated,
    required this.saveChanges2,
    required this.addRouter,
    required this.deleteRouter,
    required this.deleteRouterConfirm,
    required this.nasDeleted,
    required this.quickSetup,
    required this.quickSetupConfirm,
    required this.quickSetupSuccess,
    required this.autoConnect,
    required this.autoConnectConfirm,
    required this.autoConnectSuccess,
    required this.addBypassDevice,
    required this.deviceIp,
    required this.deviceName,
    required this.bypassAdded,
    required this.deleteBypass,
    required this.deleteBypassConfirm,
    required this.bypassDeleted,
    required this.tabNasDevices,
    required this.tabGlobalBypass,
    required this.tabBypassList,
    required this.nasSubtitle,
    required this.searchNas,
    required this.noNas,
    required this.addNasDevice,
    required this.quickSetupBtn,
    required this.connectRouterBtn,
    required this.bypassDevicesTitle,
    required this.bypassDevicesSubtitle,
    required this.searchBypassList,
    required this.noBypassDevices,
    required this.globalBypassEnabled,
    required this.globalBypassDisabled,
    required this.globalBypassDescEnabled,
    required this.globalBypassDescDisabled,
    required this.enableBypass,
    required this.disableBypass,
    required this.enableUser,
    required this.disableUser,
    required this.bypassEnableSuccess,
    required this.bypassDisableSuccess,
    required this.bypassWarningTitle,
    required this.bypassWarningText,
    required this.disableGlobalBypass,
    required this.enableGlobalBypass,
    required this.bypassWarningImportant,
    required this.addDevice,
    required this.nasNameLabel,
    required this.nasIpLabel,
    required this.bypassDisableConfirm,
    required this.profilesTitle,
    required this.profilesSubtitle,
    required this.addProfile,
    required this.editProfile,
    required this.addNewProfile,
    required this.profileName,
    required this.downloadSpeed,
    required this.uploadSpeed,
    required this.mikrotikLinkType,
    required this.noLink,
    required this.poolName,
    required this.groupName,
    required this.validityDays,
    required this.userPrice,
    required this.agentPrice,
    required this.simultaneousDevices,
    required this.expiredTransfer,
    required this.noTransfer,
    required this.expiredPoolLabel,
    required this.expiredPoolName,
    required this.expiredProfileLabel,
    required this.profileNameRequired,
    required this.profileCreated,
    required this.createProfile,
    required this.deleteProfile,
    required this.deleteProfileConfirm,
    required this.profileDeleted,
    required this.noProfiles,
    required this.searchProfiles,
    required this.speedLabel,
    required this.validityLabel,
    required this.usersLabel,
    required this.waTitle,
    required this.waSubtitle,
    required this.waConfigTitle,
    required this.waEnableService,
    required this.waPhoneNumberLabel,
    required this.waReminderSettingsTitle,
    required this.waEnableReminder,
    required this.waReminderHoursLabel,
    required this.waSaveConfig,
    required this.waConfigSaved,
    required this.waAlreadyConnected,
    required this.waWaitingLink,
    required this.waQRFetchFailed,
    required this.waUnexpectedError,
    required this.waDisconnectConfirmTitle,
    required this.waDisconnectConfirmMessage,
    required this.waDisconnectBtn,
    required this.waDisconnected,
    required this.waTemplateSaved,
    required this.waWriteMessageFirst,
    required this.waBroadcastConfirmTitle,
    required this.waBroadcastConfirmMessage,
    required this.waSendNow,
    required this.waBroadcastSent,
    required this.waTestTitle,
    required this.waEnterPhone,
    required this.waTestMessage,
    required this.waTestSent,
    required this.waSendTestBtn,
    required this.waDebtReminderConfirmTitle,
    required this.waDebtReminderConfirmMessage,
    required this.waSendBtn,
    required this.waDebtReminderSent,
    required this.waTabSettings,
    required this.waTabTemplates,
    required this.waTabBroadcast,
    required this.waRetry,
    required this.waStatusConnected,
    required this.waStatusWaiting,
    required this.waStatusDisconnected,
    required this.waEdit,
    required this.waAutoReminder,
    required this.waReminderEnabledText,
    required this.waReminderDisabled,
    required this.waQRLinkTitle,
    required this.waQRLinkDescription,
    required this.waQRFetching,
    required this.waQRFetchButton,
    required this.waQRError,
    required this.waQRInstructions,
    required this.waDisconnectBtn2,
    required this.waBroadcastTitle,
    required this.waBroadcastDescription,
    required this.waBroadcastHint,
    required this.waCharactersCount,
    required this.waSending,
    required this.waBroadcastSendBtn,
    required this.waDebtReminderTitle,
    required this.waDebtReminderDescription,
    required this.waDebtReminderBtn,
    required this.waSendingDebtReminder,
    required this.waInvalidServerData,
    required this.waConfigLoadError,
    required this.waTemplatesLoadError,
    required this.waReminderActive,
    required this.waTemplateRenewPaid,
    required this.waTemplateRenewDebt,
    required this.waTemplateAddDebt,
    required this.waTemplatePayment,
    required this.waTemplateExpiryReminder,
    required this.waTemplateDebtReminder,
    required this.vouchersTitle,
    required this.vouchersSubtitle,
    required this.generateVouchers,
    required this.generateVouchersTitle,
    required this.voucherProfile,
    required this.voucherCount,
    required this.voucherPrice,
    required this.voucherCodeType,
    required this.voucherAlphaNumeric,
    required this.voucherNumbersOnly,
    required this.voucherLettersOnly,
    required this.voucherCodeLength,
    required this.generate,
    required this.vouchersGenerated,
    required this.deleteVoucher,
    required this.deleteVoucherConfirm,
    required this.voucherDeleted,
    required this.deleteBatch,
    required this.deleteBatchConfirm,
    required this.batchDeleted,
    required this.clearAllVouchers,
    required this.clearAllVouchersConfirm,
    required this.clearAll,
    required this.allVouchersCleared,
    required this.searchVouchers,
    required this.noVouchers,
    required this.voucherProfileLabel,
    required this.voucherDay,
    required this.voucherBatchLabel,
    required this.voucherUsed,
    required this.voucherAvailable,
    required this.noBatchId,
    required this.deleteThisVoucher,
    required this.deleteEntireBatch,
    required this.streamsTitle,
    required this.streamsSubtitle,
    required this.streamsAddSource,
    required this.streamsSearch,
    required this.streamsNoSources,
    required this.streamsStatus,
    required this.streamsActive,
    required this.streamsInactive,
    required this.streamsStop,
    required this.streamsActivate,
    required this.streamsToggleSuccess,
    required this.logsTitle,
    required this.logsSubtitle,
    required this.logsAutoRefresh,
    required this.logsSearch,
    required this.logsNoLogs,
    required this.logsClear,
    required this.logsClearTitle,
    required this.logsClearConfirm,
    required this.logsClearSuccess,
    required this.logsFallback,
    required this.licenseActive,
    required this.licenseInactive,
    required this.licenseCurrentStatus,
    required this.advNotConnected,
    required this.advCloudflareUrlTitle,
    required this.advLicenseSectionTitle,
    required this.advLicenseKeyHint,
    required this.advActivate,
    required this.advEnterLicenseKey,
    required this.advLicenseActivated,
    required this.advExpiryPrefix,
    required this.advSerialPrefix,
    required this.advBlackoutTitle,
    required this.advBlackoutEnableLabel,
    required this.advBlackoutStartLabel,
    required this.advBlackoutEndLabel,
    required this.advBlackoutDatesLabel,
    required this.advAddDate,
    required this.advNoBlackoutDates,
    required this.advBlackoutExceptionsLabel,
    required this.advNoExceptions,
    required this.advExceptionHint,
    required this.advAdd,
    required this.advSaveBlackout,
    required this.advBlackoutSaved,
    required this.advBackupTitle,
    required this.advBackupDownload,
    required this.advRestore,
    required this.advTelegramTitle,
    required this.advTelegramEnableLabel,
    required this.advTelegramTokenLabel,
    required this.advTelegramChatLabel,
    required this.advSaveSettings,
    required this.advTestBackup,
    required this.advBackupEmpty,
    required this.advBackupSaved,
    required this.advSaveCanceled,
    required this.advDownloadFailed,
    required this.advExportFailed,
    required this.advCannotReadFile,
    required this.advRestoreSuccess,
    required this.advTelegramSaved,
    required this.advTelegramTestSent,
    required this.advMigrationTitle,
    required this.advMigrateDialogTitle,
    required this.advMigrateConfirm,
    required this.advStartMigration,
    required this.advMigrationStarted,
    required this.advMigrateSas4,
    required this.advImportExcel,
    required this.advExportExcel,
    required this.advCannotReadExcel,
    required this.advExcelImported,
    required this.advExportEmpty,
    required this.advExcelSaved,
    required this.advSaveFileDialog,
    required this.advSaveError,
    required this.advDangerZoneTitle,
    required this.advResetTitle,
    required this.advResetDesc,
    required this.advResetButton,
    required this.advResetDialogTitle,
    required this.advResetWarning,
    required this.advResetConfirmHint,
    required this.advResetKeyword,
    required this.advResetKeywordHint,
    required this.advResetNow,
    required this.advResetSuccess,
    required this.advTabLicense,
    required this.advTabBackup,
    required this.advTabMigration,
  });

  static final Map<String, AppStrings> _registry = {};

  /// Registers a per-language [AppStrings] instance for [code].
  ///
  /// Named `registerLocale` (not `register`) to avoid colliding with the
  /// instance field [register], which holds the "Register" button label.
  static void registerLocale(String code, AppStrings strings) {
    _registry[code] = strings;
  }


  /// Returns the [AppStrings] for [code], falling back to Arabic.
  static AppStrings fromLocale(String code) {
    return _registry[code] ?? _registry['ar']!;
  }

  /// All registered language codes.
  static List<String> get supportedCodes => _registry.keys.toList();
}
