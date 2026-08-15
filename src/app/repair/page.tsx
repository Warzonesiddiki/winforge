import { PageHeader } from "@/components/PageHeader";
import { RepairClient } from "./RepairClient";

export const dynamic = "force-dynamic";

export default function RepairPage() {
  return (
    <div className="mx-auto max-w-7xl px-4 py-6 sm:px-8 sm:py-8">
      <PageHeader title="System Repair" subtitle="One-click fixes for Windows Update, component store corruption, and networking issues." />
      <RepairClient />
    </div>
  );
}
