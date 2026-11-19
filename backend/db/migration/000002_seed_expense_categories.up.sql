-- 科目マスタの初期データ（管理画面は作らないため、マイグレーションで投入する）
-- sort_order は10刻みにして、あとから間に差し込めるようにしておく
INSERT INTO expense_categories (name, sort_order) VALUES
    ('交通費', 10),
    ('会議費', 20),
    ('接待交際費', 30),
    ('消耗品費', 40),
    ('通信費', 50),
    ('その他', 90)
ON CONFLICT (name) DO NOTHING;
