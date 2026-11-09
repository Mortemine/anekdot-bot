import { useCallback, useEffect, useState } from "react";

import { api, errorMessage } from "../api";
import type { ExternalAnekdot, User } from "../types";

interface Props {
  user: User | null;
  onSaved: () => void;
}

export function ExternalPanel({ user, onSaved }: Props) {
  const [anekdot, setAnekdot] = useState<ExternalAnekdot | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    setSaveError(null);
    setSaved(false);

    try {
      const payload = await api.externalAnekdot();
      setAnekdot(payload.anekdot);
    } catch (loadError) {
      setError(errorMessage(loadError));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const save = useCallback(async () => {
    if (anekdot === null || user === null || saving || saved) {
      return;
    }

    setSaving(true);
    setSaveError(null);

    try {
      await api.saveExternalAnekdot(anekdot.text);
      setSaved(true);
      onSaved();
    } catch (saveFailure) {
      setSaveError(errorMessage(saveFailure));
    } finally {
      setSaving(false);
    }
  }, [anekdot, onSaved, saved, saving, user]);

  const saveTitle =
    user === null
      ? "Войдите, чтобы добавлять внешние анекдоты в понравившиеся"
      : saved
        ? "Анекдот уже в понравившихся"
        : "Добавить в понравившиеся";

  return (
    <section className="panel" aria-labelledby="external-heading">
      <div className="panel-heading">
        <div>
          <h2 id="external-heading">Случайный анекдот из интернета</h2>
          <p className="muted">Свежий анекдот с baneks.site — тот же источник, что был в Telegram-боте.</p>
        </div>
        <button type="button" className="button button-primary" onClick={load} disabled={loading}>
          {loading ? "Загружаем…" : "Ещё один"}
        </button>
      </div>

      {error !== null && (
        <p className="alert" role="alert">
          {error}
        </p>
      )}

      {anekdot !== null && (
        <article className="anekdot-card">
          <p className="anekdot-text">{anekdot.text}</p>
          <footer className="anekdot-meta">
            <span className="badge">Внешний источник</span>
            <div className="external-actions">
              <span className="muted">{anekdot.source}</span>
              <button
                type="button"
                className={saved ? "like-button like-button-active" : "like-button"}
                aria-pressed={saved}
                title={saveTitle}
                disabled={user === null || saving || saved}
                onClick={() => {
                  void save();
                }}
              >
                <span aria-hidden="true">{saved ? "♥" : "♡"}</span>
                <span>{saving ? "Сохраняем…" : "В понравившиеся"}</span>
              </button>
            </div>
          </footer>
        </article>
      )}

      {saveError !== null && (
        <p className="alert" role="alert">
          {saveError}
        </p>
      )}

      {saved && (
        <p className="alert alert-success">
          Анекдот сохранён с тегом «Внешние» и добавлен в понравившиеся.
        </p>
      )}

      {anekdot === null && !loading && error === null && (
        <p className="empty-state">Нажмите «Ещё один», чтобы получить анекдот.</p>
      )}
    </section>
  );
}
