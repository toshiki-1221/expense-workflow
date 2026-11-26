import { Link } from "react-router"
import { buttonVariants } from "@/components/ui/button"
import { paths } from "@/config/paths"

export function Component() {
  return (
    <main className="p-6">
      <h1 className="text-3xl font-bold">経費精算</h1>
      <nav className="mt-4 flex gap-2">
        <Link to={paths.reports.list} className={buttonVariants()}>
          申請一覧
        </Link>
        <Link to={paths.approvals} className={buttonVariants({ variant: "outline" })}>
          承認一覧
        </Link>
        <Link to={paths.accounting} className={buttonVariants({ variant: "outline" })}>
          経理
        </Link>
      </nav>
    </main>
  )
}
