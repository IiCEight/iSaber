import type { Metadata } from "next";
import { cookies } from "next/headers";
import OwnerAccess, { type OwnerSession } from "./owner-access";

export const metadata: Metadata = {
  title: "Owner access",
  robots: { index: false, follow: false },
};

export default async function Admin() {
  let session: OwnerSession | null = null;
  let unavailable = false;
  try {
    const jar = await cookies();
    const backend = new URL(process.env.BACKEND_ORIGIN ?? "http://127.0.0.1:8080").origin;
    const response = await fetch(`${backend}/api/v1/auth/session`, {
      headers: { Cookie: jar.toString() },
      cache: "no-store",
      signal: AbortSignal.timeout(3000),
    });
    if (response.ok) session = await response.json();
    else if (response.status !== 401) unavailable = true;
  } catch {
    unavailable = true;
  }
  // Only an authenticated response can supply owner data. The client keeps the
  // CSRF token in memory; the opaque session cookie remains HttpOnly.
  return <OwnerAccess initialSession={session} unavailable={unavailable} />;
}
