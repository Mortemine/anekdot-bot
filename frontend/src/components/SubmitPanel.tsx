import { useCallback, useEffect, useState } from "react";
import type { FormEvent } from "react";

import { api, errorMessage } from "../api";
import { formatDate } from "../format";
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
  const [formOpen, setFormOpen] = useState(false);

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
      setFormOpen(false);

      return;
    }

    void loadMine();
  }, [user, loadMine]);

  if (user === null) {
    return (
      <section className="panel" aria-labelledby="submit-heading">
        <div className="panel-heading">
          <div>
            <h2 id="submit-heading">Мои анекдоты</h2>
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
      setFormOpen(false);
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
          <h2 id="submit-heading">Мои анекдоты</h2>
          <p className="muted">Анекдоты, которые вы опубликовали.</p>
        </div>
        <button
          type="button"
          className="button button-primary"
          disabled={busy}
          aria-expanded={formOpen}
          onClick={() => {
            setFormOpen(!formOpen);
            setError(null);
            setNotice(null);
          }}
        >
          {formOpen ? "Скрыть форму" : "Добавить анекдот"}
        </button>
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

      {formOpen && (
        <form className="form" onSubmit={submit}>
          <label className="field">
            <span className="field-label">Категория</span>
            <select
              className="input"
              name="category"
              value={categoryId}
              onChange={(event) => setCategoryId(Number(event.target.value))}
            >
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

          <div className="collection-actions">
            <button type="submit" className="button button-primary" disabled={busy || text.trim().length < 10}>
              {busy ? "Отправляем…" : "Опубликовать"}
            </button>
            <button
              type="button"
              className="button"
              disabled={busy}
              onClick={() => {
                setFormOpen(false);
                setError(null);
              }}
            >
              Отмена
            </button>
          </div>
        </form>
      )}

      {mine.length === 0 ? (
        <p className="empty-state">Вы пока ничего не опубликовали.</p>
      ) : (
        <ul className="anekdot-list">
          {mine.map((anekdot) => (
            <li key={anekdot.id} className="anekdot-item">
              <div className="anekdot-item-head">
                <span className="badge">{anekdot.category_name}</span>
                <span className="anekdot-item-date muted">{formatDate(anekdot.created_at)}</span>
              </div>
              <p className="anekdot-item-text">{anekdot.text}</p>
              <span className="anekdot-id muted">#{anekdot.id}</span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
