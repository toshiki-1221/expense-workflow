# フロントエンドのディレクトリ構成

[Bulletproof React](https://github.com/alan2207/bulletproof-react) の最新版をベースにした feature-based 構成です。
機能（feature）単位でコードをまとめ、依存の向きを ESLint で強制します。

## 全体像

```
src/
├── app/                  # アプリケーション層：画面とアプリ全体の組み立て
│   ├── index.tsx         # App（provider と router を組み合わせる）
│   ├── provider.tsx      # QueryClientProvider などのグローバルな Provider
│   ├── router.tsx        # ルート定義
│   └── routes/           # 画面コンポーネント（1ファイル = 1画面）
│       ├── home.tsx      # トップ（各画面へのリンク）
│       ├── login.tsx     # ログイン
│       ├── reports/      # 経費申請の画面
│       │   ├── list.tsx  # 申請一覧
│       │   └── detail.tsx # 申請詳細（承認・差し戻し操作もここで組み合わせる）
│       ├── approvals.tsx # 承認一覧（承認者向け）
│       ├── accounting.tsx # 経理（支払済みの処理）
│       └── not-found.tsx # 404
├── features/             # 機能層：業務ごとのロジックと UI
│   ├── auth/             # ログイン、/me、ログイン中ユーザー
│   ├── reports/          # 経費申請の作成・編集・提出
│   ├── approvals/        # 承認・差し戻し
│   └── payments/         # 経理の一覧・支払済み
├── components/           # 共有：どこからでも使う UI 部品
│   └── ui/               # shadcn/ui のコンポーネント
├── config/               # 共有：定数・設定（paths.ts など）
├── hooks/                # 共有：汎用のカスタムフック
├── lib/                  # 共有：ライブラリの設定・ラッパー（query-client, api-client, utils）
├── types/                # 共有：API 共通の型
├── utils/                # 共有：汎用の関数
├── testing/              # テスト用の設定・MSW ハンドラー・補助関数
├── index.css             # Tailwind と shadcn のテーマ（CSS 変数）
└── main.tsx              # エントリーポイント
```

> `features/` 以下と `hooks/`, `types/`, `utils/` は必要になった時点で作成します。

## 各レイヤーの役割

| レイヤー         | ディレクトリ                                                   | 役割                                   | 置かないもの                  |
| ---------------- | -------------------------------------------------------------- | -------------------------------------- | ----------------------------- |
| アプリケーション | `app/`                                                         | ルーティング、Provider、画面の組み立て | 業務ロジック、API 呼び出し    |
| 機能             | `features/*`                                                   | 1つの業務に閉じたロジックと UI         | 他の feature に依存するコード |
| 共有             | `components/`, `config/`, `hooks/`, `lib/`, `types/`, `utils/` | 業務に依存しない汎用コード             | 特定の業務の知識              |

### `app/routes/`（画面）

- 画面は **feature を組み合わせるだけ** にし、ロジックは持たせない。
- 「feature A の画面で feature B の部品も使いたい」場合は、feature 同士で import せず、ここで両方を並べる。
  - 例：申請詳細画面（`reports/detail.tsx`）で、`features/reports` の詳細表示と `features/approvals` の承認ボタンを組み合わせる。

### `features/*`（機能）

feature の中は次の構成を基本とし、**必要なフォルダだけ** 作ります。

```
features/reports/
├── api/          # API 呼び出しと React Query の定義（get-reports.ts, create-report.ts …）
├── components/   # この feature 専用のコンポーネント
├── hooks/        # この feature 専用のフック
├── schemas/      # zod のスキーマ（フォームのバリデーション、API レスポンスの検証）
├── types/        # この feature 専用の型
└── utils/        # この feature 専用の関数
```

- `api/` には1ファイル1エンドポイントで、`queryOptions()` によるクエリ定義と、それを使うフックをまとめて置く。
- feature をまたいで使うようになったコードは、共有レイヤーへ移す。

## 依存のルール

import は **共有 → 機能 → アプリケーション** の一方向だけ許可します。

```
  components / config / hooks / lib / types / utils   （共有）
                        ↑
                    features/*                         （機能）
                        ↑
                       app/                            （アプリケーション）

  ↑ = 「下から上を import してよい」
```

| import 元 → import 先                   | 可否   |
| --------------------------------------- | ------ |
| `app` → `features`, 共有                | OK     |
| `features/A` → 共有                     | OK     |
| `features/A` → `features/A`（自分自身） | OK     |
| `features/A` → `features/B`             | **NG** |
| `features` → `app`                      | **NG** |
| 共有 → `features`, `app`                | **NG** |

このルールは `frontend/eslint.config.js` の `import-x/no-restricted-paths` で強制しており、違反すると `npm run lint` がエラーになります。
制約対象の feature 一覧は `frontend/src/features` のディレクトリから自動で作られるため、feature を追加したときに設定を変更する必要はありません。

## 規約

### import

- `@/` エイリアス（`src/` を指す）で import する。同じ feature 内のファイル同士は相対パスでもよい。
- **`index.ts` でまとめて re-export する書き方（barrel file）は使わない。** ファイルを直接 import する。
  - 理由：Vite の開発サーバーが遅くなる、tree-shaking が効きにくい、循環参照が起きやすい。

```ts
// OK
import { getReportsQueryOptions } from "@/features/reports/api/get-reports"

// NG
import { getReportsQueryOptions } from "@/features/reports"
```

### ルーティング

- ルーターは React Router を使い、ルートは `app/router.tsx` にコードで定義する（ファイルベースルーティングは採用していない）。
- 画面は `lazy` で読み込み、画面ごとにバンドルを分割する。そのため **画面ファイルはコンポーネントを `Component` という名前で export する**。

```tsx
// app/routes/approvals.tsx
export function Component() {
  return <main>…</main>
}

// app/router.tsx
{ path: paths.approvals, lazy: () => import("./routes/approvals") }
```

- パスは `config/paths.ts` で一元管理し、リンクやリダイレクトで文字列を直書きしない。

```tsx
<Link to={paths.reports.getDetail(report.id)}>詳細</Link>
```

### ファイル名

- ファイル名は kebab-case（`get-reports.ts`, `report-form.tsx`, `not-found.tsx`）。
- テストは対象ファイルの隣に `*.test.ts(x)` として置く（例：`home.tsx` と `home.test.tsx`）。

## よくある作業

### 画面を追加する

1. `config/paths.ts` にパスを追加する。
2. `app/routes/` に画面ファイルを作り、`Component` を export する。
3. `app/router.tsx` にルートを追加する。

### feature を追加する

1. `src/features/<feature名>/` を作る。
2. 必要なフォルダ（`api/`, `components/` など）だけ作る。
3. 画面（`app/routes/`）から import して使う。

ESLint の依存ルールは自動で新しい feature にも適用されます。

### どこに置くか迷ったとき

| 置きたいもの                                       | 置き場所                                        |
| -------------------------------------------------- | ----------------------------------------------- |
| 1つの業務でしか使わないコンポーネント・フック・API | `features/<業務>/`                              |
| 複数の業務で使う UI 部品                           | `components/`                                   |
| shadcn/ui で追加したコンポーネント                 | `components/ui/`（`npx shadcn add` で自動配置） |
| ライブラリの初期化・薄いラッパー                   | `lib/`                                          |
| 複数の feature を組み合わせる処理                  | `app/routes/` の画面                            |
| テスト用の MSW ハンドラーや render の補助関数      | `testing/`                                      |

## 採用しなかった選択肢

| 選択肢                                             | 採用しなかった理由                                                                                                                                       |
| -------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Feature-Sliced Design（FSD）                       | 6層（app / pages / widgets / features / entities / shared）はこの規模には多すぎ、`features` と `entities` の境界で迷うコストが大きい。                   |
| ファイルベースルーティング（TanStack Router など） | 画面数が少なくメリットが小さい。権限ごとのガードを `router.tsx` の1か所で読めるほうが分かりやすい。検索条件を型付きでURLに持たせたくなったら再検討する。 |
