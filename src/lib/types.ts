export type RiskLevel = "low" | "medium" | "high" | "expert";

export const RISK_ORDER: Record<RiskLevel, number> = {
  low: 0,
  medium: 1,
  high: 2,
  expert: 3,
};

export const RISK_LABEL: Record<RiskLevel, string> = {
  low: "Low",
  medium: "Medium",
  high: "High",
  expert: "Expert",
};

export const RISK_CLASSES: Record<RiskLevel, string> = {
  low: "bg-emerald-500/15 text-emerald-400 border-emerald-500/30",
  medium: "bg-amber-500/15 text-amber-400 border-amber-500/30",
  high: "bg-red-500/15 text-red-400 border-red-500/30",
  expert: "bg-purple-500/15 text-purple-400 border-purple-500/30",
};

export interface HealthColor {
  label: string;
  hex: string;
  ring: string;
}

export function healthColor(score: number): HealthColor {
  if (score >= 80) return { label: "Excellent", hex: "#4CAF50", ring: "stroke-emerald-500" };
  if (score >= 60) return { label: "Good", hex: "#FFC107", ring: "stroke-amber-400" };
  if (score >= 40) return { label: "Needs Attention", hex: "#FF9800", ring: "stroke-orange-500" };
  return { label: "Critical", hex: "#F44336", ring: "stroke-red-500" };
}
