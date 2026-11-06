import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Owner access",
  robots: { index: false, follow: false },
};

// No management data or simulated authentication is exposed by this shell.
export default function Admin() {
  return (
    <>
      <h1>Owner access</h1>
      <p>The dashboard is being prepared. Sign-in is not available yet.</p>
    </>
  );
}
