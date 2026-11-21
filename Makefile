# マイグレーションは api コンテナの中の migrate CLI で実行する。
# 接続情報はコンテナ内の環境変数（.env の DB_*）から組み立てる。
MIGRATE = docker compose exec api sh -c 'migrate -path db/migration \
	-database "postgres://$$DB_USER:$$DB_PASSWORD@$$DB_HOST:$$DB_PORT/$$DB_NAME?sslmode=disable" $(1)'

.PHONY: migrate-up migrate-down migrate-version migrate-new seed

## 未適用のマイグレーションをすべて流す
migrate-up:
	$(call MIGRATE,up)

## 直前のマイグレーションを1つ戻す
migrate-down:
	$(call MIGRATE,down 1)

## 現在のバージョンを表示する
migrate-version:
	$(call MIGRATE,version)

## 新しいマイグレーションファイルを作る（例: make migrate-new name=add_index）
migrate-new:
	@test -n "$(name)" || (echo "name を指定してください（例: make migrate-new name=add_index）" && exit 1)
	docker compose exec api migrate create -ext sql -dir db/migration -seq $(name)

## 開発用の初期ユーザーを入れる（何度流しても重複しない）
seed:
	docker compose exec -T db sh -c 'psql -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"' < backend/db/seed/seed.sql
