"use client";

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
      if (!confirm(`Rename “${label.name}” to “${name}”? This updates the display name wherever it is used, including published articles and history.`)) return;
      await request<AdminLabel>(`/${kind}/${label.id}`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name, expected_version: label.version }) });
      await onChanged();
    });
  }
  return <section id="labels" aria-labelledby="labels-title">
    <h2 id="labels-title">Categories and tags</h2>
    <p>Create labels here, then select them in draft metadata. Renaming keeps their URLs and article associations. Deletion is unavailable.</p>
    <button type="button" disabled={busy} onClick={() => run(refresh)}>Refresh labels</button>
    {!labels ? <p>Labels have not loaded. Refresh to try again.</p> : (["categories", "tags"] as const).map((kind) => <section key={kind} aria-labelledby={`${kind}-title`}>
      <h3 id={`${kind}-title`}>{kind === "categories" ? "Categories" : "Tags"}</h3>
      <fieldset disabled={busy}>
        <legend>Create {kind === "categories" ? "category" : "tag"}</legend>
        <form onSubmit={(event) => create(event, kind)}>
          <label htmlFor={`${kind}-name`}>Name</label><input id={`${kind}-name`} name="name" maxLength={120} required />
          <label htmlFor={`${kind}-slug`}>URL name</label><input id={`${kind}-slug`} name="slug" maxLength={160} pattern="[a-z0-9]+(-[a-z0-9]+)*" required />
          <button type="submit">Create {kind === "categories" ? "category" : "tag"}</button>
        </form>
      </fieldset>
      {labels[kind].length === 0 ? <p>No {kind} yet.</p> : <ul>{labels[kind].map((label) => <li key={label.id}>
        <form key={label.version} onSubmit={(event) => rename(event, kind, label)}>
          <label htmlFor={`rename-${label.id}`}>Name · {label.slug}</label><input id={`rename-${label.id}`} name="name" defaultValue={label.name} maxLength={120} required disabled={busy} />
          <button type="submit" disabled={busy}>Rename</button>
        </form>
      </li>)}</ul>}
    </section>)}
  </section>;
}
