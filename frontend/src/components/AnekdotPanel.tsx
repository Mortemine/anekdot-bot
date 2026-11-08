import type { Anekdot, Category } from "../types";

interface AnekdotPanelProps {
  categories: Category[];
  selectedCategory: number | null;
  anekdot: Anekdot | null;
  loading: boolean;
  error: string | null;
  onSelectCategory: (categoryId: number | null) => void;
  onNext: () => void;
}

export function AnekdotPanel({
  categories,
  selectedCategory,
  anekdot,
  loading,
  error,
  onSelectCategory,
  onNext,
}: AnekdotPanelProps) {
  const total = categories.reduce((sum, category) => sum + category.anekdot_count, 0);
  const activeCategory = categories.find((category) => category.id === selectedCategory);

  const subtitle =
    selectedCategory === null
      ? `Все категории · ${total.toLocaleString("ru-RU")} анекдотов`
      : `${activeCategory?.name ?? "Категория"} · ${(activeCategory?.anekdot_count ?? 0).toLocaleString("ru-RU")} анекдотов`;

  return (
    <section className="panel" aria-labelledby="anekdots-heading">
      <div className="panel-heading">
        <div>
          <h2 id="anekdots-heading">Анекдоты</h2>
          <p className="muted">{subtitle}</p>
        </div>
        <button type="button" className="button button-primary" onClick={onNext} disabled={loading}>
          {loading ? "Загружаем…" : "Следующий!"}
        </button>
      </div>

      <div className="category-list" role="group" aria-label="Категории анекдотов">
        <button
          type="button"
          className={selectedCategory === null ? "chip chip-active" : "chip"}
          aria-pressed={selectedCategory === null}
          onClick={() => onSelectCategory(null)}
        >
          Все
        </button>
        {categories.map((category) => (
          <button
            key={category.id}
            type="button"
            className={category.id === selectedCategory ? "chip chip-active" : "chip"}
            aria-pressed={category.id === selectedCategory}
            onClick={() => onSelectCategory(category.id)}
          >
            {category.name}
            <span className="chip-count">{category.anekdot_count}</span>
          </button>
        ))}
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
            <span className="badge">{anekdot.category_name}</span>
            <span className="muted">#{anekdot.id}</span>
          </footer>
        </article>
      )}

      {anekdot === null && !loading && error === null && (
        <p className="empty-state">Выберите категорию и нажмите «Следующий!».</p>
      )}

      {loading && anekdot === null && <p className="empty-state">Подбираем анекдот…</p>}
    </section>
  );
}
