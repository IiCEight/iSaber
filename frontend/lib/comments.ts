export type PublicComment = { id: string; display_name: string; body: string; created_at: string };
export type CommentStatus = "pending" | "approved" | "rejected" | "removed";
export type AdminComment = PublicComment & { article_id: string; status: CommentStatus; version: number; moderated_at: string | null };
