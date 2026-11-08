import { AnekdotListPanel } from "./AnekdotListPanel";
import type { Anekdot, Collection, User } from "../types";

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
  notice?: string | null;
  collections?: Collection[];
  onToggleLike: (target: Anekdot) => void;
  onToggleFavorite: (target: Anekdot) => void;
  onAddToCollection?: (target: Anekdot, collectionId: number) => void;
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
  notice = null,
  collections = [],
  onToggleLike,
  onToggleFavorite,
  onAddToCollection,
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
      notice={notice}
      collections={collections}
      onToggleLike={onToggleLike}
      onToggleFavorite={onToggleFavorite}
      onAddToCollection={onAddToCollection}
      onReload={onReload}
    />
  );
}
