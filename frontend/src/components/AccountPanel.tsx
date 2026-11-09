import { useState } from "react";
import type { FormEvent } from "react";

import { api, ApiError, errorMessage } from "../api";
import type { AuthResult, User } from "../types";

type Mode = "login" | "register";

interface AccountPanelProps {
  user: User | null;
  onAuthenticated: (result: AuthResult) => void;
  onLogout: () => void;
}

export function AccountPanel({ user, onAuthenticated, onLogout }: AccountPanelProps) {
  const [mode, setMode] = useState<Mode>("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (user !== null) {
    return (
      <section className="panel" aria-labelledby="account-heading">
        <div className="panel-heading">
          <div>
            <h2 id="account-heading">Аккаунт</h2>
            <p className="muted">Вы вошли как {user.display_name}.</p>
          </div>
          <button type="button" className="button" onClick={onLogout}>
            Выйти
          </button>
        </div>

        <dl className="profile">
          <div className="profile-row">
            <dt>Почта</dt>
            <dd>{user.email}</dd>
          </div>
          <div className="profile-row">
            <dt>Имя</dt>
            <dd>{user.display_name}</dd>
          </div>
          <div className="profile-row">
            <dt>С нами с</dt>
            <dd>{new Date(user.created_at).toLocaleDateString("ru-RU")}</dd>
          </div>
        </dl>

        <p className="muted">Понравившиеся и избранные анекдоты доступны в соответствующих разделах.</p>
      </section>
    );
  }

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setBusy(true);
    setError(null);

    try {
      const result =
        mode === "login"
          ? await api.login(email.trim(), password)
          : await api.register(email.trim(), password, displayName.trim());

      onAuthenticated(result);
      setPassword("");
      setDisplayName("");
    } catch (authError) {
      if (authError instanceof ApiError && authError.status === 401) {
        setError("Неверная почта или пароль.");
      } else {
        setError(errorMessage(authError));
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="panel" aria-labelledby="account-heading">
      <div className="panel-heading">
        <div>
          <h2 id="account-heading">Аккаунт</h2>
          <p className="muted">
            Войдите, чтобы ставить лайки и публиковать свои анекдоты.
          </p>
        </div>
      </div>

      <div className="segmented" role="group" aria-label="Вход или регистрация">
        <button
          type="button"
          className={mode === "login" ? "chip chip-active" : "chip"}
          aria-pressed={mode === "login"}
          onClick={() => setMode("login")}
        >
          Вход
        </button>
        <button
          type="button"
          className={mode === "register" ? "chip chip-active" : "chip"}
          aria-pressed={mode === "register"}
          onClick={() => setMode("register")}
        >
          Регистрация
        </button>
      </div>

      {error !== null && (
        <p className="alert" role="alert">
          {error}
        </p>
      )}

      <form className="form form-narrow" onSubmit={submit}>
        <label className="field">
          <span className="field-label">Почта</span>
          <input
            className="input"
            type="email"
            name="email"
            autoComplete="email"
            required
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            placeholder="reader@example.com"
          />
        </label>

        {mode === "register" && (
          <label className="field">
            <span className="field-label">Как к вам обращаться</span>
            <input
              className="input"
              type="text"
              name="displayName"
              autoComplete="nickname"
              value={displayName}
              onChange={(event) => setDisplayName(event.target.value)}
              placeholder="Необязательно"
            />
            <span className="field-hint">Если оставить пустым, возьмём часть адреса до собаки.</span>
          </label>
        )}

        <label className="field">
          <span className="field-label">Пароль</span>
          <input
            className="input"
            type="password"
            name="password"
            autoComplete={mode === "login" ? "current-password" : "new-password"}
            required
            minLength={mode === "register" ? 8 : undefined}
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            placeholder="Минимум 8 символов"
          />
        </label>

        <button type="submit" className="button button-primary" disabled={busy}>
          {busy ? "Секунду…" : mode === "login" ? "Войти" : "Зарегистрироваться"}
        </button>
      </form>
    </section>
  );
}
