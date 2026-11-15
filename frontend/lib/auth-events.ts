"use client";

export function notifyAuthChanged(state: "signed-in" | "signed-out") {
  if (typeof BroadcastChannel === "undefined") return;
  const channel = new BroadcastChannel("isaber:auth");
  channel.postMessage(state);
  channel.close();
}
