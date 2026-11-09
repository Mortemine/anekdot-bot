import { useCallback, useState } from "react";

import { api, errorMessage } from "../api";
import { formatDate } from "../format";
import type { Collection, CollectionDetail, CollectionSection, User } from "../types";

interface Props {
  user: User | null;
  section: CollectionSection;
  collections: Collection[];
  loading: boolean;
  error: string | null;
  publicCollections: Collection[];
  publicLoading: boolean;
  publicError: string | null;
  favoriteCollections: Collection[];
  onReload: () => void;
}

interface Draft {
  name: string;
  isPublic: boolean;
}

const headings: Record<CollectionSection, { title: string; description: string }> = {
  mine: {
    title: "Мои коллекции",
    description: "Собирайте анекдоты в тематические подборки и делитесь ими.",
  },
  favorites: {
    title: "Избранные коллекции",
    description: "Публичные коллекции, которые вы сохранили.",
  },
  public: {
    title: "Публичные коллекции",
    description: "Подборки других пользователей — открывайте их и добавляйте в избранное.",
  },
};

export function CollectionsPanel({
  user,
  section,
  collections,
  loading,
  error,
  publicCollections,
  publicLoading,
  publicError,
  favoriteCollections,
  onReload,
}: Props) {
  const [newName, setNewName] = useState("");
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [editingID, setEditingID] = useState<number | null>(null);
  const [draft, setDraft] = useState<Draft>({ name: "", isPublic: false });
  const [detail, setDetail] = useState<CollectionDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const refreshAll = useCallback(() => {
    onReload();
  }, [onReload]);

  const runAction = useCallback(async (action: () => Promise<string | null>) => {
    setBusy(true);
    setActionError(null);
    setNotice(null);

    try {
      const message = await action();
      setNotice(message);
    } catch (actionError) {
      setActionError(errorMessage(actionError));
    } finally {
      setBusy(false);
    }
  }, []);

  const handleCreate = useCallback(async () => {
    if (newName.trim() === "") {
      setActionError("Введите название коллекции.");

      return;
    }

    await runAction(async () => {
      const created = await api.createCollection(newName.trim());
      setNewName("");
      await refreshAll();

      return `Коллекция «${created.collection.name}» создана.`;
    });
  }, [newName, refreshAll, runAction]);

  const handleStartEdit = useCallback((collection: Collection) => {
    setEditingID(collection.id);
    setDraft({ name: collection.name, isPublic: collection.is_public });
    setActionError(null);
    setNotice(null);
  }, []);

  const handleSave = useCallback(
    async (collection: Collection) => {
      if (draft.name.trim() === "") {
        setActionError("Введите название коллекции.");

        return;
      }

      await runAction(async () => {
        await api.updateCollection(collection.id, draft.name.trim(), draft.isPublic);
        setEditingID(null);
        await refreshAll();

        return `Коллекция «${draft.name.trim()}» сохранена.`;
      });
    },
    [draft, refreshAll, runAction],
  );

  const handleToggleVisibility = useCallback(
    async (collection: Collection) => {
      await runAction(async () => {
        await api.updateCollection(collection.id, collection.name, !collection.is_public);
        await refreshAll();

        return collection.is_public
          ? `Коллекция «${collection.name}» теперь приватная.`
          : `Коллекция «${collection.name}» теперь публичная.`;
      });
    },
    [refreshAll, runAction],
  );

  const handleDelete = useCallback(
    async (collection: Collection) => {
      await runAction(async () => {
        await api.deleteCollection(collection.id);

        if (detail !== null && detail.collection.id === collection.id) {
          setDetail(null);
        }

        await refreshAll();

        return `Коллекция «${collection.name}» удалена.`;
      });
    },
    [detail, refreshAll, runAction],
  );

  const handleOpenDetail = useCallback(async (collection: Collection) => {
    setDetailLoading(true);
    setActionError(null);

    try {
      const payload = await api.getCollection(collection.id);
      setDetail(payload);
    } catch (detailError) {
      setActionError(errorMessage(detailError));
    } finally {
      setDetailLoading(false);
    }
  }, []);

  const handleToggleCollectionFavorite = useCallback(
    async (collection: Collection) => {
      await runAction(async () => {
        if (collection.favorited) {
          await api.unfavoriteCollection(collection.id);
        } else {
          await api.favoriteCollection(collection.id);
        }

        await refreshAll();

        return null;
      });
    },
    [refreshAll, runAction],
  );

  return (
    <section className="panel">
      <div className="panel-heading">
        <div>
          <h2>{headings[section].title}</h2>
          <p className="muted">{headings[section].description}</p>
        </div>
      </div>

      {actionError !== null && <p className="alert">{actionError}</p>}
      {notice !== null && <p className="alert alert-success">{notice}</p>}

      {section === "mine" &&
        (user === null ? (
          <p className="empty-state">Войдите в аккаунт, чтобы создавать свои коллекции и добавлять их в избранное.</p>
        ) : (
          <>
            <div className="collection-create">
              <input
                className="input"
                placeholder="Название новой коллекции"
                maxLength={100}
                value={newName}
                disabled={busy}
                onChange={(event) => setNewName(event.target.value)}
              />
              <button
                type="button"
                className="button button-primary"
                disabled={busy}
                onClick={() => {
                  void handleCreate();
                }}
              >
                Создать
              </button>
            </div>

            {error !== null && <p className="alert">{error}</p>}
            {loading && collections.length === 0 && <p className="empty-state">Загружаем коллекции…</p>}
            {!loading && error === null && collections.length === 0 && (
              <p className="empty-state">У вас пока нет коллекций. Создайте первую — это займёт секунду.</p>
            )}
            {collections.length > 0 && (
              <ul className="collection-list">
                {collections.map((collection) => (
                  <li key={collection.id} className="collection-item">
                    {editingID === collection.id ? (
                      <div className="collection-edit">
                        <input
                          className="input"
                          value={draft.name}
                          maxLength={100}
                          disabled={busy}
                          onChange={(event) => setDraft({ ...draft, name: event.target.value })}
                        />
                        <label className="collection-visibility">
                          <input
                            type="checkbox"
                            checked={draft.isPublic}
                            disabled={busy}
                            onChange={(event) => setDraft({ ...draft, isPublic: event.target.checked })}
                          />
                          <span>Публичная коллекция</span>
                        </label>
                        <div className="collection-actions">
                          <button
                            type="button"
                            className="button button-primary"
                            disabled={busy}
                            onClick={() => {
                              void handleSave(collection);
                            }}
                          >
                            Сохранить
                          </button>
                          <button type="button" className="button" disabled={busy} onClick={() => setEditingID(null)}>
                            Отмена
                          </button>
                        </div>
                      </div>
                    ) : (
                      <div className="collection-main">
                        <div className="collection-head">
                          <button
                            type="button"
                            className="collection-name"
                            disabled={detailLoading}
                            onClick={() => {
                              void handleOpenDetail(collection);
                            }}
                          >
                            {collection.name}
                          </button>
                          <span className={collection.is_public ? "badge badge-public" : "badge"}>
                            {collection.is_public ? "Публичная" : "Приватная"}
                          </span>
                        </div>
                        <p className="muted collection-meta">
                          Анекдотов: {collection.anekdot_count} · создана {formatDate(collection.created_at)}
                        </p>
                        <div className="collection-actions">
                          <button
                            type="button"
                            className="button"
                            disabled={busy}
                            onClick={() => handleStartEdit(collection)}
                          >
                            Редактировать
                          </button>
                          <button
                            type="button"
                            className="button"
                            disabled={busy}
                            onClick={() => {
                              void handleToggleVisibility(collection);
                            }}
                          >
                            {collection.is_public ? "Сделать приватной" : "Сделать публичной"}
                          </button>
                          <button
                            type="button"
                            className="button"
                            disabled={busy}
                            onClick={() => {
                              void handleDelete(collection);
                            }}
                          >
                            Удалить
                          </button>
                        </div>
                      </div>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </>
        ))}

      {section === "favorites" &&
        (user === null ? (
          <p className="empty-state">Войдите в аккаунт, чтобы добавлять коллекции в избранное.</p>
        ) : favoriteCollections.length === 0 ? (
          <p className="empty-state">Вы пока не добавили ни одной коллекции в избранное.</p>
        ) : (
          <ul className="collection-list">
            {favoriteCollections.map((collection) => (
              <li key={collection.id} className="collection-item">
                <div className="collection-main">
                  <div className="collection-head">
                    <button
                      type="button"
                      className="collection-name"
                      disabled={detailLoading}
                      onClick={() => {
                        void handleOpenDetail(collection);
                      }}
                    >
                      {collection.name}
                    </button>
                    <span className="badge badge-public">Публичная</span>
                  </div>
                  <p className="muted collection-meta">
                    Автор: {collection.owner_name} · анекдотов: {collection.anekdot_count}
                  </p>
                  <div className="collection-actions">
                    <button
                      type="button"
                      className="favorite-button favorite-button-active"
                      disabled={busy}
                      onClick={() => {
                        void handleToggleCollectionFavorite(collection);
                      }}
                    >
                      <span aria-hidden="true">★</span>
                      <span>В избранном</span>
                    </button>
                  </div>
                </div>
              </li>
            ))}
          </ul>
        ))}

      {section === "public" && (
        <>
          {publicError !== null && <p className="alert">{publicError}</p>}
          {publicLoading && publicCollections.length === 0 && (
            <p className="empty-state">Загружаем общие коллекции…</p>
          )}
          {!publicLoading && publicError === null && publicCollections.length === 0 && (
            <p className="empty-state">Публичных коллекций пока нет.</p>
          )}
          {publicCollections.length > 0 && (
            <ul className="collection-list">
              {publicCollections.map((collection) => {
                const favored = collection.favorited || favoriteCollections.some((item) => item.id === collection.id);
                const own = user !== null && collection.owner_id === user.id;

                return (
                  <li key={collection.id} className="collection-item">
                    <div className="collection-main">
                      <div className="collection-head">
                        <button
                          type="button"
                          className="collection-name"
                          disabled={detailLoading}
                          onClick={() => {
                            void handleOpenDetail(collection);
                          }}
                        >
                          {collection.name}
                        </button>
                        <span className="badge badge-public">Публичная</span>
                      </div>
                      <p className="muted collection-meta">
                        Автор: {own ? "вы" : collection.owner_name} · анекдотов: {collection.anekdot_count}
                      </p>
                      <div className="collection-actions">
                        <button
                          type="button"
                          className={favored ? "favorite-button favorite-button-active" : "favorite-button"}
                          aria-pressed={favored}
                          title={user === null ? "Войдите, чтобы добавлять коллекции в избранное" : "В избранное"}
                          disabled={busy || user === null}
                          onClick={() => {
                            void handleToggleCollectionFavorite({ ...collection, favorited: favored });
                          }}
                        >
                          <span aria-hidden="true">{favored ? "★" : "☆"}</span>
                          <span>{favored ? "В избранном" : "В избранное"}</span>
                        </button>
                      </div>
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
        </>
      )}

      {detail !== null && (
        <div className="collection-detail">
          <div className="panel-heading">
            <div>
              <h3 className="section-title">{detail.collection.name}</h3>
              <p className="muted">
                Автор: {detail.collection.owner_name === "" ? "вы" : detail.collection.owner_name} · анекдотов:{" "}
                {detail.anekdots.length}
              </p>
            </div>
            <button type="button" className="button" onClick={() => setDetail(null)}>
              Закрыть
            </button>
          </div>
          {detail.anekdots.length === 0 ? (
            <p className="empty-state">В этой коллекции пока нет анекдотов.</p>
          ) : (
            <ul className="anekdot-list">
              {detail.anekdots.map((anekdot) => (
                <li key={anekdot.id} className="anekdot-item">
                  <div className="anekdot-item-head">
                    <span className="badge">{anekdot.category_name}</span>
                    <span className="anekdot-item-date muted">{formatDate(anekdot.created_at)}</span>
                  </div>
                  <p className="anekdot-item-text">{anekdot.text}</p>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </section>
  );
}
