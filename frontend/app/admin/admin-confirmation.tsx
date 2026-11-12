"use client";

import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from "react";

type Options = { title?: string; action?: string; danger?: boolean };
type Ask = (message: string, options?: Options) => Promise<boolean>;
const Confirmation = createContext<Ask | null>(null);

export function useAdminConfirmation() {
  const confirmAction = useContext(Confirmation);
  if (!confirmAction) throw new Error("Admin confirmation provider is required");
  return confirmAction;
}

export function AdminConfirmationProvider({ children }: { children: ReactNode }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const cancel = useRef<HTMLButtonElement>(null);
  const resolve = useRef<((value: boolean) => void) | null>(null);
  const [pending, setPending] = useState<({ message: string } & Options) | null>(null);
  const ask = useCallback<Ask>((message, options = {}) => new Promise(done => {
    if (resolve.current) { done(false); return; }
    resolve.current = done;
    setPending({ message, ...options });
  }), []);
  useEffect(() => {
    if (!pending || !dialog.current) return;
    dialog.current.returnValue = "cancel";
    dialog.current.showModal();
    cancel.current?.focus();
  }, [pending]);
  useEffect(() => () => { resolve.current?.(false); resolve.current = null; }, []);
  return <Confirmation.Provider value={ask}>{children}<dialog ref={dialog} className="admin-confirmation" aria-labelledby="admin-confirm-title" aria-describedby="admin-confirm-description" onClose={() => {
    resolve.current?.(dialog.current?.returnValue === "confirm");
    resolve.current = null; setPending(null);
  }}><form method="dialog"><div className="dialog-icon" aria-hidden="true">!</div><h2 id="admin-confirm-title">{pending?.title ?? "Confirm change"}</h2><p id="admin-confirm-description">{pending?.message}</p><div className="dialog-actions"><button type="submit" ref={cancel} className="button" value="cancel">Cancel</button><button type="submit" className={`button ${pending?.danger ? "danger" : "primary"}`} value="confirm">{pending?.action ?? "Confirm"}</button></div></form></dialog></Confirmation.Provider>;
}
