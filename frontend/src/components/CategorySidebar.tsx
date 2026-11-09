import { useMemo, useState } from "react";

import type { Category, CategorySort } from "../types";
import { IconChevronsUpDown } from "./Icons";

interface Props {
  categories: Category[];
  selectedCategory: number | null;
  onSelect: (categoryId: number | null) => void;
}

const sortOptions: ReadonlyArray<{ id: CategorySort; label: string }> = [
  { id: "name", label: "По алфавиту" },
  { id: "count", label: "По количеству" },
  { id: "popularity", label: "По популярности" },
];

function sortCategories(categories: Category[], sort: CategorySort): Category[] {
  const sorted = [...categories];

  switch (sort) {
    case "count":
      sorted.sort(
        (left, right) => right.anekdot_count - left.anekdot_count || left.name.localeCompare(right.name, "ru"),
      );
      break;
    case "popularity":
      sorted.sort((left, right) => right.popularity - left.popularity || left.name.localeCompare(right.name, "ru"));
      break;
    default:
      sorted.sort((left, right) => left.name.localeCompare(right.name, "ru"));
  }

  return sorted;
}

export function CategorySidebar({ categories, selectedCategory, onSelect }: Props) {
  const [sort, setSort] = useState<CategorySort>("name");

  const total = categories.reduce((sum, category) => sum + category.anekdot_count, 0);
  const sorted = useMemo(() => sortCategories(categories, sort), [categories, sort]);

  return (
    <aside className="sidebar">
      <div className="sidebar-header">
        <h2 className="sidebar-title">Категории</h2>
        <label className="sort-control" title="Сортировка категорий">
          <span className="sort-icon" aria-hidden="true">
            <IconChevronsUpDown />
          </span>
          <select
            className="sort-select"
            aria-label="Сортировка категорий"
            value={sort}
            onChange={(event) => {
              const value = event.target.value;
              setSort(value === "count" || value === "popularity" ? value : "name");
            }}
          >
            {sortOptions.map((option) => (
              <option key={option.id} value={option.id}>
                {option.label}
              </option>
            ))}
          </select>
        </label>
      </div>
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
        {sorted.map((category) => (
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
