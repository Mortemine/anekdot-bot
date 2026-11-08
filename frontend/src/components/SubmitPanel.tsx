import { useCallback, useEffect, useState } from "react";
import type { FormEvent } from "react";

import { api, errorMessage } from "../api";
import type { Anekdot, Category, User } from "../types";

interface SubmitPanelProps {
  categories: Category[];
  user: User | null;
}

export function SubmitPanel({ categories, user }: SubmitPanelProps) {
  const [categoryId, setCategoryId] = useState(0);
  const [text, setText] = useState("");
  const [mine, setMine] = useState<Anekdot[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const loadMine = useCallback(async () => {
    try {
      const payload = await api.myAnekdots();
      setMine(payload.anekdots);
    } catch (loadError) {
      setError(errorMessage(loadError));
    }
  }, []);

  useEffect(() => {
    if (user === null) {
      setMine([]);
      return;
    }

    void loadMine();
  }, [user, loadMine]);

  if (user === null) {
    return (
      <section className="panel" aria-labelledby="submit-heading">
        <div className="panel-heading">
          <div>
            <h2 id="submit-heading">Свой анекдот</h2>
            <p className="muted">Публиковать анекдоты могут только зарегистрированные читатели.</p>
          </div>
        </div>
        <p className="empty-state">Откройте раздел «Аккаунт», войдите или зарегистрируйтесь — и возвращайтесь.</p>
      </section>
    );
  }

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setBusy(true);
    setError(null);
    setNotice(null);

    try {
      const payload = await api.createAnekdot(categoryId, text.trim());
      setText("");
      setNotice(`Анекдот №${payload.anekdot.id} опубликован.`);
      await loadMine();
    } catch (submitError) {
      setError(errorMessage(submitError));
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="panel" aria-labelledby="submit-heading">
      <div className="panel-heading">
        <div>
          <h2 id="submit-heading">Свой анекдот</h2>
          <p className="muted">Анекдот появится в общей ленте сразу после отправки.</p>
        </div>
      </div>

      {error !== null && (
        <p className="alert" role="alert">
          {error}
        </p>
      )}

      {notice !== null && (
        <p className="alert alert-success" role="status">
          {notice}
        </p>
      )}

      <form className="form" onSubmit={submit}>
        <label className="field">
          <span className="field-label">Категория</span>
          <select
            className="input"
            name="category"
            value={categoryId}
            onChange={(event) => setCategoryId(Number(event.target.value))}
          >
            <option value={0}>Вне категорий</option>
            {categories.map((category) => (
              <option key={category.id} value={category.id}>
                {category.name}
              </option>
            ))}
          </select>
        </label>

        <label className="field">
          <span className="field-label">Текст анекдота</span>
          <textarea
            className="input textarea"
            name="text"
            required
            minLength={10}
            maxLength={4000}
            value={text}
            onChange={(event) => setText(event.target.value)}
            placeholder="От 10 до 4000 символов"
          />
          <span className="field-hint">Осталось символов: {4000 - text.length}</span>
        </label>

        <button type="submit" className="button button-primary" disabled={busy || text.trim().length < 10}>
          {busy ? "Отправляем…" : "Опубликовать"}
        </button>
      </form>

      <h3 className="section-title">Мои анекдоты</h3>

      {mine.length === 0 ? (
        <p className="empty-state">Вы пока ничего не опубликовали.</p>
      ) : (
        <ul className="anekdot-list">
          {mine.map((anekdot) => (
            <li key={anekdot.id} className="anekdot-item">
              <p className="anekdot-item-text">{anekdot.text}</p>
              <footer className="anekdot-meta">
                <span className="badge">{anekdot.category_name}</span>
                <span className="muted">#{anekdot.id}</span>
              </footer>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
