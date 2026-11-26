import { cn } from "./utils"

describe("cn", () => {
  it("衝突する Tailwind クラスは後ろを優先する", () => {
    expect(cn("p-4 text-sm", "p-2")).toBe("text-sm p-2")
  })

  it("falsy な値は無視する", () => {
    const isHidden: boolean = false
    expect(cn("font-bold", isHidden && "hidden", undefined, null)).toBe("font-bold")
  })
})
