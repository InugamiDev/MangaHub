"use client";

import { useCallback, useEffect, useState } from "react";
import { MessageSquare, Star } from "lucide-react";
import { apiFetch, type Review, type ReviewSummary } from "@/lib/api";

type ReviewPayload = {
  reviews: Review[];
  summary: ReviewSummary;
};

export function ReviewPanel({ mangaId }: { mangaId: string }) {
  const [reviews, setReviews] = useState<Review[]>([]);
  const [summary, setSummary] = useState<ReviewSummary>({ average_rating: 0, review_count: 0 });
  const [rating, setRating] = useState(5);
  const [body, setBody] = useState("");
  const [message, setMessage] = useState("Complete this manga in your library before submitting a review.");
  const [loading, setLoading] = useState(false);

  const loadReviews = useCallback(async () => {
    const payload = await apiFetch<ReviewPayload>(`/manga/${mangaId}/reviews`);
    setReviews(payload.reviews);
    setSummary(payload.summary);
  }, [mangaId]);

  useEffect(() => {
    loadReviews().catch((error) => setMessage(error instanceof Error ? error.message : "Failed to load reviews."));
  }, [loadReviews]);

  async function submitReview() {
    const token = localStorage.getItem("mangahub_token");
    if (!token) {
      setMessage("Login before submitting a review.");
      return;
    }
    setLoading(true);
    setMessage("");
    try {
      await apiFetch(`/manga/${mangaId}/reviews`, {
        method: "POST",
        token,
        body: JSON.stringify({ rating, body }),
      });
      setBody("");
      setMessage("Review saved. Community rating updated.");
      await loadReviews();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Review failed.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="content-panel review-panel">
      {/* intent: expose UC-018 and UC-019 from the public manga detail page */}
      {/* status: done */}
      {/* next: add moderation controls if reviews become public production content */}
      {/* blockers: none */}
      {/* confidence: high */}
      <div className="panel-heading-row">
        <div>
          <p className="kicker">Community</p>
          <h2><MessageSquare size={18} /> Reviews and ratings</h2>
        </div>
        <strong className="rating-pill"><Star size={15} /> {summary.review_count ? `${summary.average_rating.toFixed(1)} / 5` : "No ratings"}</strong>
      </div>
      <div className="review-form-grid">
        <label>
          Rating
          <select value={rating} onChange={(event) => setRating(Number(event.target.value))}>
            {[5, 4, 3, 2, 1].map((value) => <option key={value} value={value}>{value} stars</option>)}
          </select>
        </label>
        <label className="review-body-field">
          Review
          <textarea value={body} onChange={(event) => setBody(event.target.value)} placeholder="What should another reader know after finishing this manga?" rows={4} />
        </label>
        <button type="button" className="button-primary reader-button" onClick={submitReview} disabled={loading}>
          {loading ? "Saving..." : "Submit review"}
        </button>
      </div>
      {message ? <p className="auth-message">{message}</p> : null}
      <div className="review-list">
        {reviews.length > 0 ? reviews.map((review) => (
          <article key={review.id}>
            <strong>{review.username}<span>{review.rating} / 5</span></strong>
            <p>{review.body}</p>
          </article>
        )) : <div className="empty-state">No community reviews yet.</div>}
      </div>
    </section>
  );
}
