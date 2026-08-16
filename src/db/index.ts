import { drizzle, type NodePgDatabase } from "drizzle-orm/node-postgres";
import { Pool } from "pg";
import * as schema from "./schema";

// The Pool and Drizzle instance are constructed lazily so that importing this
// module (which every page, API route, and server action does at the top
// level) never requires DATABASE_URL at build time. Next.js evaluates modules
// during "collect page data" even for force-dynamic routes; constructing the
// pool there made `next build` fail when DATABASE_URL was unset. The real
// connection is created on the first query — i.e. at request time.
let dbInstance: NodePgDatabase<typeof schema> | null = null;

function createDb(): NodePgDatabase<typeof schema> {
  const databaseUrl = process.env.DATABASE_URL;
  if (!databaseUrl) {
    throw new Error("DATABASE_URL is required");
  }
  const pool = new Pool({ connectionString: databaseUrl });
  return drizzle(pool, { schema });
}

function getDb(): NodePgDatabase<typeof schema> {
  if (!dbInstance) {
    dbInstance = createDb();
  }
  return dbInstance;
}

// A transparent proxy so existing `db.select()...` / `db.insert()...` call
// sites keep working unchanged while construction is deferred to first use.
export const db = new Proxy({} as NodePgDatabase<typeof schema>, {
  get(_target, prop, receiver) {
    const real = getDb();
    const value = Reflect.get(real, prop, receiver);
    return typeof value === "function" ? (value as (...args: unknown[]) => unknown).bind(real) : value;
  },
}) as NodePgDatabase<typeof schema>;

// In development, allow HMR to reuse the same instance across hot reloads.
const globalForDb = globalThis as typeof globalThis & {
  __winforgeDb?: NodePgDatabase<typeof schema> | null;
};
if (process.env.NODE_ENV !== "production") {
  Object.defineProperty(globalForDb, "__winforgeDb", {
    get: () => dbInstance,
    set(v: NodePgDatabase<typeof schema> | undefined | null) {
      dbInstance = v ?? null;
    },
    configurable: true,
  });
}

export { getDb };
