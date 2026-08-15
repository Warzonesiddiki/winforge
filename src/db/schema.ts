import {
  boolean,
  integer,
  jsonb,
  pgEnum,
  pgTable,
  serial,
  text,
  timestamp,
  uuid,
} from "drizzle-orm/pg-core";

export const riskLevelEnum = pgEnum("risk_level", [
  "low",
  "medium",
  "high",
  "expert",
]);

export const packageStatusEnum = pgEnum("package_status", [
  "installed",
  "removed",
  "protected",
]);

// ── Tweaks catalog + simulated live state ─────────────────────────────
export const tweaks = pgTable("tweaks", {
  id: text("id").primaryKey(),
  name: text("name").notNull(),
  description: text("description").notNull(),
  category: text("category").notNull(),
  risk: riskLevelEnum("risk").notNull().default("low"),
  defaultEnabled: boolean("default_enabled").notNull().default(false),
  applied: boolean("applied").notNull().default(false),
  tags: text("tags").array().notNull().default([]),
  warningMessage: text("warning_message"),
  breaksFeatures: text("breaks_features").array().notNull().default([]),
  operations: text("operations").array().notNull().default([]),
  undoOperations: text("undo_operations").array().notNull().default([]),
  updatedAt: timestamp("updated_at").notNull().defaultNow(),
});

// ── Debloat / Appx package catalog ─────────────────────────────────────
export const debloatPackages = pgTable("debloat_packages", {
  packageName: text("package_name").primaryKey(),
  displayName: text("display_name").notNull(),
  category: text("category").notNull(),
  risk: riskLevelEnum("risk").notNull().default("low"),
  canReinstall: boolean("can_reinstall").notNull().default(true),
  storeId: text("store_id"),
  breaksFeatures: text("breaks_features").array().notNull().default([]),
  status: packageStatusEnum("status").notNull().default("installed"),
  provisionedRemoved: boolean("provisioned_removed").notNull().default(false),
  updatedAt: timestamp("updated_at").notNull().defaultNow(),
});

// ── Privacy hardening rules ─────────────────────────────────────────────
export const privacyRules = pgTable("privacy_rules", {
  id: text("id").primaryKey(),
  name: text("name").notNull(),
  description: text("description").notNull(),
  category: text("category").notNull(),
  risk: riskLevelEnum("risk").notNull().default("low"),
  defaultEnabled: boolean("default_enabled").notNull().default(false),
  enabled: boolean("enabled").notNull().default(false),
  updatedAt: timestamp("updated_at").notNull().defaultNow(),
});

// ── Software installer catalog ─────────────────────────────────────────
export const applications = pgTable("applications", {
  id: text("id").primaryKey(),
  name: text("name").notNull(),
  publisher: text("publisher").notNull(),
  category: text("category").notNull(),
  version: text("version").notNull().default("latest"),
  source: text("source").notNull().default("winget"),
  installed: boolean("installed").notNull().default(false),
  updatedAt: timestamp("updated_at").notNull().defaultNow(),
});

// ── Startup items ────────────────────────────────────────────────────────
export const startupItems = pgTable("startup_items", {
  id: text("id").primaryKey(),
  name: text("name").notNull(),
  publisher: text("publisher").notNull(),
  command: text("command").notNull(),
  impact: text("impact").notNull().default("low"),
  enabled: boolean("enabled").notNull().default(true),
});

// ── Presets ──────────────────────────────────────────────────────────────
export const presets = pgTable("presets", {
  id: text("id").primaryKey(),
  name: text("name").notNull(),
  description: text("description").notNull(),
  tweakIds: text("tweak_ids").array().notNull().default([]),
  debloatPackages: text("debloat_packages").array().notNull().default([]),
  privacyRuleIds: text("privacy_rule_ids").array().notNull().default([]),
});

// ── Operation history / undo log ─────────────────────────────────────────
export const operationHistory = pgTable("operation_history", {
  id: uuid("id").defaultRandom().primaryKey(),
  timestamp: timestamp("timestamp").notNull().defaultNow(),
  operationType: text("operation_type").notNull(),
  category: text("category").notNull(),
  target: text("target").notNull(),
  previousValue: text("previous_value"),
  newValue: text("new_value"),
  risk: riskLevelEnum("risk").notNull().default("low"),
  success: boolean("success").notNull().default(true),
  errorMessage: text("error_message"),
  canUndo: boolean("can_undo").notNull().default(true),
  undone: boolean("undone").notNull().default(false),
  undoData: jsonb("undo_data").$type<{
    kind: "tweak" | "privacy" | "debloat" | "app" | "update" | "startup" | "service" | "task" | "context_menu" | "pack";
    id: string;
    field: string;
    value: boolean | string;
  } | null>(),
});

// ── Windows Update simulation ─────────────────────────────────────────────
export const windowsUpdates = pgTable("windows_updates", {
  id: text("id").primaryKey(),
  title: text("title").notNull(),
  kb: text("kb").notNull(),
  sizeMb: integer("size_mb").notNull().default(0),
  severity: text("severity").notNull().default("Optional"),
  releaseDate: text("release_date").notNull(),
  installed: boolean("installed").notNull().default(false),
  hidden: boolean("hidden").notNull().default(false),
});

// ── Restore points (simulated) ────────────────────────────────────────────
export const restorePoints = pgTable("restore_points", {
  id: serial("id").primaryKey(),
  sequenceNumber: integer("sequence_number").notNull(),
  description: text("description").notNull(),
  createdAt: timestamp("created_at").notNull().defaultNow(),
});

// ── ISO builder jobs ───────────────────────────────────────────────────────
export const isoJobs = pgTable("iso_jobs", {
  id: uuid("id").defaultRandom().primaryKey(),
  createdAt: timestamp("created_at").notNull().defaultNow(),
  status: text("status").notNull().default("pending"),
  options: jsonb("options").$type<Record<string, boolean>>().notNull(),
  log: text("log").array().notNull().default([]),
  sha256: text("sha256"),
});

// ── Windows Services catalog (services.json equivalent) ────────────────────
export const services = pgTable("services", {
  id: text("id").primaryKey(), // service name, e.g. "DiagTrack"
  displayName: text("display_name").notNull(),
  description: text("description").notNull(),
  category: text("category").notNull(),
  startType: text("start_type").notNull().default("Automatic"),
  status: text("status").notNull().default("Running"),
  risk: riskLevelEnum("risk").notNull().default("low"),
  protected: boolean("protected").notNull().default(false),
  recommended: text("recommended").notNull().default("keep"), // keep | disable | manual
  updatedAt: timestamp("updated_at").notNull().defaultNow(),
});

// ── Scheduled Tasks catalog (tasks.json equivalent) ────────────────────────
export const scheduledTasks = pgTable("scheduled_tasks", {
  id: text("id").primaryKey(),
  name: text("name").notNull(),
  path: text("path").notNull(),
  description: text("description").notNull(),
  enabled: boolean("enabled").notNull().default(true),
  risk: riskLevelEnum("risk").notNull().default("low"),
  category: text("category").notNull(),
  updatedAt: timestamp("updated_at").notNull().defaultNow(),
});

// ── Context Menu items catalog (Explorer right-click clutter removal) ──────
export const contextMenuItems = pgTable("context_menu_items", {
  id: text("id").primaryKey(),
  title: text("title").notNull(),
  description: text("description").notNull(),
  registryKey: text("registry_key").notNull(),
  targetExtension: text("target_extension").notNull().default("*"), // *, Directory, Drive, Background
  enabled: boolean("enabled").notNull().default(true),
  risk: riskLevelEnum("risk").notNull().default("low"),
  category: text("category").notNull().default("Windows Default"),
  updatedAt: timestamp("updated_at").notNull().defaultNow(),
});

// ── Community Packs / Plugins catalog ───────────────────────────────────────
export const communityPacks = pgTable("community_packs", {
  id: text("id").primaryKey(),
  name: text("name").notNull(),
  author: text("author").notNull(),
  description: text("description").notNull(),
  version: text("version").notNull().default("1.0.0"),
  category: text("category").notNull(),
  icon: text("icon").notNull().default("⚡"),
  tweakIds: text("tweak_ids").array().notNull().default([]),
  debloatPackages: text("debloat_packages").array().notNull().default([]),
  privacyRuleIds: text("privacy_rule_ids").array().notNull().default([]),
  installed: boolean("installed").notNull().default(false),
  updatedAt: timestamp("updated_at").notNull().defaultNow(),
});

// ── Health score history (persistent trend tracking) ───────────────────────
export const healthHistory = pgTable("health_history", {
  id: serial("id").primaryKey(),
  timestamp: timestamp("timestamp").notNull().defaultNow(),
  score: integer("score").notNull(),
  privacyScore: integer("privacy_score").notNull(),
  bloatCount: integer("bloat_count").notNull(),
  appliedTweaks: integer("applied_tweaks").notNull(),
  pendingUpdates: integer("pending_updates").notNull(),
});

// ── System snapshots (baseline capture + compare + restore) ────────────────
export interface SnapshotState {
  tweaks: Record<string, boolean>; // tweak id -> applied
  packages: Record<string, "installed" | "removed">; // package name -> status
  privacy: Record<string, boolean>; // rule id -> enabled
}

export const snapshots = pgTable("snapshots", {
  id: uuid("id").defaultRandom().primaryKey(),
  name: text("name").notNull(),
  createdAt: timestamp("created_at").notNull().defaultNow(),
  state: jsonb("state").$type<SnapshotState>().notNull(),
});

// ── App settings (singleton row) ──────────────────────────────────────────
export const appSettings = pgTable("app_settings", {
  id: integer("id").primaryKey().default(1),
  theme: text("theme").notNull().default("dark"),
  backdrop: text("backdrop").notNull().default("mica"),
  language: text("language").notNull().default("en-US"),
  restorePointBeforeMutation: boolean("restore_point_before_mutation")
    .notNull()
    .default(true),
  showExpertTweaks: boolean("show_expert_tweaks").notNull().default(false),
  showCopilotTweaksSeparately: boolean("show_copilot_tweaks_separately")
    .notNull()
    .default(true),
  autoMaintenanceEnabled: boolean("auto_maintenance_enabled")
    .notNull()
    .default(false),
});
