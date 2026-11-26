import { useParams } from "react-router"

// TODO: features/reports の詳細と、features/approvals の承認・差し戻し操作を組み合わせる
export function Component() {
  const { id } = useParams<{ id: string }>()

  return (
    <main className="p-6">
      <h1 className="text-2xl font-bold">申請詳細 #{id}</h1>
    </main>
  )
}
