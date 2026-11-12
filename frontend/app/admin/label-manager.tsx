"use client";

import { useAdminConfirmation } from "./admin-confirmation";
import { AdminNote } from "./admin-ui";
import type { FormEvent } from "react";
import type { AdminLabel } from "../../lib/articles";

type Request = <T>(path: string, init?: RequestInit) => Promise<T>;
type Props = {
  labels: { categories: AdminLabel[]; tags: AdminLabel[] } | null;
  busy: boolean;
  request: Request;
  run: (action: () => Promise<void>) => Promise<void>;
  onChanged: () => Promise<void>;
  refresh: () => Promise<void>;
};

export default function LabelManager({ labels, busy, request, run, onChanged, refresh }: Props) {
  const confirmAction = useAdminConfirmation();
  async function create(event: FormEvent<HTMLFormElement>, kind: "categories" | "tags") {
    event.preventDefault(); const form = event.currentTarget; const data = new FormData(form);
    await run(async () => {
      await request<AdminLabel>(`/${kind}`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name: data.get("name"), slug: data.get("slug") }) });
      form.reset(); await onChanged();
    });
  }
  async function rename(event: FormEvent<HTMLFormElement>, kind: "categories" | "tags", label: AdminLabel) {
    event.preventDefault(); const name = new FormData(event.currentTarget).get("name");
    await run(async () => {
      if (name === label.name) return;
      if (!await confirmAction(`Rename “${label.name}” to “${name}”? This updates the display name wherever it is used, including published articles and history.`)) return;
      await request<AdminLabel>(`/${kind}/${label.id}`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name, expected_version: label.version }) });
      await onChanged();
    });
  }
  return <section id="labels" aria-labelledby="labels-title">
    <div className="admin-section-toolbar"><h2 id="labels-title" className="sr-only">Label management</h2><button type="button" className="button" disabled={busy} onClick={() => run(refresh)}>Refresh labels</button></div>
    {!labels ? <div className="card empty"><h2>Labels are unavailable</h2><p>Refresh labels to try again. Existing article associations are preserved.</p></div> : <div className="label-grid">{(["categories", "tags"] as const).map(kind => <section className="card" key={kind} aria-labelledby={`${kind}-title`}>
      <div className="card-head"><div><h2 id={`${kind}-title`}>{kind === "categories" ? "Categories" : "Tags"}</h2><p>{kind === "categories" ? "One main category for each article." : "Multiple topics across the collection."}</p></div></div>
      <fieldset disabled={busy} className="card-body"><legend className="sr-only">Create {kind === "categories" ? "category" : "tag"}</legend>
        <form className="fields" onSubmit={event => create(event, kind)}>
          <div className="two-col"><label htmlFor={`${kind}-name`}>Name<input id={`${kind}-name`} name="name" maxLength={120} required placeholder={kind === "categories" ? "e.g. Technology" : "e.g. Learning"} /></label><label htmlFor={`${kind}-slug`}>URL name<input id={`${kind}-slug`} name="slug" maxLength={160} pattern="[a-z0-9]+(-[a-z0-9]+)*" required placeholder={kind === "categories" ? "e.g. technology" : "e.g. learning"} /></label></div>
          <div><button className="button primary" type="submit">Create {kind === "categories" ? "category" : "tag"}</button></div>
        </form>
      </fieldset>
      {labels[kind].length === 0 ? <p className="card-body muted">No {kind} yet.</p> : <ul className="admin-label-list">{labels[kind].map(label => <li key={label.id}><form key={label.version} className="admin-label-row" onSubmit={event => rename(event, kind, label)}>
        <label htmlFor={`rename-${label.id}`}><span className="sr-only">Name for {label.slug}</span><input id={`rename-${label.id}`} name="name" defaultValue={label.name} maxLength={120} required disabled={busy} /></label><code>{label.slug}</code><button className="button small" type="submit" disabled={busy} aria-label={`Rename ${label.name}`}>Rename</button>
      </form></li>)}</ul>}
    </section>)}</div>}
    <AdminNote>Renaming updates display names across articles and history. URL names and article associations stay fixed. Label deletion is unavailable.</AdminNote>
  </section>;
}
