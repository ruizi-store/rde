// 共享服务层导出 - 核心服务
export * from "./api";
export * from "./auth";
export * from "./files";
export * from "./system";
export * from "./terminal";
export * from "./preferences";
export * from "./sudo";
export * from "./privilege";
export {
  notificationService,
  categoryInfo,
  severityInfo,
  channelTypeInfo,
  allCategories,
  allSeverities,
  allChannelTypes,
  type Notification as AppNotification,
  type NotificationSettings as AppNotificationSettings,
  type NotificationCategory,
  type NotificationSeverity,
  type NotificationChannel,
  type NotificationRule,
  type ChannelType,
} from "./notification";

