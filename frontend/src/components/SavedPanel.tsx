import { AnekdotListPanel } from "./AnekdotListPanel";
import type { Anekdot, User } from "../types";

interface Props {
  user: User | null;
  heading: string;
  description: string;
  guestText: string;
  emptyText: string;
  anekdots: Anekdot[];
  loading: boolean;
  error: string | null;
  likedIDs: ReadonlySet<number>;
  favoriteIDs: ReadonlySet<number>;
  busyID: number | null;
  onToggleLike: (target: Anekdot) => void;
  onToggleFavorite: (target: Anekdot) => void;
  onReload: () => void;
}

export function SavedPanel({
  user,
  heading,
  description,
  guestText,
  emptyText,
  anekdots,
  loading,
  error,
  likedIDs,
  favoriteIDs,
  busyID,
  onToggleLike,
  onToggleFavorite,
  onReload,
}: Props) {
  if (user === null) {
    return (
      <section className="panel">
        <h2>{heading}</h2>
        <p className="empty-state">{guestText}</p>
      </section>
    );
  }

  return (
    <AnekdotListPanel
      heading={heading}
      description={description}
      anekdots={anekdots}
      loading={loading}
      error={error}
      likedIDs={likedIDs}
      favoriteIDs={favoriteIDs}
      canReact
      busyID={busyID}
      emptyText={emptyText}
      onToggleLike={onToggleLike}
      onToggleFavorite={onToggleFavorite}
      onReload={onReload}
    />
  );
}
