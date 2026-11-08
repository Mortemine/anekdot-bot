import { useCallback, useEffect, useState } from "react";

import { api, errorMessage } from "../api";
import type { ExternalAnekdot } from "../types";

export function ExternalPanel() {
  const [anekdot, setAnekdot] = useState<ExternalAnekdot | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);

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
            <span className="muted">{anekdot.source}</span>
          </footer>
        </article>
      )}

      {anekdot === null && !loading && error === null && (
        <p className="empty-state">Нажмите «Ещё один», чтобы получить анекдот.</p>
      )}
    </section>
  );
}
