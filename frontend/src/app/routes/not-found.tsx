import { Link } from "react-router"
import { paths } from "@/config/paths"

export function Component() {
  return (
    <main className="p-6">
      <h1 className="text-2xl font-bold">ページが見つかりません</h1>
      <Link to={paths.home} className="mt-4 inline-block underline">
        トップへ戻る
      </Link>
    </main>
  )
}
