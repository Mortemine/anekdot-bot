INSERT INTO categories (id, name)
VALUES (46, 'Внешние')
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;
