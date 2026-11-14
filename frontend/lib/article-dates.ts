// The dashboard uses Asia/Shanghai (UTC+08:00), independent of browser timezone.
export function updatedAtInput(value: string | null | undefined): string {
  if (!value) return "";
  return new Date(new Date(value).getTime() + 8 * 60 * 60 * 1000).toISOString().slice(0, -1);
}

export function updatedAtTimestamp(value: string): string | null {
  return value ? new Date(`${value}+08:00`).toISOString() : null;
}
