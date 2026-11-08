import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { api, errorMessage, getToken, setToken } from "./api";
import { AccountPanel } from "./components/AccountPanel";
import { AnekdotListPanel } from "./components/AnekdotListPanel";
import { CategorySidebar } from "./components/CategorySidebar";
import { CollectionsPanel } from "./components/CollectionsPanel";
import { ExternalPanel } from "./components/ExternalPanel";
import { SavedPanel } from "./components/SavedPanel";
import { SubmitPanel } from "./components/SubmitPanel";
import type { Anekdot, AnekdotSort, AuthResult, Category, Collection, User } from "./types";

type Tab = "anekdots" | "external" | "likes" | "favorites" | "collections" | "submit" | "account";

const tabs: ReadonlyArray<{ id: Tab; label: string }> = [
  { id: "anekdots", label: "Анекдоты" },
  { id: "external", label: "Случайный анекдот" },
  { id: "likes", label: "Понравившиеся" },
  { id: "favorites", label: "Избранное" },
  { id: "collections", label: "Коллекции" },
  { id: "submit", label: "Свой анекдот" },
  { id: "account", label: "Аккаунт" },
];

const pageSizes = [10, 20, 40, 100] as const;
const defaultPageSize = 10;

export default function App() {
  const [tab, setTab] = useState<Tab>("anekdots");
  const [categories, setCategories] = useState<Category[]>([]);
  const [selectedCategory, setSelectedCategory] = useState<number | null>(null);
  const [anekdots, setAnekdots] = useState<Anekdot[]>([]);
  const [anekdotsTotal, setAnekdotsTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState<number>(defaultPageSize);
  const [sort, setSort] = useState<AnekdotSort>("created");
  const [anekdotsLoading, setAnekdotsLoading] = useState(true);
  const [anekdotsError, setAnekdotsError] = useState<string | null>(null);
  const [likes, setLikes] = useState<Anekdot[]>([]);
  const [likesLoading, setLikesLoading] = useState(false);
  const [likesError, setLikesError] = useState<string | null>(null);
  const [favorites, setFavorites] = useState<Anekdot[]>([]);
  const [favoritesLoading, setFavoritesLoading] = useState(false);
  const [favoritesError, setFavoritesError] = useState<string | null>(null);
  const [user, setUser] = useState<User | null>(null);
  const [collections, setCollections] = useState<Collection[]>([]);
  const [collectionsLoading, setCollectionsLoading] = useState(false);
  const [collectionsError, setCollectionsError] = useState<string | null>(null);
  const [busyID, setBusyID] = useState<number | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionNotice, setActionNotice] = useState<string | null>(null);
  const latestAnekdots = useRef(0);
  const latestLikes = useRef(0);
  const latestFavorites = useRef(0);
  const latestCollections = useRef(0);

  const loadAnekdots = useCallback(
    async (categoryId: number | null, pageNumber: number, size: number, order: AnekdotSort) => {
      latestAnekdots.current += 1;
      const requestId = latestAnekdots.current;

      setAnekdotsLoading(true);
      setAnekdotsError(null);

      try {
        const payload = await api.listAnekdots(categoryId, {
          limit: size,
          offset: (pageNumber - 1) * size,
          sort: order,
        });
        if (requestId !== latestAnekdots.current) {
          return;
        }

        setAnekdots(payload.anekdots);
        setAnekdotsTotal(payload.total);

        const lastPage = Math.max(1, Math.ceil(payload.total / size));
        if (pageNumber > lastPage) {
          setPage(lastPage);
        }
      } catch (loadError) {
        if (requestId !== latestAnekdots.current) {
          return;
        }

        setAnekdotsError(errorMessage(loadError));
      } finally {
        if (requestId === latestAnekdots.current) {
          setAnekdotsLoading(false);
        }
      }
    },
    [],
  );

  const reloadAnecdots = useCallback(
    () => loadAnekdots(selectedCategory, page, pageSize, sort),
    [loadAnekdots, page, pageSize, selectedCategory, sort],
  );

  const loadLikes = useCallback(async () => {
    latestLikes.current += 1;
    const requestId = latestLikes.current;

    setLikesLoading(true);
    setLikesError(null);

    try {
      const payload = await api.listLikes();
      if (requestId !== latestLikes.current) {
        return;
      }

      setLikes(payload.anekdots);
    } catch (loadError) {
      if (requestId !== latestLikes.current) {
        return;
      }

      setLikesError(errorMessage(loadError));
    } finally {
      if (requestId === latestLikes.current) {
        setLikesLoading(false);
      }
    }
  }, []);

  const loadFavorites = useCallback(async () => {
    latestFavorites.current += 1;
    const requestId = latestFavorites.current;

    setFavoritesLoading(true);
    setFavoritesError(null);

    try {
      const payload = await api.listFavorites();
      if (requestId !== latestFavorites.current) {
        return;
      }

      setFavorites(payload.anekdots);
    } catch (loadError) {
      if (requestId !== latestFavorites.current) {
        return;
      }

      setFavoritesError(errorMessage(loadError));
    } finally {
      if (requestId === latestFavorites.current) {
        setFavoritesLoading(false);
      }
    }
  }, []);

  const loadCollections = useCallback(async () => {
    latestCollections.current += 1;
    const requestId = latestCollections.current;

    setCollectionsLoading(true);
    setCollectionsError(null);

    try {
      const payload = await api.listCollections();
      if (requestId !== latestCollections.current) {
        return;
      }

      setCollections(payload.collections);
    } catch (loadError) {
      if (requestId !== latestCollections.current) {
        return;
      }

      setCollectionsError(errorMessage(loadError));
    } finally {
      if (requestId === latestCollections.current) {
        setCollectionsLoading(false);
      }
    }
  }, []);

  useEffect(() => {
    let cancelled = false;

    api
      .listCategories()
      .then((payload) => {
        if (!cancelled) {
          setCategories(payload.categories);
        }
      })
      .catch((loadError: unknown) => {
        if (!cancelled) {
          setAnekdotsError(errorMessage(loadError));
        }
      });

    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    void loadAnekdots(selectedCategory, page, pageSize, sort);
  }, [loadAnekdots, page, pageSize, selectedCategory, sort]);

  useEffect(() => {
    if (getToken() === null) {
      return;
    }

    let cancelled = false;

    api
      .me()
      .then((payload) => {
        if (!cancelled) {
          setUser(payload.user);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setToken(null);
        }
      });

    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (user === null) {
      setLikes([]);
      setFavorites([]);
      setCollections([]);
      return;
    }

    void loadLikes();
    void loadFavorites();
    void loadCollections();
  }, [user, loadLikes, loadFavorites, loadCollections]);

  useEffect(() => {
    setActionError(null);
    setActionNotice(null);
  }, [tab]);

  const likedIDs = useMemo(() => new Set(likes.map((item) => item.id)), [likes]);
  const favoriteIDs = useMemo(() => new Set(favorites.map((item) => item.id)), [favorites]);

  const handleSelectCategory = useCallback((categoryId: number | null) => {
    setSelectedCategory(categoryId);
    setPage(1);
    setTab("anekdots");
  }, []);

  const handlePageChange = useCallback((nextPage: number) => {
    setPage(Math.max(1, nextPage));
  }, []);

  const handlePageSizeChange = useCallback((nextPageSize: number) => {
    setPageSize(nextPageSize);
    setPage(1);
  }, []);

  const handleSortChange = useCallback((nextSort: AnekdotSort) => {
    setSort(nextSort);
    setPage(1);
  }, []);

  const handleAuthenticated = useCallback((result: AuthResult) => {
    setToken(result.token);
    setUser(result.user);
    setTab("anekdots");
  }, []);

  const handleLogout = useCallback(async () => {
    await api.logout().catch(() => undefined);

    setToken(null);
    setUser(null);
    setLikes([]);
    setFavorites([]);
    setCollections([]);
    setActionError(null);
    setActionNotice(null);
  }, []);

  const handleAddToCollection = useCallback(
    async (target: Anekdot, collectionId: number) => {
      if (user === null) {
        return;
      }

      setBusyID(target.id);
      setActionError(null);
      setActionNotice(null);

      const collection = collections.find((item) => item.id === collectionId);

      try {
        await api.addToCollection(collectionId, target.id);
        setActionNotice(`Анекдот добавлен в коллекцию «${collection?.name ?? "коллекция"}».`);
        await loadCollections();
      } catch (addError) {
        setActionError(errorMessage(addError));
      } finally {
        setBusyID(null);
      }
    },
    [collections, loadCollections, user],
  );

  const handleToggleLike = useCallback(
    async (target: Anekdot) => {
      if (user === null) {
        return;
      }

      setBusyID(target.id);
      setActionError(null);

      try {
        if (likedIDs.has(target.id)) {
          await api.removeLike(target.id);
        } else {
          await api.addLike(target.id);
        }

        await Promise.all([loadLikes(), reloadAnecdots()]);
      } catch (likeError) {
        setActionError(errorMessage(likeError));
      } finally {
        setBusyID(null);
      }
    },
    [likedIDs, loadLikes, reloadAnecdots, user],
  );

  const handleToggleFavorite = useCallback(
    async (target: Anekdot) => {
      if (user === null) {
        return;
      }

      setBusyID(target.id);
      setActionError(null);

      try {
        if (favoriteIDs.has(target.id)) {
          await api.removeFavorite(target.id);
        } else {
          await api.addFavorite(target.id);
        }

        await Promise.all([loadFavorites(), reloadAnecdots()]);
      } catch (favoriteError) {
        setActionError(errorMessage(favoriteError));
      } finally {
        setBusyID(null);
      }
    },
    [favoriteIDs, loadFavorites, reloadAnecdots, user],
  );

  const selectedCategoryName = categories.find((category) => category.id === selectedCategory)?.name ?? "Все анекдоты";

  return (
    <div className="app">
      <header className="app-header">
        <div className="brand">
          <span className="brand-mark" aria-hidden="true">
            😂
          </span>
          <div>
            <h1>anekdotus</h1>
          </div>
        </div>
        <p className="muted session">{user === null ? "Гость" : `${user.display_name} · ${user.email}`}</p>
      </header>

      <nav className="tabs" aria-label="Разделы">
        {tabs.map((item) => (
          <button
            key={item.id}
            type="button"
            className={item.id === tab ? "tab tab-active" : "tab"}
            aria-current={item.id === tab ? "page" : undefined}
            onClick={() => setTab(item.id)}
          >
            {item.label}
          </button>
        ))}
      </nav>

      <div className="layout">
        <CategorySidebar
          categories={categories}
          selectedCategory={selectedCategory}
          onSelect={handleSelectCategory}
        />

        <main className="app-main">
          {tab === "anekdots" && (
            <AnekdotListPanel
              heading={selectedCategoryName}
              description={
                selectedCategory === null ? "Свежие анекдоты из архива" : `Анекдоты категории «${selectedCategoryName}»`
              }
              anekdots={anekdots}
              loading={anekdotsLoading}
              error={anekdotsError ?? actionError}
              likedIDs={likedIDs}
              favoriteIDs={favoriteIDs}
              canReact={user !== null}
              busyID={busyID}
              emptyText="В этой категории пока нет анекдотов."
              notice={actionNotice}
              collections={collections}
              onToggleLike={(target) => {
                void handleToggleLike(target);
              }}
              onToggleFavorite={(target) => {
                void handleToggleFavorite(target);
              }}
              onAddToCollection={(target, collectionId) => {
                void handleAddToCollection(target, collectionId);
              }}
              onReload={() => {
                void reloadAnecdots();
              }}
              paging={{
                page,
                pageSize,
                total: anekdotsTotal,
                sort,
                pageSizeOptions: pageSizes,
                onPageChange: handlePageChange,
                onPageSizeChange: handlePageSizeChange,
                onSortChange: handleSortChange,
              }}
            />
          )}
          {tab === "external" && <ExternalPanel />}
          {tab === "likes" && (
            <SavedPanel
              user={user}
              heading="Понравившиеся"
              description="Анекдоты, которым вы поставили лайк"
              guestText="Войдите в аккаунт, чтобы видеть понравившиеся анекдоты."
              emptyText="Вы пока не лайкнули ни одного анекдота."
              anekdots={likes}
              loading={likesLoading}
              error={likesError ?? actionError}
              likedIDs={likedIDs}
              favoriteIDs={favoriteIDs}
              busyID={busyID}
              notice={actionNotice}
              collections={collections}
              onToggleLike={(target) => {
                void handleToggleLike(target);
              }}
              onToggleFavorite={(target) => {
                void handleToggleFavorite(target);
              }}
              onAddToCollection={(target, collectionId) => {
                void handleAddToCollection(target, collectionId);
              }}
              onReload={() => {
                void loadLikes();
              }}
            />
          )}
          {tab === "favorites" && (
            <SavedPanel
              user={user}
              heading="Избранное"
              description="Анекдоты, которые вы сохранили отдельно от лайков"
              guestText="Войдите в аккаунт, чтобы видеть избранные анекдоты."
              emptyText="Вы пока ничего не добавили в избранное."
              anekdots={favorites}
              loading={favoritesLoading}
              error={favoritesError ?? actionError}
              likedIDs={likedIDs}
              favoriteIDs={favoriteIDs}
              busyID={busyID}
              notice={actionNotice}
              collections={collections}
              onToggleLike={(target) => {
                void handleToggleLike(target);
              }}
              onToggleFavorite={(target) => {
                void handleToggleFavorite(target);
              }}
              onAddToCollection={(target, collectionId) => {
                void handleAddToCollection(target, collectionId);
              }}
              onReload={() => {
                void loadFavorites();
              }}
            />
          )}
          {tab === "collections" && (
            <CollectionsPanel
              user={user}
              collections={collections}
              loading={collectionsLoading}
              error={collectionsError}
              onReload={() => {
                void loadCollections();
              }}
            />
          )}
          {tab === "submit" && <SubmitPanel categories={categories} user={user} />}
          {tab === "account" && (
            <AccountPanel
              user={user}
              onAuthenticated={handleAuthenticated}
              onLogout={() => {
                void handleLogout();
              }}
            />
          )}
        </main>
      </div>
    </div>
  );
}
