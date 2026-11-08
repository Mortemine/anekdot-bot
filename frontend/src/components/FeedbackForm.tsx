import { useState, type FormEvent } from "react";

import { api, errorMessage } from "../api";

type SubmitState = "idle" | "sending" | "sent";

export function FeedbackForm() {
  const [message, setMessage] = useState("");
  const [contact, setContact] = useState("");
  const [state, setState] = useState<SubmitState>("idle");
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (message.trim() === "") {
      setError("Введите текст сообщения.");
      return;
    }

    setState("sending");
    setError(null);

    try {
      await api.submitFeedback(message, contact);
      setState("sent");
      setMessage("");
      setContact("");
    } catch (submitError) {
      setError(errorMessage(submitError));
      setState("idle");
    }
  }

  return (
    <section className="panel" aria-labelledby="feedback-heading">
      <div className="panel-heading">
        <div>
          <h2 id="feedback-heading">Связь с администрацией</h2>
          <p className="muted">Сообщите об ошибке, предложите идею или пришлите анекдот.</p>
        </div>
      </div>

      {state === "sent" && (
        <p className="alert alert-success" role="status">
          Спасибо! Сообщение отправлено администрации.
        </p>
      )}

      {error !== null && (
        <p className="alert" role="alert">
          {error}
        </p>
      )}

      <form className="form" onSubmit={handleSubmit}>
        <label className="field">
          <span className="field-label">Сообщение</span>
          <textarea
            className="input textarea"
            value={message}
            maxLength={4000}
            required
            rows={6}
            placeholder="Что хотите сообщить?"
            onChange={(event) => setMessage(event.target.value)}
          />
          <span className="field-hint">{message.length} / 4000</span>
        </label>

        <label className="field">
          <span className="field-label">Контакт для ответа (необязательно)</span>
          <input
            className="input"
            value={contact}
            maxLength={200}
            placeholder="@username или email"
            onChange={(event) => setContact(event.target.value)}
          />
          <span className="field-hint">Оставьте пустым, если ответ не нужен.</span>
        </label>

        <button type="submit" className="button button-primary" disabled={state === "sending"}>
          {state === "sending" ? "Отправляем…" : "Отправить"}
        </button>
      </form>
    </section>
  );
}
