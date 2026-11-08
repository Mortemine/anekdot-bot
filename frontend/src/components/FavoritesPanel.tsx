import type { Anekdot, User } from "../types";

interface FavoritesPanelProps {
  user: User | null;
  favorites: Anekdot[];
  loading: boolean;
  error: string | null;
  onRemove: (anekdot: Anekdot) => void;
}

export function FavoritesPanel({ user, favorites, loading, error, onRemove }: FavoritesPanelProps) {
  if (user === null) {
    return (
      <section className="panel" aria-labelledby="favorites-heading">
        <div className="panel-heading">
          <div>
            <h2 id="favorites-heading">Избранное</h2>
            <p className="muted">Избранное хранится в аккаунте, поэтому нужен вход.</p>
          </div>
        </div>
        <p className="empty-state">Откройте раздел «Аккаунт» и войдите, чтобы сохранять анекдоты.</p>
      </section>
    );
  }

  return (
    <section className="panel" aria-labelledby="favorites-heading">
      <div className="panel-heading">
        <div>
          <h2 id="favorites-heading">Избранное</h2>
          <p className="muted">
            {favorites.length === 0
              ? "Пока пусто"
              : `${favorites.length.toLocaleString("ru-RU")} сохранённых анекдотов`}
          </p>
        </div>
      </div>

      {error !== null && (
        <p className="alert" role="alert">
          {error}
        </p>
      )}

      {loading && favorites.length === 0 && <p className="empty-state">Загружаем избранное…</p>}

      {!loading && favorites.length === 0 && error === null && (
        <p className="empty-state">
          Нажмите «В избранное» под анекдотом в разделе «Анекдоты» — он появится здесь.
        </p>
      )}

      {favorites.length > 0 && (
        <ul className="anekdot-list">
          {favorites.map((anekdot) => (
            <li key={anekdot.id} className="anekdot-item">
              <p className="anekdot-item-text">{anekdot.text}</p>
              <footer className="anekdot-meta">
                <span className="badge">{anekdot.category_name}</span>
                <span className="muted">#{anekdot.id}</span>
                <button type="button" className="button" onClick={() => onRemove(anekdot)}>
                  Убрать
                </button>
              </footer>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
