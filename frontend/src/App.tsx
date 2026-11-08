import { useCallback, useEffect, useRef, useState } from "react";

import { api, errorMessage } from "./api";
import { AdminPanel } from "./components/AdminPanel";
import { AnekdotPanel } from "./components/AnekdotPanel";
import { ExternalPanel } from "./components/ExternalPanel";
import { FeedbackForm } from "./components/FeedbackForm";
import type { Anekdot, Category } from "./types";

type Tab = "anekdots" | "external" | "feedback" | "admin";

const tabs: ReadonlyArray<{ id: Tab; label: string }> = [
  { id: "anekdots", label: "Анекдоты" },
  { id: "external", label: "Случайный анекдот" },
  { id: "feedback", label: "Связь с администрацией" },
  { id: "admin", label: "Админка" },
];

export default function App() {
  const [tab, setTab] = useState<Tab>("anekdots");
  const [categories, setCategories] = useState<Category[]>([]);
  const [selectedCategory, setSelectedCategory] = useState<number | null>(null);
  const [anekdot, setAnekdot] = useState<Anekdot | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
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

  return (
    <div className="app">
      <header className="app-header">
        <div className="brand">
          <span className="brand-mark" aria-hidden="true">
            😂
          </span>
          <div>
            <h1>Анекдот Бот</h1>
            <p className="muted">Анекдоты по категориям, случайный анекдот и обратная связь</p>
          </div>
        </div>
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
            error={error}
            onSelectCategory={setSelectedCategory}
            onNext={() => {
              void loadAnekdot(selectedCategory);
            }}
          />
        )}
        {tab === "external" && <ExternalPanel />}
        {tab === "feedback" && <FeedbackForm />}
        {tab === "admin" && <AdminPanel />}
      </main>

      <footer className="app-footer">
        <p>Контент не проходит предварительную модерацию: анекдоты могут содержать ненормативную лексику.</p>
        <p className="muted">Архив Telegram-бота и случайные анекдоты с baneks.site.</p>
      </footer>
    </div>
  );
}
