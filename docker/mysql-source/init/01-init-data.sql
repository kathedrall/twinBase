-- Criar schema de teste no banco de origem
CREATE SCHEMA IF NOT EXISTS `ecommerce`;
CREATE SCHEMA IF NOT EXISTS `blog`;
CREATE SCHEMA IF NOT EXISTS `analytics`;

-- Usar o schema ecommerce
USE `ecommerce`;

-- Criar tabela de usuários
CREATE TABLE IF NOT EXISTS `users` (
    `id` INT AUTO_INCREMENT PRIMARY KEY,
    `name` VARCHAR(255) NOT NULL,
    `email` VARCHAR(255) UNIQUE NOT NULL,
    `password` VARCHAR(255) NOT NULL,
    `age` INT,
    `city` VARCHAR(100),
    `country` VARCHAR(100),
    `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX `idx_email` (`email`),
    INDEX `idx_city` (`city`),
    INDEX `idx_created_at` (`created_at`)
);

-- Criar tabela de categorias
CREATE TABLE IF NOT EXISTS `categories` (
    `id` INT AUTO_INCREMENT PRIMARY KEY,
    `name` VARCHAR(255) NOT NULL,
    `description` TEXT,
    `parent_id` INT NULL,
    `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY `uk_name` (`name`),
    INDEX `idx_parent_id` (`parent_id`)
);

-- Criar tabela de produtos
CREATE TABLE IF NOT EXISTS `products` (
    `id` INT AUTO_INCREMENT PRIMARY KEY,
    `name` VARCHAR(255) NOT NULL,
    `description` TEXT,
    `price` DECIMAL(10,2) NOT NULL,
    `category_id` INT,
    `stock` INT DEFAULT 0,
    `sku` VARCHAR(100),
    `weight` DECIMAL(8,2),
    `is_active` BOOLEAN DEFAULT TRUE,
    `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (`category_id`) REFERENCES `categories`(`id`) ON DELETE SET NULL,
    INDEX `idx_category_id` (`category_id`),
    INDEX `idx_price` (`price`),
    INDEX `idx_name` (`name`),
    INDEX `idx_sku` (`sku`),
    INDEX `idx_is_active` (`is_active`)
);

-- Criar tabela de pedidos
CREATE TABLE IF NOT EXISTS `orders` (
    `id` INT AUTO_INCREMENT PRIMARY KEY,
    `user_id` INT NOT NULL,
    `total` DECIMAL(10,2) NOT NULL,
    `status` ENUM('pending', 'processing', 'shipped', 'delivered', 'cancelled') DEFAULT 'pending',
    `shipping_address` TEXT,
    `order_date` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    `shipped_date` TIMESTAMP NULL,
    `delivered_date` TIMESTAMP NULL,
    `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE,
    INDEX `idx_user_id` (`user_id`),
    INDEX `idx_status` (`status`),
    INDEX `idx_order_date` (`order_date`)
);

-- Criar tabela de itens do pedido
CREATE TABLE IF NOT EXISTS `order_items` (
    `id` INT AUTO_INCREMENT PRIMARY KEY,
    `order_id` INT NOT NULL,
    `product_id` INT NOT NULL,
    `quantity` INT NOT NULL,
    `price` DECIMAL(10,2) NOT NULL,
    `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (`order_id`) REFERENCES `orders`(`id`) ON DELETE CASCADE,
    FOREIGN KEY (`product_id`) REFERENCES `products`(`id`) ON DELETE CASCADE,
    INDEX `idx_order_id` (`order_id`),
    INDEX `idx_product_id` (`product_id`)
);

-- Inserir categorias (50 categorias)
INSERT INTO `categories` (`name`, `description`, `parent_id`) VALUES
('Electronics', 'Electronic devices and gadgets', NULL),
('Books', 'Physical and digital books', NULL),
('Clothing', 'Fashion and apparel', NULL),
('Home & Garden', 'Home improvement and gardening supplies', NULL),
('Sports', 'Sports equipment and accessories', NULL),
('Automotive', 'Car accessories and parts', NULL),
('Beauty', 'Beauty and personal care products', NULL),
('Toys', 'Toys and games for all ages', NULL),
('Health', 'Health and wellness products', NULL),
('Food', 'Food and beverages', NULL),
('Smartphones', 'Mobile phones and accessories', 1),
('Laptops', 'Portable computers', 1),
('Tablets', 'Tablet computers', 1),
('Audio', 'Headphones and speakers', 1),
('Gaming', 'Gaming consoles and accessories', 1),
('Fiction', 'Fiction books', 2),
('Non-Fiction', 'Non-fiction books', 2),
('Textbooks', 'Educational books', 2),
('Comics', 'Comic books and graphic novels', 2),
('Mens Clothing', 'Clothing for men', 3),
('Womens Clothing', 'Clothing for women', 3),
('Kids Clothing', 'Clothing for children', 3),
('Shoes', 'Footwear for all', 3),
('Accessories', 'Fashion accessories', 3),
('Furniture', 'Home furniture', 4),
('Decor', 'Home decoration items', 4),
('Kitchen', 'Kitchen appliances and tools', 4),
('Bathroom', 'Bathroom accessories', 4),
('Garden Tools', 'Gardening equipment', 4),
('Soccer', 'Soccer equipment', 5),
('Basketball', 'Basketball equipment', 5),
('Tennis', 'Tennis equipment', 5),
('Swimming', 'Swimming accessories', 5),
('Fitness', 'Fitness equipment', 5),
('Car Parts', 'Automotive parts', 6),
('Car Electronics', 'Car electronic devices', 6),
('Tires', 'Car tires', 6),
('Makeup', 'Cosmetic products', 7),
('Skincare', 'Skin care products', 7),
('Hair Care', 'Hair care products', 7),
('Board Games', 'Traditional board games', 8),
('Video Games', 'Digital games', 8),
('Action Figures', 'Collectible figures', 8),
('Dolls', 'Dolls and accessories', 8),
('Vitamins', 'Health supplements', 9),
('Medical', 'Medical supplies', 9),
('Organic Food', 'Organic food products', 10),
('Beverages', 'Drinks and beverages', 10),
('Snacks', 'Snack foods', 10),
('Frozen Food', 'Frozen food items', 10);

-- Procedimento para gerar usuários em massa
DELIMITER $$

CREATE PROCEDURE GenerateUsers(IN num_users INT)
BEGIN
    DECLARE i INT DEFAULT 1;
    DECLARE user_name VARCHAR(255);
    DECLARE user_email VARCHAR(255);
    DECLARE user_age INT;
    DECLARE user_city VARCHAR(100);
    DECLARE user_country VARCHAR(100);
    
    WHILE i <= num_users DO
        SET user_name = CONCAT('User', LPAD(i, 6, '0'));
        SET user_email = CONCAT('user', i, '@email.com');
        SET user_age = FLOOR(18 + (RAND() * 65));
        SET user_city = ELT(FLOOR(1 + (RAND() * 20)), 
            'São Paulo', 'Rio de Janeiro', 'Belo Horizonte', 'Salvador', 'Brasília',
            'Fortaleza', 'Curitiba', 'Recife', 'Porto Alegre', 'Manaus',
            'Belém', 'Goiânia', 'Guarulhos', 'Campinas', 'São Luís',
            'São Gonçalo', 'Maceió', 'Duque de Caxias', 'Natal', 'Teresina');
        SET user_country = 'Brasil';
        
        INSERT INTO `users` (`name`, `email`, `password`, `age`, `city`, `country`) 
        VALUES (user_name, user_email, 'hash123456', user_age, user_city, user_country);
        
        SET i = i + 1;
    END WHILE;
END$$

CREATE PROCEDURE GenerateProducts(IN num_products INT)
BEGIN
    DECLARE i INT DEFAULT 1;
    DECLARE product_name VARCHAR(255);
    DECLARE product_description TEXT;
    DECLARE product_price DECIMAL(10,2);
    DECLARE product_category INT;
    DECLARE product_stock INT;
    DECLARE product_sku VARCHAR(100);
    DECLARE product_weight DECIMAL(8,2);
    
    WHILE i <= num_products DO
        SET product_name = CONCAT('Product ', LPAD(i, 6, '0'));
        SET product_description = CONCAT('High quality product number ', i, ' with amazing features and excellent performance.');
        SET product_price = ROUND(10 + (RAND() * 2000), 2);
        SET product_category = FLOOR(1 + (RAND() * 50));
        SET product_stock = FLOOR(0 + (RAND() * 1000));
        SET product_sku = CONCAT('SKU', LPAD(i, 8, '0'));
        SET product_weight = ROUND(0.1 + (RAND() * 50), 2);
        
        INSERT INTO `products` (`name`, `description`, `price`, `category_id`, `stock`, `sku`, `weight`) 
        VALUES (product_name, product_description, product_price, product_category, product_stock, product_sku, product_weight);
        
        SET i = i + 1;
    END WHILE;
END$$

CREATE PROCEDURE GenerateOrders(IN num_orders INT)
BEGIN
    DECLARE i INT DEFAULT 1;
    DECLARE order_user_id INT;
    DECLARE order_total DECIMAL(10,2);
    DECLARE order_status VARCHAR(20);
    DECLARE order_address TEXT;
    DECLARE max_user_id INT;
    
    SELECT MAX(id) INTO max_user_id FROM users;
    
    WHILE i <= num_orders DO
        SET order_user_id = FLOOR(1 + (RAND() * max_user_id));
        SET order_total = ROUND(50 + (RAND() * 2000), 2);
        SET order_status = ELT(FLOOR(1 + (RAND() * 5)), 'pending', 'processing', 'shipped', 'delivered', 'cancelled');
        SET order_address = CONCAT('Rua das Flores, ', FLOOR(1 + (RAND() * 999)), ', Bairro Central, CEP 12345-678');
        
        INSERT INTO `orders` (`user_id`, `total`, `status`, `shipping_address`) 
        VALUES (order_user_id, order_total, order_status, order_address);
        
        SET i = i + 1;
    END WHILE;
END$$

CREATE PROCEDURE GenerateOrderItems(IN num_items INT)
BEGIN
    DECLARE i INT DEFAULT 1;
    DECLARE item_order_id INT;
    DECLARE item_product_id INT;
    DECLARE item_quantity INT;
    DECLARE item_price DECIMAL(10,2);
    DECLARE max_order_id INT;
    DECLARE max_product_id INT;
    
    SELECT MAX(id) INTO max_order_id FROM orders;
    SELECT MAX(id) INTO max_product_id FROM products;
    
    WHILE i <= num_items DO
        SET item_order_id = FLOOR(1 + (RAND() * max_order_id));
        SET item_product_id = FLOOR(1 + (RAND() * max_product_id));
        SET item_quantity = FLOOR(1 + (RAND() * 10));
        SET item_price = ROUND(10 + (RAND() * 500), 2);
        
        INSERT IGNORE INTO `order_items` (`order_id`, `product_id`, `quantity`, `price`) 
        VALUES (item_order_id, item_product_id, item_quantity, item_price);
        
        SET i = i + 1;
    END WHILE;
END$$

DELIMITER ;

-- Gerar dados em massa
CALL GenerateUsers(50000);        -- 50.000 usuários
CALL GenerateProducts(25000);     -- 25.000 produtos  
CALL GenerateOrders(75000);       -- 75.000 pedidos
CALL GenerateOrderItems(200000);  -- 200.000 itens de pedidos

-- Remover procedimentos
DROP PROCEDURE GenerateUsers;
DROP PROCEDURE GenerateProducts;
DROP PROCEDURE GenerateOrders;
DROP PROCEDURE GenerateOrderItems;

-- Usar o schema blog
USE `blog`;

-- Criar tabela de autores
CREATE TABLE IF NOT EXISTS `authors` (
    `id` INT AUTO_INCREMENT PRIMARY KEY,
    `name` VARCHAR(255) NOT NULL,
    `email` VARCHAR(255) UNIQUE NOT NULL,
    `bio` TEXT,
    `website` VARCHAR(255),
    `social_media` JSON,
    `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX `idx_email` (`email`)
);

-- Criar tabela de posts
CREATE TABLE IF NOT EXISTS `posts` (
    `id` INT AUTO_INCREMENT PRIMARY KEY,
    `title` VARCHAR(500) NOT NULL,
    `slug` VARCHAR(500) NOT NULL,
    `content` LONGTEXT NOT NULL,
    `excerpt` TEXT,
    `author_id` INT NOT NULL,
    `published` BOOLEAN DEFAULT FALSE,
    `featured` BOOLEAN DEFAULT FALSE,
    `views` INT DEFAULT 0,
    `published_at` TIMESTAMP NULL,
    `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (`author_id`) REFERENCES `authors`(`id`) ON DELETE CASCADE,
    INDEX `idx_author_id` (`author_id`),
    INDEX `idx_published` (`published`),
    INDEX `idx_published_at` (`published_at`),
    INDEX `idx_slug` (`slug`),
    INDEX `idx_views` (`views`)
);

-- Criar tabela de comentários
CREATE TABLE IF NOT EXISTS `comments` (
    `id` INT AUTO_INCREMENT PRIMARY KEY,
    `post_id` INT NOT NULL,
    `author_name` VARCHAR(255) NOT NULL,
    `author_email` VARCHAR(255) NOT NULL,
    `content` TEXT NOT NULL,
    `approved` BOOLEAN DEFAULT FALSE,
    `ip_address` VARCHAR(45),
    `user_agent` TEXT,
    `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (`post_id`) REFERENCES `posts`(`id`) ON DELETE CASCADE,
    INDEX `idx_post_id` (`post_id`),
    INDEX `idx_approved` (`approved`),
    INDEX `idx_created_at` (`created_at`)
);

-- Criar tabela de tags
CREATE TABLE IF NOT EXISTS `tags` (
    `id` INT AUTO_INCREMENT PRIMARY KEY,
    `name` VARCHAR(100) NOT NULL,
    `slug` VARCHAR(100) NOT NULL,
    `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY `uk_name` (`name`),
    UNIQUE KEY `uk_slug` (`slug`)
);

-- Criar tabela de relacionamento post-tags
CREATE TABLE IF NOT EXISTS `post_tags` (
    `id` INT AUTO_INCREMENT PRIMARY KEY,
    `post_id` INT NOT NULL,
    `tag_id` INT NOT NULL,
    `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (`post_id`) REFERENCES `posts`(`id`) ON DELETE CASCADE,
    FOREIGN KEY (`tag_id`) REFERENCES `tags`(`id`) ON DELETE CASCADE,
    UNIQUE KEY `uk_post_tag` (`post_id`, `tag_id`),
    INDEX `idx_post_id` (`post_id`),
    INDEX `idx_tag_id` (`tag_id`)
);

-- Procedimentos para gerar dados do blog
DELIMITER $$

CREATE PROCEDURE GenerateAuthors(IN num_authors INT)
BEGIN
    DECLARE i INT DEFAULT 1;
    DECLARE author_name VARCHAR(255);
    DECLARE author_email VARCHAR(255);
    DECLARE author_bio TEXT;
    
    WHILE i <= num_authors DO
        SET author_name = CONCAT('Author ', LPAD(i, 4, '0'));
        SET author_email = CONCAT('author', i, '@blog.com');
        SET author_bio = CONCAT('Experienced writer specializing in various topics. Author ', i, ' has been writing for many years.');
        
        INSERT INTO `authors` (`name`, `email`, `bio`) 
        VALUES (author_name, author_email, author_bio);
        
        SET i = i + 1;
    END WHILE;
END$$

CREATE PROCEDURE GenerateTags(IN num_tags INT)
BEGIN
    DECLARE i INT DEFAULT 1;
    DECLARE tag_name VARCHAR(100);
    DECLARE tag_slug VARCHAR(100);
    
    WHILE i <= num_tags DO
        SET tag_name = CONCAT('Tag ', LPAD(i, 3, '0'));
        SET tag_slug = CONCAT('tag-', LPAD(i, 3, '0'));
        
        INSERT INTO `tags` (`name`, `slug`) 
        VALUES (tag_name, tag_slug);
        
        SET i = i + 1;
    END WHILE;
END$$

CREATE PROCEDURE GeneratePosts(IN num_posts INT)
BEGIN
    DECLARE i INT DEFAULT 1;
    DECLARE post_title VARCHAR(500);
    DECLARE post_slug VARCHAR(500);
    DECLARE post_content LONGTEXT;
    DECLARE post_excerpt TEXT;
    DECLARE post_author_id INT;
    DECLARE post_published BOOLEAN;
    DECLARE post_views INT;
    DECLARE max_author_id INT;
    
    SELECT MAX(id) INTO max_author_id FROM authors;
    
    WHILE i <= num_posts DO
        SET post_title = CONCAT('Amazing Blog Post Number ', LPAD(i, 6, '0'), ': Exploring New Horizons');
        SET post_slug = CONCAT('blog-post-', LPAD(i, 6, '0'));
        SET post_content = CONCAT('This is the content of blog post number ', i, '. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Sed ut perspiciatis unde omnis iste natus error sit voluptatem accusantium doloremque laudantium, totam rem aperiam, eaque ipsa quae ab illo inventore veritatis et quasi architecto beatae vitae dicta sunt explicabo. Nemo enim ipsam voluptatem quia voluptas sit aspernatur aut odit aut fugit, sed quia consequuntur magni dolores eos qui ratione voluptatem sequi nesciunt.');
        SET post_excerpt = CONCAT('This is a brief excerpt for blog post ', i, ' that gives readers a preview of the content.');
        SET post_author_id = FLOOR(1 + (RAND() * max_author_id));
        SET post_published = RAND() > 0.3; -- 70% dos posts publicados
        SET post_views = FLOOR(0 + (RAND() * 10000));
        
        INSERT INTO `posts` (`title`, `slug`, `content`, `excerpt`, `author_id`, `published`, `views`, `published_at`) 
        VALUES (post_title, post_slug, post_content, post_excerpt, post_author_id, post_published, post_views, 
                IF(post_published, NOW() - INTERVAL FLOOR(RAND() * 365) DAY, NULL));
        
        SET i = i + 1;
    END WHILE;
END$$

CREATE PROCEDURE GenerateComments(IN num_comments INT)
BEGIN
    DECLARE i INT DEFAULT 1;
    DECLARE comment_post_id INT;
    DECLARE comment_author VARCHAR(255);
    DECLARE comment_email VARCHAR(255);
    DECLARE comment_content TEXT;
    DECLARE comment_approved BOOLEAN;
    DECLARE max_post_id INT;
    
    SELECT MAX(id) INTO max_post_id FROM posts;
    
    WHILE i <= num_comments DO
        SET comment_post_id = FLOOR(1 + (RAND() * max_post_id));
        SET comment_author = CONCAT('Commenter ', LPAD(i, 6, '0'));
        SET comment_email = CONCAT('commenter', i, '@example.com');
        SET comment_content = CONCAT('This is comment number ', i, '. Great post! I really enjoyed reading this content and found it very informative.');
        SET comment_approved = RAND() > 0.2; -- 80% dos comentários aprovados
        
        INSERT INTO `comments` (`post_id`, `author_name`, `author_email`, `content`, `approved`) 
        VALUES (comment_post_id, comment_author, comment_email, comment_content, comment_approved);
        
        SET i = i + 1;
    END WHILE;
END$$

CREATE PROCEDURE GeneratePostTags(IN num_relations INT)
BEGIN
    DECLARE i INT DEFAULT 1;
    DECLARE relation_post_id INT;
    DECLARE relation_tag_id INT;
    DECLARE max_post_id INT;
    DECLARE max_tag_id INT;
    
    SELECT MAX(id) INTO max_post_id FROM posts;
    SELECT MAX(id) INTO max_tag_id FROM tags;
    
    WHILE i <= num_relations DO
        SET relation_post_id = FLOOR(1 + (RAND() * max_post_id));
        SET relation_tag_id = FLOOR(1 + (RAND() * max_tag_id));
        
        INSERT IGNORE INTO `post_tags` (`post_id`, `tag_id`) 
        VALUES (relation_post_id, relation_tag_id);
        
        SET i = i + 1;
    END WHILE;
END$$

DELIMITER ;

-- Gerar dados do blog
CALL GenerateAuthors(1000);        -- 1.000 autores
CALL GenerateTags(500);            -- 500 tags
CALL GeneratePosts(50000);         -- 50.000 posts
CALL GenerateComments(150000);     -- 150.000 comentários
CALL GeneratePostTags(100000);     -- 100.000 relações post-tag

-- Remover procedimentos
DROP PROCEDURE GenerateAuthors;
DROP PROCEDURE GenerateTags;
DROP PROCEDURE GeneratePosts;
DROP PROCEDURE GenerateComments;
DROP PROCEDURE GeneratePostTags;

-- Usar o schema analytics
USE `analytics`;

-- Criar tabela de eventos
CREATE TABLE IF NOT EXISTS `events` (
    `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
    `user_id` INT,
    `session_id` VARCHAR(255),
    `event_type` VARCHAR(100) NOT NULL,
    `event_data` JSON,
    `url` VARCHAR(1000),
    `user_agent` TEXT,
    `ip_address` VARCHAR(45),
    `country` VARCHAR(100),
    `city` VARCHAR(100),
    `device_type` ENUM('desktop', 'mobile', 'tablet') DEFAULT 'desktop',
    `browser` VARCHAR(100),
    `os` VARCHAR(100),
    `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX `idx_user_id` (`user_id`),
    INDEX `idx_session_id` (`session_id`),
    INDEX `idx_event_type` (`event_type`),
    INDEX `idx_created_at` (`created_at`),
    INDEX `idx_country` (`country`),
    INDEX `idx_device_type` (`device_type`)
);

-- Procedimento para gerar eventos de analytics
DELIMITER $$

CREATE PROCEDURE GenerateEvents(IN num_events INT)
BEGIN
    DECLARE i INT DEFAULT 1;
    DECLARE event_user_id INT;
    DECLARE event_session_id VARCHAR(255);
    DECLARE event_type VARCHAR(100);
    DECLARE event_url VARCHAR(1000);
    DECLARE event_country VARCHAR(100);
    DECLARE event_city VARCHAR(100);
    DECLARE event_device VARCHAR(20);
    DECLARE event_browser VARCHAR(100);
    DECLARE event_os VARCHAR(100);
    
    WHILE i <= num_events DO
        SET event_user_id = FLOOR(1 + (RAND() * 50000));
        SET event_session_id = CONCAT('session_', FLOOR(1 + (RAND() * 100000)));
        SET event_type = ELT(FLOOR(1 + (RAND() * 8)), 
            'page_view', 'click', 'form_submit', 'purchase', 'login', 'logout', 'search', 'download');
        SET event_url = CONCAT('https://example.com/', 
            ELT(FLOOR(1 + (RAND() * 10)), 'home', 'products', 'about', 'contact', 'blog', 'login', 'register', 'cart', 'checkout', 'profile'));
        SET event_country = ELT(FLOOR(1 + (RAND() * 5)), 'Brasil', 'Estados Unidos', 'Argentina', 'Chile', 'Uruguai');
        SET event_city = ELT(FLOOR(1 + (RAND() * 10)), 
            'São Paulo', 'Rio de Janeiro', 'Belo Horizonte', 'Salvador', 'Brasília',
            'New York', 'Los Angeles', 'Buenos Aires', 'Santiago', 'Montevideo');
        SET event_device = ELT(FLOOR(1 + (RAND() * 3)), 'desktop', 'mobile', 'tablet');
        SET event_browser = ELT(FLOOR(1 + (RAND() * 5)), 'Chrome', 'Firefox', 'Safari', 'Edge', 'Opera');
        SET event_os = ELT(FLOOR(1 + (RAND() * 6)), 'Windows', 'macOS', 'Linux', 'iOS', 'Android', 'Other');
        
        INSERT INTO `events` (`user_id`, `session_id`, `event_type`, `url`, `country`, `city`, `device_type`, `browser`, `os`) 
        VALUES (event_user_id, event_session_id, event_type, event_url, event_country, event_city, event_device, event_browser, event_os);
        
        SET i = i + 1;
    END WHILE;
END$$

DELIMITER ;

-- Gerar eventos de analytics
CALL GenerateEvents(500000);      -- 500.000 eventos

-- Remover procedimento
DROP PROCEDURE GenerateEvents;

-- Mostrar estatísticas finais
SELECT 'DADOS GERADOS COM SUCESSO!' as status;

SELECT 
    'ecommerce' as schema_name,
    'users' as table_name,
    COUNT(*) as total_records
FROM ecommerce.users
UNION ALL
SELECT 'ecommerce', 'categories', COUNT(*) FROM ecommerce.categories
UNION ALL
SELECT 'ecommerce', 'products', COUNT(*) FROM ecommerce.products
UNION ALL
SELECT 'ecommerce', 'orders', COUNT(*) FROM ecommerce.orders
UNION ALL
SELECT 'ecommerce', 'order_items', COUNT(*) FROM ecommerce.order_items
UNION ALL
SELECT 'blog', 'authors', COUNT(*) FROM blog.authors
UNION ALL
SELECT 'blog', 'posts', COUNT(*) FROM blog.posts
UNION ALL
SELECT 'blog', 'comments', COUNT(*) FROM blog.comments
UNION ALL
SELECT 'blog', 'tags', COUNT(*) FROM blog.tags
UNION ALL
SELECT 'blog', 'post_tags', COUNT(*) FROM blog.post_tags
UNION ALL
SELECT 'analytics', 'events', COUNT(*) FROM analytics.events;

SELECT 
    CONCAT('Total de registros: ', 
    (SELECT COUNT(*) FROM ecommerce.users) +
    (SELECT COUNT(*) FROM ecommerce.categories) +
    (SELECT COUNT(*) FROM ecommerce.products) +
    (SELECT COUNT(*) FROM ecommerce.orders) +
    (SELECT COUNT(*) FROM ecommerce.order_items) +
    (SELECT COUNT(*) FROM blog.authors) +
    (SELECT COUNT(*) FROM blog.posts) +
    (SELECT COUNT(*) FROM blog.comments) +
    (SELECT COUNT(*) FROM blog.tags) +
    (SELECT COUNT(*) FROM blog.post_tags) +
    (SELECT COUNT(*) FROM analytics.events)
    ) as total_summary;