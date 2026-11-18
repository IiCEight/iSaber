export type FriendDetails = { site_name: string; site_url: string; introduction: string; icon_url: string };
export type Friend = FriendDetails & { id: string };
export type AdminFriend = Friend & { published: boolean; position: number; version: number; created_at: string; updated_at: string };
export type FriendRequest = FriendDetails & { id: string; email: string; status: "pending" | "approved" | "rejected"; friend_id: string | null; version: number; created_at: string; moderated_at: string | null };

export function friendImageURL(value: string) {
  if (!value.trim()) return "";
  try { const url = new URL(value.trim()); return url.protocol === "https:" && !url.username && !url.password ? url.href : ""; } catch { return ""; }
}
