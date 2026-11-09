import type { CollectionSection } from "../types";

interface Props {
  section: CollectionSection;
  mineCount: number;
  favoritesCount: number;
  publicCount: number;
  onSelect: (section: CollectionSection) => void;
}

const sections: ReadonlyArray<{ id: CollectionSection; label: string }> = [
  { id: "mine", label: "Мои коллекции" },
  { id: "favorites", label: "Избранные" },
  { id: "public", label: "Публичные" },
];

export function CollectionsSidebar({ section, mineCount, favoritesCount, publicCount, onSelect }: Props) {
  const counts: Record<CollectionSection, number> = {
    mine: mineCount,
    favorites: favoritesCount,
    public: publicCount,
  };

  return (
    <aside className="sidebar">
      <h2 className="sidebar-title">Коллекции</h2>
      <div className="sidebar-list">
        {sections.map((item) => (
          <button
            key={item.id}
            type="button"
            className={
              item.id === section ? "category-button category-button-active" : "category-button"
            }
            aria-current={item.id === section ? "true" : undefined}
            onClick={() => onSelect(item.id)}
          >
            <span className="category-button-name">{item.label}</span>
            <span className="category-button-count">{counts[item.id]}</span>
          </button>
        ))}
      </div>
    </aside>
  );
}
