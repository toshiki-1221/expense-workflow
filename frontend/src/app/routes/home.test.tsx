import { render, screen } from "@testing-library/react"
import { createMemoryRouter, RouterProvider } from "react-router"
import { Component as HomePage } from "./home"

describe("ホーム画面", () => {
  it("見出しと各画面へのリンクを表示する", () => {
    const router = createMemoryRouter([{ path: "/", element: <HomePage /> }])
    render(<RouterProvider router={router} />)

    expect(screen.getByRole("heading", { name: "経費精算" })).toBeInTheDocument()
    expect(screen.getByRole("link", { name: "申請一覧" })).toHaveAttribute("href", "/reports")
    expect(screen.getByRole("link", { name: "承認一覧" })).toHaveAttribute("href", "/approvals")
    expect(screen.getByRole("link", { name: "経理" })).toHaveAttribute("href", "/accounting")
  })
})
