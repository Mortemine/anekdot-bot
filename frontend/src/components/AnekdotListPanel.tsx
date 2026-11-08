import type { Anekdot, AnekdotSort } from "../types";

export interface AnekdotPaging {
  page: number;
  pageSize: number;
  total: number;
  sort: AnekdotSort;
  pageSizeOptions: readonly number[];
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
  onSortChange: (sort: AnekdotSort) => void;
}

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
  paging?: AnekdotPaging;
}

const pageWindowRadius = 3;

function pageNumbers(current: number, totalPages: number): number[] {
  const size = pageWindowRadius * 2 + 1;
  if (totalPages <= size) {
    return Array.from({ length: totalPages }, (_, index) => index + 1);
  }

  const start = Math.min(Math.max(current - pageWindowRadius, 1), totalPages - size + 1);

  return Array.from({ length: size }, (_, index) => start + index);
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
  paging,
}: Props) {
  const totalPages = paging === undefined ? 1 : Math.max(1, Math.ceil(paging.total / paging.pageSize));
  const pages = paging === undefined ? [] : pageNumbers(paging.page, totalPages);

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

      {paging !== undefined && (
        <div className="list-controls">
          <p className="control-summary muted">Всего анекдотов: {paging.total}</p>
          <label className="sort-control" title="Сортировка">
            <span className="sort-icon" aria-hidden="true">
              ⇅
            </span>
            <select
              className="sort-select"
              aria-label="Сортировка"
              value={paging.sort}
              disabled={loading}
              onChange={(event) => paging.onSortChange(event.target.value as AnekdotSort)}
            >
              <option value="created">По дате создания</option>
              <option value="likes">По лайкам</option>
              <option value="favorites">По избранному</option>
            </select>
          </label>
        </div>
      )}

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
                      <span className="like-count">{item.favorite_count}</span>
                    </button>
                  </div>
                </div>
              </li>
            );
          })}
        </ul>
      )}

      {paging !== undefined && (
        <div className="list-footer">
          {totalPages > 1 && (
            <nav className="pagination" aria-label="Постраничная навигация">
              <button
                type="button"
                className="page-button"
                disabled={loading || paging.page <= 1}
                onClick={() => paging.onPageChange(paging.page - 1)}
              >
                Назад
              </button>
              {pages.map((number) => (
                <button
                  key={number}
                  type="button"
                  className={number === paging.page ? "page-button page-button-active" : "page-button"}
                  aria-current={number === paging.page ? "page" : undefined}
                  disabled={loading}
                  onClick={() => paging.onPageChange(number)}
                >
                  {number}
                </button>
              ))}
              <button
                type="button"
                className="page-button"
                disabled={loading || paging.page >= totalPages}
                onClick={() => paging.onPageChange(paging.page + 1)}
              >
                Вперёд
              </button>
              <span className="pagination-status muted">
                Страница {paging.page} из {totalPages}
              </span>
            </nav>
          )}
          <label className="sort-control paging-size" title="Количество на странице">
            <span className="sort-icon" aria-hidden="true">
              На странице
            </span>
            <select
              className="sort-select"
              aria-label="Количество на странице"
              value={paging.pageSize}
              disabled={loading}
              onChange={(event) => paging.onPageSizeChange(Number(event.target.value))}
            >
              {paging.pageSizeOptions.map((size) => (
                <option key={size} value={size}>
                  {size}
                </option>
              ))}
            </select>
          </label>
        </div>
      )}
    </section>
  );
}
