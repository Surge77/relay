DELETE FROM memberships WHERE conversation_id = 'general';
DELETE FROM conversations WHERE id = 'general';
DELETE FROM users WHERE id IN ('alice', 'bob', 'carol');
