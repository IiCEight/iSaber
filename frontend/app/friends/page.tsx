import type { Metadata } from "next";
import Link from "next/link";
import { backendOrigin } from "../../lib/public-api";
import type { Friend } from "../../lib/friends";
import FriendIcon from "../../components/friend-icon";
import FriendsMotion, { FriendsMotionControl } from "../../components/friends-motion";
import FriendRequestForm, { FriendIdentity } from "./request-form";
import "./friends.css";

export const metadata: Metadata = { title: "Friends", description: "Independent minds, personal corners, and a few places worth getting lost in." };
export default async function Friends() {
  let friends: Friend[] | null = null;
  try { const response = await fetch(`${backendOrigin()}/api/v1/friends`, { cache: "no-store", signal: AbortSignal.timeout(5000) }); if (response.ok) friends = (await response.json()).items; } catch { /* A public retry state keeps the rest of the page usable. */ }
  const colors = ["friend-card-violet", "", "", "friend-card-lilac", "friend-card-peach", ""];
  return <FriendsMotion>
    <section className="friends-hero" aria-labelledby="friends-title"><div className="friends-hero-copy"><p className="friends-eyebrow">Friends / A small, connected web</p><h1 id="friends-title">Good<br />company.</h1><p className="friends-intro">Independent minds. Personal corners.<br />A few places worth getting lost in.</p><p className="friends-intro-zh" lang="zh-CN">一些有趣的人，一些值得停留的地方。</p></div>
    <div className="friends-art" aria-hidden="true"><div className="friends-art-rig"><span className="friends-circle friends-circle-one" /><span className="friends-circle friends-circle-two" /></div><span className="friends-art-caption">Separate worlds.<br />Shared curiosity.</span></div>
    <div className="friends-hero-foot"><a href="#friends-directory">Meet the neighbours <span aria-hidden="true">↓</span></a><a href="#friend-request">Request a friend link <span aria-hidden="true">↘</span></a></div></section>
    <section className="friends-directory" id="friends-directory" aria-labelledby="friends-directory-title"><div className="friends-section-heading"><h2 id="friends-directory-title">Across the web.</h2><p>{friends ? `${String(friends.length).padStart(2, "0")} ${friends.length === 1 ? "place" : "places"} to explore` : "Directory unavailable"}</p></div>
    {friends === null ? <div className="friends-state"><p className="friends-eyebrow">A brief interruption</p><h3>Connections can wait a moment.</h3><p>The directory is temporarily unavailable. Please try again shortly.</p><a href="/friends#friends-directory">Try again ↗</a></div> : friends.length === 0 ? <div className="friends-state"><p className="friends-eyebrow">Room to grow</p><h3>Room for good company.</h3><p>A few favourite corners of the web will find their way here.</p><a href="#friend-request">Request a friend link ↘</a></div> : <ol className="friends-grid">{friends.map((friend, i) => <li className={`friend-card ${colors[i % colors.length]}`} key={friend.id}><article><div className="friend-card-top"><span className="friend-number">{String(i + 1).padStart(2, "0")} /</span><FriendIcon name={friend.site_name} url={friend.icon_url} className={i % 2 ? "friend-avatar-peach" : "friend-avatar-outline"} /></div><h3>{friend.site_name}</h3><p>{friend.introduction}</p><div className="friend-card-bottom"><span>{new URL(friend.site_url).hostname}</span><a className="friend-visit" href={friend.site_url} target="_blank" rel="noopener noreferrer"><span>Visit site<span className="friends-sr-only">: {friend.site_name} (opens in a new tab)</span></span><span aria-hidden="true">↗</span></a></div></article></li>)}</ol>}
    <div className="friends-directory-foot"><p>A small collection, with room for more.</p><FriendsMotionControl /><a href="#content">Back to the top ↑</a></div></section>
    <section className="friends-request" id="friend-request" aria-labelledby="friend-request-title"><div className="friends-request-intro"><p className="friends-eyebrow">Your corner of the web, meet mine</p><h2 id="friend-request-title">Let’s<br /> connect.</h2><p>Have a personal site? Leave a little introduction.<br />I’d love to find out what you’re making.</p><p lang="zh-CN">交换一个链接，也认识一个有趣的人。</p><FriendIdentity /></div><FriendRequestForm /></section>
    <footer className="friends-footer"><Link className="site-brand" href="/">iSaber</Link><nav aria-label="Footer"><Link href="/archive">Archive</Link><Link href="/friends" aria-current="page">Friends</Link><Link href="/about">About</Link></nav></footer>
  </FriendsMotion>;
}
