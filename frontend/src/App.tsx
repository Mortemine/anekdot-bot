import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { api, errorMessage, getToken, setToken } from "./api";
import { AccountPanel } from "./components/AccountPanel";
import { AnekdotListPanel } from "./components/AnekdotListPanel";
import { CategorySidebar } from "./components/CategorySidebar";
import { ExternalPanel } from "./components/ExternalPanel";
import { LikesPanel } from "./components/LikesPanel";
import { SubmitPanel } from "./components/SubmitPanel";
import type { Anekdot, AuthResult, Category, User } from "./types";

type Tab = "anekdots" | "external" | "likes" | "submit" | "account";

const tabs: ReadonlyArray<{ id: Tab; label: string }> = [
  { id: "anekdots", label: "Анекдоты" },
  { id: "external", label: "Случайный анекдот" },
  { id: "likes", label: "Лайки" },
  { id: "submit", label: "Свой анекдот" },
  { id: "account", label: "Аккаунт" },
];

const pageSize = 40;

export default function App() {
  const [tab, setTab] = useState<Tab>("anekdots");
  const [categories, setCategories] = useState<Category[]>([]);
  const [selectedCategory, setSelectedCategory] = useState<number | null>(null);
  const [anekdots, setAnekdots] = useState<Anekdot[]>([]);
  const [anekdotsLoading, setAnekdotsLoading] = useState(true);
  const [anekdotsError, setAnekdotsError] = useState<string | null>(null);
  const [likes, setLikes] = useState<Anekdot[]>([]);
  const [likesLoading, setLikesLoading] = useState(false);
  const [likesError, setLikesError] = useState<string | null>(null);
  const [user, setUser] = useState<User | null>(null);
  const [busyID, setBusyID] = useState<number | null>(null);
  const latestRequest = useRef(0);

  const loadAnekdots = useCallback(async (categoryId: number | null) => {
    latestRequest.current += 1;
    const requestId = latestRequest.current;

    setAnekdotsLoading(true);
    setAnekdotsError(null);

    try {
      const payload = await api.listAnekdots(categoryId, pageSize);
      if (requestId !== latestRequest.current) {
        return;
      }

      setAnekdots(payload.anekdots);
    } catch (loadError) {
      if (requestId !== latestRequest.current) {
        return;
      }

      setAnekdotsError(errorMessage(loadError));
    } finally {
      if (requestId === latestRequest.current) {
        setAnekdotsLoading(false);
      }
    }
  }, []);

  const loadLikes = useCallback(async () => {
    setLikesLoading(true);
    setLikesError(null);

    try {
      const payload = await api.listLikes();
      setLikes(payload.anekdots);
    } catch (loadError) {
      setLikesError(errorMessage(loadError));
    } finally {
      setLikesLoading(false);
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
    void loadAnekdots(selectedCategory);
  }, [loadAnekdots, selectedCategory]);

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
      return;
    }

    void loadLikes();
  }, [user, loadLikes]);

  const likedIDs = useMemo(() => new Set(likes.map((item) => item.id)), [likes]);

  const handleSelectCategory = useCallback((categoryId: number | null) => {
    setSelectedCategory(categoryId);
    setTab("anekdots");
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
  }, []);

  const handleToggleLike = useCallback(
    async (target: Anekdot) => {
      if (user === null) {
        return;
      }

      setBusyID(target.id);
      setLikesError(null);
      setAnekdotsError(null);

      try {
        if (likedIDs.has(target.id)) {
          await api.removeLike(target.id);
        } else {
          await api.addLike(target.id);
        }

        await Promise.all([loadLikes(), loadAnekdots(selectedCategory)]);
      } catch (likeError) {
        setLikesError(errorMessage(likeError));
      } finally {
        setBusyID(null);
      }
    },
    [likedIDs, loadAnekdots, loadLikes, selectedCategory, user],
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
            <h1>Анекдот Бот</h1>
            <p className="muted">Категории слева, анекдоты с лайками справа</p>
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
              error={anekdotsError ?? likesError}
              likedIDs={likedIDs}
              canLike={user !== null}
              busyID={busyID}
              emptyText="В этой категории пока нет анекдотов."
              onToggleLike={(target) => {
                void handleToggleLike(target);
              }}
              onReload={() => {
                void loadAnekdots(selectedCategory);
              }}
            />
          )}
          {tab === "external" && <ExternalPanel />}
          {tab === "likes" && (
            <LikesPanel
              user={user}
              likes={likes}
              loading={likesLoading}
              error={likesError}
              busyID={busyID}
              onToggleLike={(target) => {
                void handleToggleLike(target);
              }}
              onReload={() => {
                void loadLikes();
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

      <footer className="app-footer">
        <p>Контент не проходит предварительную модерацию: анекдоты могут содержать ненормативную лексику.</p>
        <p className="muted">Архив Telegram-бота и случайные анекдоты с baneks.site.</p>
      </footer>
    </div>
  );
}
