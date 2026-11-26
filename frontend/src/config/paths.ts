// ルートのパスを一元管理する。リンクやリダイレクトでは文字列を直書きせずこれを使う
export const paths = {
  home: "/",
  login: "/login",
  reports: {
    list: "/reports",
    detail: "/reports/:id",
    getDetail: (id: number | string) => `/reports/${id}`,
  },
  approvals: "/approvals",
  accounting: "/accounting",
} as const
