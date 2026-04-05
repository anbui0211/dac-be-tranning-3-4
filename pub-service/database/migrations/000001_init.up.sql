-- Create contents table
CREATE TABLE IF NOT EXISTS contents (
    id VARCHAR(36) PRIMARY KEY,
    content TEXT NOT NULL,
    image_url VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- Create message_schedules table
CREATE TABLE IF NOT EXISTS message_schedules (
    id VARCHAR(36) PRIMARY KEY,
    content_id VARCHAR(36),
    segment VARCHAR(100),
    time_schedule VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (content_id) REFERENCES contents(id) ON DELETE SET NULL
);

-- Seed 5 content records
INSERT INTO contents (id, content, image_url) VALUES
('c001', 'Welcome message', 'https://example.com/img1.jpg'),
('c002', 'Promotional offer', 'https://example.com/img2.jpg'),
('c003', 'Product announcement', 'https://example.com/img3.jpg'),
('c004', 'Event invitation', 'https://example.com/img4.jpg'),
('c005', 'Newsletter update', 'https://example.com/img5.jpg');

-- Seed 5 message schedule records
INSERT INTO message_schedules (id, content_id, segment, time_schedule) VALUES
('s001', 'c001', 'new_users', '09:00'),
('s002', 'c002', 'premium_users', '10:00'),
('s003', 'c003', 'all_users', '12:00'),
('s004', 'c004', 'vip_users', '14:00'),
('s005', 'c005', 'subscribers', '16:00');
