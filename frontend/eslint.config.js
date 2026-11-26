import fs from "node:fs"
import js from "@eslint/js"
import globals from "globals"
import reactHooks from "eslint-plugin-react-hooks"
import reactRefresh from "eslint-plugin-react-refresh"
import { importX } from "eslint-plugin-import-x"
import { createTypeScriptImportResolver } from "eslint-import-resolver-typescript"
import tseslint from "typescript-eslint"
import { defineConfig, globalIgnores } from "eslint/config"

// src/features 配下の feature 名一覧（feature が増えると自動で制約対象になる）
const features = fs
  .readdirSync(new URL("./src/features", import.meta.url), { withFileTypes: true })
  .filter((entry) => entry.isDirectory())
  .map((entry) => entry.name)

// 共有レイヤー：どこからでも使われる側なので、features / app に依存してはいけない
const sharedDirs = ["components", "config", "hooks", "lib", "types", "utils"]

export default defineConfig([
  globalIgnores(["dist", "coverage"]),
  {
    files: ["**/*.{ts,tsx}"],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      globals: globals.browser,
    },
  },
  {
    // 依存の向きを shared → features → app の一方向に強制する
    files: ["src/**/*.{ts,tsx}"],
    plugins: { "import-x": importX },
    settings: {
      "import-x/resolver-next": [
        createTypeScriptImportResolver({ project: "./tsconfig.app.json" }),
      ],
    },
    rules: {
      "import-x/no-restricted-paths": [
        "error",
        {
          zones: [
            // feature 同士は import しない（組み合わせは app/routes で行う）
            ...features.map((feature) => ({
              target: `./src/features/${feature}`,
              from: "./src/features",
              except: [`./${feature}`],
              message: "feature 間の import は禁止です。app/routes で組み合わせてください。",
            })),
            // features から app は import しない
            {
              target: "./src/features",
              from: "./src/app",
              message: "features から app への import は禁止です。",
            },
            // 共有レイヤーから features / app は import しない
            {
              target: sharedDirs.map((dir) => `./src/${dir}`),
              from: ["./src/features", "./src/app"],
              message: "共有レイヤーから features / app への import は禁止です。",
            },
          ],
        },
      ],
    },
  },
  {
    // shadcn/ui のコンポーネントは variants 等も export するため除外
    files: ["src/components/ui/**/*.{ts,tsx}"],
    rules: {
      "react-refresh/only-export-components": "off",
    },
  },
])
