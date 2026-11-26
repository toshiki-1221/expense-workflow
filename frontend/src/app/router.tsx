import { createBrowserRouter, RouterProvider } from "react-router"
import { paths } from "@/config/paths"

// 各画面は lazy で読み込み、画面ごとにバンドルを分割する
const router = createBrowserRouter([
  {
    path: paths.home,
    lazy: () => import("./routes/home"),
  },
  {
    path: paths.login,
    lazy: () => import("./routes/login"),
  },
  {
    path: paths.reports.list,
    lazy: () => import("./routes/reports/list"),
  },
  {
    path: paths.reports.detail,
    lazy: () => import("./routes/reports/detail"),
  },
  {
    path: paths.approvals,
    lazy: () => import("./routes/approvals"),
  },
  {
    path: paths.accounting,
    lazy: () => import("./routes/accounting"),
  },
  {
    path: "*",
    lazy: () => import("./routes/not-found"),
  },
])

export function AppRouter() {
  return <RouterProvider router={router} />
}
