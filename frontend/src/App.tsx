import { useCallback, useEffect, useRef, useState } from "react";

import { api, errorMessage, getToken, setToken } from "./api";
import { AccountPanel } from "./components/AccountPanel";
import { AnekdotPanel } from "./components/AnekdotPanel";
import { ExternalPanel } from "./components/ExternalPanel";
import { FavoritesPanel } from "./components/FavoritesPanel";
import { SubmitPanel } from "./components/SubmitPanel";
import type { Anekdot, AuthResult, Category, User } from "./types";

type Tab = "anekdots" | "external" | "favorites" | "submit" | "account";

const tabs: ReadonlyArray<{ id: Tab; label: string }> = [
  { id: "anekdots", label: "Анекдоты" },
  { id: "external", label: "Случайный анекдот" },
  { id: "favorites", label: "Избранное" },
  { id: "submit", label: "Свой анекдот" },
  { id: "account", label: "Аккаунт" },
];

export default function App() {
  const [tab, setTab] = useState<Tab>("anekdots");
  const [categories, setCategories] = useState<Category[]>([]);
  const [selectedCategory, setSelectedCategory] = useState<number | null>(null);
  const [anekdot, setAnekdot] = useState<Anekdot | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [user, setUser] = useState<User | null>(null);
  const [favorites, setFavorites] = useState<Anekdot[]>([]);
  const [favoritesLoading, setFavoritesLoading] = useState(false);
  const [favoritesError, setFavoritesError] = useState<string | null>(null);
  const latestRequest = useRef(0);

  const loadAnekdot = useCallback(async (categoryId: number | null) => {
    latestRequest.current += 1;
    const requestId = latestRequest.current;

    setLoading(true);
    setError(null);

    try {
      const payload = await api.randomAnekdot(categoryId);
      if (requestId !== latestRequest.current) {
        return;
      }
      setAnekdot(payload.anekdot);
    } catch (loadError) {
      if (requestId !== latestRequest.current) {
        return;
      }
      setError(errorMessage(loadError));
    } finally {
      if (requestId === latestRequest.current) {
        setLoading(false);
      }
    }
  }, []);

  const loadFavorites = useCallback(async () => {
    setFavoritesLoading(true);
    setFavoritesError(null);

    try {
      const payload = await api.listFavorites();
      setFavorites(payload.anekdots);
    } catch (loadError) {
      setFavoritesError(errorMessage(loadError));
    } finally {
      setFavoritesLoading(false);
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
          setError(errorMessage(loadError));
        }
      });

    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    void loadAnekdot(selectedCategory);
  }, [loadAnekdot, selectedCategory]);

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
      setFavorites([]);
      return;
    }

    void loadFavorites();
  }, [user, loadFavorites]);

  const handleAuthenticated = useCallback((result: AuthResult) => {
    setToken(result.token);
    setUser(result.user);
    setTab("anekdots");
  }, []);

  const handleLogout = useCallback(async () => {
    try {
      await api.logout();
    } catch {
      // The session is dropped locally even if the server already forgot it.
    }

    setToken(null);
    setUser(null);
    setFavorites([]);
  }, []);

  const handleToggleFavorite = useCallback(async () => {
    if (user === null || anekdot === null) {
      return;
    }

    const saved = favorites.some((item) => item.id === anekdot.id);
    setFavoritesError(null);

    try {
      if (saved) {
        await api.removeFavorite(anekdot.id);
      } else {
        await api.addFavorite(anekdot.id);
      }
      await loadFavorites();
    } catch (favoriteError) {
      setFavoritesError(errorMessage(favoriteError));
    }
  }, [anekdot, favorites, loadFavorites, user]);

  const handleRemoveFavorite = useCallback(
    async (target: Anekdot) => {
      setFavoritesError(null);

      try {
        await api.removeFavorite(target.id);
        await loadFavorites();
      } catch (favoriteError) {
        setFavoritesError(errorMessage(favoriteError));
      }
    },
    [loadFavorites],
  );

  const isCurrentFavorite = anekdot !== null && favorites.some((item) => item.id === anekdot.id);

  return (
    <div className="app">
      <header className="app-header">
        <div className="brand">
          <span className="brand-mark" aria-hidden="true">
            😂
          </span>
          <div>
            <h1>Анекдот Бот</h1>
            <p className="muted">
              Анекдоты по категориям, случайный анекдот, избранное и свои анекдоты
            </p>
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

      <main className="app-main">
        {tab === "anekdots" && (
          <AnekdotPanel
            categories={categories}
            selectedCategory={selectedCategory}
            anekdot={anekdot}
            loading={loading}
            error={error ?? favoritesError}
            canFavorite={user !== null}
            isFavorite={isCurrentFavorite}
            onSelectCategory={setSelectedCategory}
            onNext={() => {
              void loadAnekdot(selectedCategory);
            }}
            onToggleFavorite={() => {
              void handleToggleFavorite();
            }}
          />
        )}
        {tab === "external" && <ExternalPanel />}
        {tab === "favorites" && (
          <FavoritesPanel
            user={user}
            favorites={favorites}
            loading={favoritesLoading}
            error={favoritesError}
            onRemove={(target) => {
              void handleRemoveFavorite(target);
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

      <footer className="app-footer">
        <p>Контент не проходит предварительную модерацию: анекдоты могут содержать ненормативную лексику.</p>
        <p className="muted">Архив Telegram-бота и случайные анекдоты с baneks.site.</p>
      </footer>
    </div>
  );
}
