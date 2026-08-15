import { PageHeader } from "@/components/PageHeader";
import { IsoClient } from "./IsoClient";

export const dynamic = "force-dynamic";

export default function IsoPage() {
  return (
    <div className="mx-auto max-w-7xl px-4 py-6 sm:px-8 sm:py-8">
      <PageHeader title="ISO Builder" subtitle="MicroWin-style custom Windows image builder — debloat, privacy tweaks, and TPM bypass baked into a bootable ISO." />
      <IsoClient />
    </div>
  );
}
