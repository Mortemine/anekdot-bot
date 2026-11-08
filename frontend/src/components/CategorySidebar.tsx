import type { Category } from "../types";

interface Props {
  categories: Category[];
  selectedCategory: number | null;
  onSelect: (categoryId: number | null) => void;
}

export function CategorySidebar({ categories, selectedCategory, onSelect }: Props) {
  const total = categories.reduce((sum, category) => sum + category.anekdot_count, 0);

  return (
    <aside className="sidebar">
      <h2 className="sidebar-title">Категории</h2>
      <div className="sidebar-list">
        <button
          type="button"
          className={selectedCategory === null ? "category-button category-button-active" : "category-button"}
          aria-current={selectedCategory === null ? "true" : undefined}
          onClick={() => onSelect(null)}
        >
          <span className="category-button-name">Все анекдоты</span>
          <span className="category-button-count">{total}</span>
        </button>
        {categories.map((category) => (
          <button
            key={category.id}
            type="button"
            className={
              category.id === selectedCategory ? "category-button category-button-active" : "category-button"
            }
            aria-current={category.id === selectedCategory ? "true" : undefined}
            onClick={() => onSelect(category.id)}
          >
            <span className="category-button-name">{category.name}</span>
            <span className="category-button-count">{category.anekdot_count}</span>
          </button>
        ))}
      </div>
    </aside>
  );
}
