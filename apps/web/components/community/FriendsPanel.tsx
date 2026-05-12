"use client";

import { useCallback, useEffect, useState } from "react";
import { Activity, Check, UserPlus, Users } from "lucide-react";
import { apiFetch, type ActivityEvent, type FriendConnection } from "@/lib/api";

type FriendsPayload = {
  friends: FriendConnection[];
  pending_inbound: FriendConnection[];
  pending_outbound: FriendConnection[];
};

export function FriendsPanel() {
  const [friends, setFriends] = useState<FriendConnection[]>([]);
  const [pendingInbound, setPendingInbound] = useState<FriendConnection[]>([]);
  const [pendingOutbound, setPendingOutbound] = useState<FriendConnection[]>([]);
  const [activity, setActivity] = useState<ActivityEvent[]>([]);
  const [username, setUsername] = useState("");
  const [message, setMessage] = useState("Login, send a friend request, and accept it to see friend activity.");
  const [loading, setLoading] = useState(false);

  const loadSocial = useCallback(async () => {
    const token = localStorage.getItem("mangahub_token");
    if (!token) return;
    const [friendsPayload, activityPayload] = await Promise.all([
      apiFetch<FriendsPayload>("/users/friends", { token }),
      apiFetch<{ activity: ActivityEvent[] }>("/users/activity", { token }),
    ]);
    setFriends(friendsPayload.friends);
    setPendingInbound(friendsPayload.pending_inbound);
    setPendingOutbound(friendsPayload.pending_outbound);
    setActivity(activityPayload.activity);
  }, []);

  useEffect(() => {
    loadSocial().catch((error) => setMessage(error instanceof Error ? error.message : "Failed to load social data."));
  }, [loadSocial]);

  async function sendRequest() {
    const token = localStorage.getItem("mangahub_token");
    if (!token) {
      setMessage("Login before adding friends.");
      return;
    }
    setLoading(true);
    try {
      await apiFetch("/users/friends/request", {
        method: "POST",
        token,
        body: JSON.stringify({ username }),
      });
      setUsername("");
      setMessage("Friend request sent.");
      await loadSocial();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Friend request failed.");
    } finally {
      setLoading(false);
    }
  }

  async function respond(requesterId: string, status: "accepted" | "declined") {
    const token = localStorage.getItem("mangahub_token");
    if (!token) return;
    setLoading(true);
    try {
      await apiFetch("/users/friends/respond", {
        method: "POST",
        token,
        body: JSON.stringify({ requester_id: requesterId, status }),
      });
      setMessage(`Friend request ${status}.`);
      await loadSocial();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Friend response failed.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="social-grid">
      {/* intent: expose UC-020 and UC-021 in a compact browser demo */}
      {/* status: done */}
      {/* next: add notifications when friend requests should be real-time */}
      {/* blockers: none */}
      {/* confidence: high */}
      <article className="content-panel social-card">
        <p className="kicker">Friends</p>
        <h2><UserPlus size={18} /> Add a reader</h2>
        <div className="inline-action-form">
          <input value={username} onChange={(event) => setUsername(event.target.value)} placeholder="friend_username" />
          <button type="button" className="button-primary reader-button" onClick={sendRequest} disabled={loading}>Send request</button>
        </div>
        {message ? <p className="auth-message">{message}</p> : null}
        <h3>Accepted friends</h3>
        <div className="compact-list">
          {friends.length ? friends.map((friend) => <span key={friend.user_id}><Users size={14} /> {friend.username}</span>) : <span>No friends yet.</span>}
        </div>
      </article>

      <article className="content-panel social-card">
        <p className="kicker">Requests</p>
        <h2><Check size={18} /> Pending approvals</h2>
        <div className="request-list">
          {pendingInbound.length ? pendingInbound.map((friend) => (
            <div key={friend.user_id}>
              <span>{friend.username}</span>
              <button type="button" onClick={() => respond(friend.user_id, "accepted")} disabled={loading}>Accept</button>
              <button type="button" onClick={() => respond(friend.user_id, "declined")} disabled={loading}>Decline</button>
            </div>
          )) : <div className="empty-state">No inbound requests.</div>}
        </div>
        <h3>Sent requests</h3>
        <div className="compact-list">
          {pendingOutbound.length ? pendingOutbound.map((friend) => <span key={friend.user_id}>{friend.username} · pending</span>) : <span>No sent requests.</span>}
        </div>
      </article>

      <article className="content-panel social-card social-card-wide">
        <p className="kicker">Activity</p>
        <h2><Activity size={18} /> Friend activity feed</h2>
        <div className="activity-feed-list">
          {activity.length ? activity.map((event) => (
            <div key={`${event.type}-${event.user_id}-${event.manga_id}-${event.created_at}`}>
              <strong>{event.username}</strong>
              <span>{event.type === "review" ? `rated ${event.title} ${event.rating}/5` : `completed ${event.title}`}</span>
              {event.body ? <p>{event.body}</p> : null}
            </div>
          )) : <div className="empty-state">No friend activity yet.</div>}
        </div>
      </article>
    </section>
  );
}
