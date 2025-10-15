-- 001_schema.sql — простая пересборка каталога

-- На всякий случай отключаем проверки FK (если когда-то появятся внешние ключи)
SET FOREIGN_KEY_CHECKS = 0;

-- Удаляем таблицу, если есть
DROP TABLE IF EXISTS products;

-- Включаем проверки FK обратно
SET FOREIGN_KEY_CHECKS = 1;

-- Создаём таблицу с нужными полями (без индексов)
CREATE TABLE products (
                          id          INT AUTO_INCREMENT PRIMARY KEY,
                          name        VARCHAR(255) NOT NULL,
                          article     VARCHAR(100) NOT NULL,
                          price       DECIMAL(10,2) NOT NULL,
                          image_alt   VARCHAR(255),
                          created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Заполняем заново (одним INSERT с несколькими значениями)
INSERT INTO products (name, article, price, image_alt) VALUES
('Смартфон XYZ Pro',        'ART-001', 299.99, 'Смартфон с 128GB'),
( 'Ноутбук ABC Ultra',       'ART-002', 899.00, 'Ноутбук 16" i7'),
( 'Планшет DEF Mini',        'ART-003', 199.50, 'Планшет 10"'),
('Наушники GHI Wireless',   'ART-004',  79.90, 'Беспроводные TWS'),
( 'Клавиатура KLM Mechanical','ART-005',129.00, NULL);



