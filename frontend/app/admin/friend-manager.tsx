"use client";
import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { useAdminConfirmation } from "./admin-confirmation";
import { AdminBadge, AdminNote } from "./admin-ui";
import type { AdminFriend, FriendDetails, FriendRequest } from "../../lib/friends";
import type { NumberedPage } from "../../lib/articles";

type Request = <T>(path: string, init?: RequestInit, signature?: string) => Promise<T>;
function Fields({ value, readOnly = false }: { value?: FriendDetails; readOnly?: boolean }) {
  return <><div className="two-col"><label>Website name<input readOnly={readOnly} name="site_name" maxLength={80} defaultValue={value?.site_name} required /></label><label>Website URL<input readOnly={readOnly} name="site_url" type="url" maxLength={2048} defaultValue={value?.site_url} required placeholder="https://" /></label></div><label>Introduction<textarea readOnly={readOnly} name="introduction" maxLength={300} rows={3} defaultValue={value?.introduction} required /></label><label>Profile icon URL (optional)<input readOnly={readOnly} name="icon_url" type="url" maxLength={2048} defaultValue={value?.icon_url} placeholder="https://your-site.example/icon.png" /></label></>;
}
export default function FriendManager({ active, busy, request, run, onDirty }: { active: boolean; busy: boolean; request: Request; run: (action: () => Promise<void>) => Promise<void>; onDirty: (dirty: boolean) => void }) {
  const confirm = useAdminConfirmation();
  const [friends, setFriends] = useState<AdminFriend[] | null>(null);
  const [requests, setRequests] = useState<NumberedPage<FriendRequest> | null>(null);
  const [status, setStatus] = useState("pending"), [page, setPage] = useState(1);
  const [message, setMessage] = useState(""), [loading, setLoading] = useState(false), [dirty, setDirty] = useState(false);
  const generation = useRef(0);
  const newForm = useRef<HTMLFormElement>(null);
  const root = useRef<HTMLElement>(null);
  const createAttempt = useRef<{ fields: Record<string, string>; signature: string } | null>(null);
  const [createUncertain, setCreateUncertain] = useState(false);
  const dirtyKeys = useRef(new Set<string>());
  const setChanged = (key: string, value = true) => { if (value) dirtyKeys.current.add(key); else dirtyKeys.current.delete(key); const changed = dirtyKeys.current.size > 0; setDirty(changed); onDirty(changed); };
  const loadDirectory = useCallback(async () => { const result = await request<{ items: AdminFriend[] }>("/friends"); setFriends(result.items); }, [request]);
  const loadRequests = useCallback(async () => { const seq=++generation.current; const result=await request<NumberedPage<FriendRequest>>(`/friend-requests?page=${page}&limit=20${status ? `&status=${status}` : ""}`); if(seq===generation.current) setRequests(result); }, [request, page, status]);
  useEffect(() => { if (!active || friends !== null) return; let live=true; setLoading(true); loadDirectory().catch(() => {if(live)setMessage("The directory could not be loaded. Use Reload directory to try again.");}).finally(()=>{if(live)setLoading(false);});return()=>{live=false;}; }, [active, friends, loadDirectory]);
  useEffect(() => { if (!active) return; let live=true;setRequests(null);loadRequests().catch(()=>{if(live)setMessage("Requests could not be loaded. Use Reload requests to try again.");});return()=>{live=false;generation.current++;}; }, [active, loadRequests]);
  async function discard() {
    if (!dirty) return true;
    if (!await confirm("Discard unsaved friend-link edits before refreshing this directory?", { title: "Leave unsaved changes?", action: "Discard changes", danger: true })) return false;
    if (createUncertain) { setMessage("Retry the unconfirmed draft creation before refreshing the directory."); return false; }
    root.current?.querySelectorAll("form").forEach(form => { form.reset(); });dirtyKeys.current.clear();setDirty(false);onDirty(false);return true;
  }
  async function save(event: FormEvent<HTMLFormElement>, friend?: AdminFriend) {
    event.preventDefault();const form=event.currentTarget;const data=new FormData(form);
    const fields=Object.fromEntries(["site_name","site_url","introduction","icon_url"].map(key=>[key,String(data.get(key)??"").trim()]));
    const published=data.get("published")==="on";
    if(friend && published!==friend.published && !await confirm(published ? `Publish “${fields.site_name}” in the public Friends directory?` : `Hide “${fields.site_name}” from the public Friends directory?`))return;
    await run(async()=>{
      if(friend) await request(`/friends/${friend.id}`,{method:"PATCH",headers:{"Content-Type":"application/json"},body:JSON.stringify({...fields,published,expected_version:friend.version})});
      else {
        if (!createAttempt.current) createAttempt.current = { fields, signature: `friend-create:${JSON.stringify(fields)}` };
        const attempt = createAttempt.current;
        try { await request("/friends", {method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(attempt.fields)},attempt.signature); createAttempt.current = null; setCreateUncertain(false); }
        catch (error) { const problem = error as {status?:number;code?:string}; const uncertain = !problem.status || problem.status>=500 || problem.code==="request_in_progress"; setCreateUncertain(uncertain); if (!uncertain) createAttempt.current = null; throw error; }
      }
      setChanged(friend?.id ?? "new", false);if(!friend)form.reset();
      setMessage(friend ? "Friend link saved." : "Draft created. Review it below before publishing.");
      try {await loadDirectory();}catch{setMessage("Saved. Reload the directory to see the updated version.");}
    });
  }
  async function move(index: number, direction: number) {
    if(!friends || !await discard())return;
    const ordered=[...friends];[ordered[index],ordered[index+direction]]=[ordered[index+direction],ordered[index]];
    await request("/friends/order",{method:"PUT",headers:{"Content-Type":"application/json"},body:JSON.stringify({items:ordered.map(friend=>({id:friend.id,expected_version:friend.version}))})});
    setMessage("Directory order saved.");await loadDirectory();
  }
  async function moderate(item: FriendRequest, next: "approved" | "rejected") {
    if(!await confirm(next==="approved" ? `Approve “${item.site_name}” and create a hidden draft for review? It will not be public until you publish it.` : `Reject the request from “${item.site_name}”?`))return;
    if(!await discard())return;
    await request(`/friend-requests/${item.id}`,{method:"PATCH",headers:{"Content-Type":"application/json"},body:JSON.stringify({status:next,expected_version:item.version})});
    setMessage(next==="approved" ? "Request approved. Review the new draft in the directory below before publishing." : "Request rejected.");
    try {await Promise.all([loadDirectory(),loadRequests()]);}catch{setMessage("Review saved. Reload the directory and requests to see the result.");}
  }
  return <section aria-label="Friend link management" ref={root}>
    {message&&<p className="admin-inline-note" role="status">{message}</p>}
    <section className="card"><div className="card-head"><h2>Friend-link requests</h2><a href="/friends" target="_blank" rel="noreferrer">View Friends ↗</a></div>
    <div className="toolbar"><label>Status<select value={status} disabled={busy} onChange={e=>{setStatus(e.target.value);setPage(1);}}><option value="pending">Pending</option><option value="approved">Approved</option><option value="rejected">Rejected</option><option value="">All statuses</option></select></label><button className="button" type="button" disabled={busy} onClick={()=>run(loadRequests)}>Reload requests</button></div>
    {requests ? requests.items.length ? <ol className="comment-list">{requests.items.map(item=><li className="comment-item" key={item.id}><article><div className="comment-heading"><h3>{item.site_name}</h3><AdminBadge tone={item.status==="pending"?"amber":item.status==="approved"?"green":""}>{item.status}</AdminBadge></div><p><a href={item.site_url} target="_blank" rel="noopener noreferrer">{item.site_url} ↗</a></p><p className="comment-body">{item.introduction}</p>{item.icon_url&&<p>Icon URL: <a href={item.icon_url} target="_blank" rel="noopener noreferrer">{item.icon_url} ↗</a></p>}{item.email&&<p>Private reply email: {item.email}</p>}<p className="help">Submitted {new Date(item.created_at).toLocaleString("en",{timeZone:"Asia/Shanghai"})}</p>{item.status==="pending"&&<div className="comment-actions"><button type="button" className="button primary" disabled={busy} onClick={()=>run(()=>moderate(item,"approved"))}>Approve as draft</button><button type="button" className="button danger" disabled={busy} onClick={()=>run(()=>moderate(item,"rejected"))}>Reject request</button></div>}{item.friend_id&&<a href={`#friend-${item.friend_id}`}>Review directory entry ↓</a>}</article></li>)}</ol>:<p className="empty">No {status||"matching"} requests.</p>:<p className="empty" role="status">Use Reload requests if the review queue does not load.</p>}
    {requests&&<div className="table-foot"><span>{requests.total} requests · Page {requests.page} of {requests.total_pages}</span><div><button className="button small" type="button" disabled={busy||requests.page<=1} onClick={()=>setPage(requests.page-1)}>Previous requests</button><button className="button small" type="button" disabled={busy||requests.page>=requests.total_pages} onClick={()=>setPage(requests.page+1)}>Next requests</button></div></div>}</section>
    <section className="card"><div className="card-head"><h2>Add a friend link</h2></div><fieldset className="card-body" disabled={busy}><legend className="sr-only">Create a hidden draft</legend><form className="fields" ref={newForm} onInput={()=>setChanged("new")} onSubmit={e=>void save(e)}><Fields readOnly={createUncertain} />{createUncertain && <p role="status">Creation could not be confirmed. Retry the same draft before editing; your fields are preserved.</p>}<div><button className="button primary" type="submit">{createUncertain ? "Retry draft creation" : "Create draft link"}</button></div></form></fieldset></section>
    <section className="card"><div className="card-head"><div><h2>Directory</h2><p>Up to 200 entries, in your chosen order.</p></div><button className="button" type="button" disabled={busy||loading} onClick={()=>run(async()=>{if(await discard())await loadDirectory();})}>Reload directory</button></div>
    {friends ? friends.length ? friends.map((friend,index)=><section className="card-body admin-friend-editor" key={`${friend.id}:${friend.version}`} id={`friend-${friend.id}`}><div className="comment-heading"><h3>{friend.site_name}</h3><AdminBadge tone={friend.published?"green":"amber"}>{friend.published?"Published":"Hidden draft"}</AdminBadge></div><fieldset disabled={busy}><legend className="sr-only">Edit {friend.site_name}</legend><form className="fields" onInput={()=>setChanged(friend.id)} onSubmit={e=>void save(e,friend)}><Fields value={friend}/><label className="switch-check"><input name="published" type="checkbox" defaultChecked={friend.published}/>Published in the public directory</label><div className="comment-actions"><button className="button primary" type="submit">Save {friend.site_name}</button><button className="button" type="button" disabled={index===0} onClick={()=>run(()=>move(index,-1))} aria-label={`Move ${friend.site_name} up`}>Move up ↑</button><button className="button" type="button" disabled={index===friends.length-1} onClick={()=>run(()=>move(index,1))} aria-label={`Move ${friend.site_name} down`}>Move down ↓</button></div></form></fieldset></section>):<p className="empty">No entries yet. Add a draft or approve a request.</p>:<p className="empty">{loading?"Loading directory…":"Reload the directory to try again."}</p>}</section>
    <AdminNote>Requests and reply emails are owner-only. Approval creates a hidden draft. Publish it separately after reviewing the name, URLs, and introduction. Hiding retains the entry. No reply email is sent automatically.</AdminNote>
  </section>;
}
