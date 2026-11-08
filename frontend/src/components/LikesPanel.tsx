import { AnekdotListPanel } from "./AnekdotListPanel";
import type { Anekdot, User } from "../types";

interface Props {
  user: User | null;
  likes: Anekdot[];
  loading: boolean;
  error: string | null;
  busyID: number | null;
  onToggleLike: (target: Anekdot) => void;
  onReload: () => void;
}

export function LikesPanel({ user, likes, loading, error, busyID, onToggleLike, onReload }: Props) {
  if (user === null) {
    return (
      <section className="panel">
        <h2>Лайки</h2>
        <p className="empty-state">Войдите в аккаунт, чтобы видеть лайкнутые анекдоты.</p>
      </section>
    );
  }

  return (
    <AnekdotListPanel
      heading="Лайки"
      description="Анекдоты, которые вам понравились"
      anekdots={likes}
      loading={loading}
      error={error}
      likedIDs={new Set(likes.map((item) => item.id))}
      canLike
      busyID={busyID}
      emptyText="Вы пока не лайкнули ни одного анекдота."
      onToggleLike={onToggleLike}
      onReload={onReload}
    />
  );
}
