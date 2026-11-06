// Process liveness only. This does not claim that Go or storage is ready.
export function GET() {
  return Response.json({ status: "ok" }, { headers: { "Cache-Control": "no-store" } });
}
