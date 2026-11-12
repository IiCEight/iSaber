import type { ReactNode } from "react";
import { AdminConfirmationProvider } from "./admin-confirmation";
import "./admin.css";

export default function AdminLayout({ children }: { children: ReactNode }) {
  return <div className="admin-page" data-admin-ui=""><AdminConfirmationProvider>{children}</AdminConfirmationProvider></div>;
}
