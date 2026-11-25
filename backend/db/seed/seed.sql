-- 開発用の初期ユーザー。本番に流れないよう、マイグレーションとは分ける。
-- 実行: make seed
-- パスワードはすべて "password"（go run ./cmd/hashpw password で作成）
-- メールアドレスは小文字で入れる（ログイン時に小文字へ正規化して照合するため）

BEGIN;

INSERT INTO users (email, password_hash, name, role) VALUES
    ('sato@example.com',   '$2a$12$7NS/7VC4AkoxXGPHN0L29uhyiCeLQKv5EAKspCdr9E0cNUywuEYB2', '佐藤部長', 'member'),
    ('yamada@example.com', '$2a$12$7NS/7VC4AkoxXGPHN0L29uhyiCeLQKv5EAKspCdr9E0cNUywuEYB2', '山田太郎', 'member'),
    ('keiri@example.com',  '$2a$12$7NS/7VC4AkoxXGPHN0L29uhyiCeLQKv5EAKspCdr9E0cNUywuEYB2', '経理花子', 'accounting')
ON CONFLICT (email) DO NOTHING;

-- 山田さんの承認者は佐藤部長（佐藤部長は is_approver = true になる）
UPDATE users
SET approver_id = (SELECT id FROM users WHERE email = 'sato@example.com'),
    updated_at  = CURRENT_TIMESTAMP
WHERE email = 'yamada@example.com'
  AND approver_id IS NULL;

COMMIT;
