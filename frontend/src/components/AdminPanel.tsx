import { useState, type FormEvent } from "react";

import { api, errorMessage } from "../api";
import type { Feedback } from "../types";

const statusLabels: Record<Feedback["status"], string> = {
  new: "Новое",
  read: "Прочитано",
  archived: "В архиве",
};

export function AdminPanel() {
  const [token, setToken] = useState("");
  const [messages, setMessages] = useState<Feedback[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleLoad(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (token.trim() === "") {
      setError("Введите токен администратора.");
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const payload = await api.listFeedback(token.trim());
      setMessages(payload.feedback);
    } catch (loadError) {
      setMessages(null);
      setError(errorMessage(loadError));
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="panel" aria-labelledby="admin-heading">
      <div className="panel-heading">
        <div>
          <h2 id="admin-heading">Сообщения администрации</h2>
          <p className="muted">Токен хранится только в памяти вкладки и не сохраняется на сервере.</p>
        </div>
      </div>

      <form className="form form-inline" onSubmit={handleLoad}>
        <label className="field">
          <span className="field-label">Токен администратора</span>
          <input
            className="input"
            type="password"
            value={token}
            autoComplete="off"
            placeholder="ADMIN_TOKEN"
            onChange={(event) => setToken(event.target.value)}
          />
        </label>
        <button type="submit" className="button button-primary" disabled={loading}>
          {loading ? "Загружаем…" : "Показать сообщения"}
        </button>
      </form>

      {error !== null && (
        <p className="alert" role="alert">
          {error}
        </p>
      )}

      {messages !== null && messages.length === 0 && <p className="empty-state">Сообщений пока нет.</p>}

      {messages !== null && messages.length > 0 && (
        <ul className="feedback-list">
          {messages.map((message) => (
            <li key={message.id} className="feedback-item">
              <header className="feedback-meta">
                <span className={`badge badge-${message.status}`}>{statusLabels[message.status]}</span>
                <time dateTime={message.created_at}>{new Date(message.created_at).toLocaleString("ru-RU")}</time>
              </header>
              <p className="feedback-text">{message.message}</p>
              {message.contact !== null && <p className="muted">Контакт: {message.contact}</p>}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
