import type { Anekdot } from "../types";

interface Props {
  heading: string;
  description: string;
  anekdots: Anekdot[];
  loading: boolean;
  error: string | null;
  likedIDs: ReadonlySet<number>;
  canLike: boolean;
  busyID: number | null;
  emptyText: string;
  onToggleLike: (target: Anekdot) => void;
  onReload: () => void;
}

export function AnekdotListPanel({
  heading,
  description,
  anekdots,
  loading,
  error,
  likedIDs,
  canLike,
  busyID,
  emptyText,
  onToggleLike,
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

            return (
              <li key={item.id} className="anekdot-item">
                <p className="anekdot-item-text">{item.text}</p>
                <div className="anekdot-item-meta">
                  <span className="badge">{item.category_name}</span>
                  <button
                    type="button"
                    className={liked ? "like-button like-button-active" : "like-button"}
                    aria-pressed={liked}
                    aria-label={liked ? "Убрать лайк" : "Поставить лайк"}
                    title={canLike ? (liked ? "Убрать лайк" : "Поставить лайк") : "Войдите, чтобы ставить лайки"}
                    disabled={!canLike || busyID === item.id}
                    onClick={() => onToggleLike(item)}
                  >
                    <span aria-hidden="true">{liked ? "♥" : "♡"}</span>
                    <span className="like-count">{item.like_count}</span>
                  </button>
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
