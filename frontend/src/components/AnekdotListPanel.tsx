import type { Anekdot } from "../types";

interface Props {
  heading: string;
  description: string;
  anekdots: Anekdot[];
  loading: boolean;
  error: string | null;
  likedIDs: ReadonlySet<number>;
  favoriteIDs: ReadonlySet<number>;
  canReact: boolean;
  busyID: number | null;
  emptyText: string;
  onToggleLike: (target: Anekdot) => void;
  onToggleFavorite: (target: Anekdot) => void;
  onReload: () => void;
}

export function AnekdotListPanel({
  heading,
  description,
  anekdots,
  loading,
  error,
  likedIDs,
  favoriteIDs,
  canReact,
  busyID,
  emptyText,
  onToggleLike,
  onToggleFavorite,
  onReload,
}: Props) {
  return (
    <section className="panel">
      <div className="panel-heading">
        <div>
          <h2>{heading}</h2>
          <p className="muted">{description}</p>
        </div>
        <button type="button" className="button" onClick={onReload} disabled={loading}>
          Обновить
        </button>
      </div>

      {error !== null && <p className="alert">{error}</p>}

      {loading && anekdots.length === 0 && <p className="empty-state">Загружаем анекдоты…</p>}

      {!loading && error === null && anekdots.length === 0 && <p className="empty-state">{emptyText}</p>}

      {anekdots.length > 0 && (
        <ul className="anekdot-list">
          {anekdots.map((item) => {
            const liked = likedIDs.has(item.id);
            const favorite = favoriteIDs.has(item.id);
            const disabled = !canReact || busyID === item.id;
            const likeTitle = canReact ? (liked ? "Убрать лайк" : "Поставить лайк") : "Войдите, чтобы ставить лайки";
            const favoriteTitle = canReact
              ? favorite
                ? "Убрать из избранного"
                : "Добавить в избранное"
              : "Войдите, чтобы добавлять в избранное";

            return (
              <li key={item.id} className="anekdot-item">
                <p className="anekdot-item-text">{item.text}</p>
                <div className="anekdot-item-meta">
                  <span className="badge">{item.category_name}</span>
                  <div className="anekdot-item-actions">
                    <button
                      type="button"
                      className={liked ? "like-button like-button-active" : "like-button"}
                      aria-pressed={liked}
                      aria-label={liked ? "Убрать лайк" : "Поставить лайк"}
                      title={likeTitle}
                      disabled={disabled}
                      onClick={() => onToggleLike(item)}
                    >
                      <span aria-hidden="true">{liked ? "♥" : "♡"}</span>
                      <span className="like-count">{item.like_count}</span>
                    </button>
                    <button
                      type="button"
                      className={favorite ? "favorite-button favorite-button-active" : "favorite-button"}
                      aria-pressed={favorite}
                      aria-label={favorite ? "Убрать из избранного" : "Добавить в избранное"}
                      title={favoriteTitle}
                      disabled={disabled}
                      onClick={() => onToggleFavorite(item)}
                    >
                      <span aria-hidden="true">{favorite ? "★" : "☆"}</span>
                    </button>
                  </div>
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
