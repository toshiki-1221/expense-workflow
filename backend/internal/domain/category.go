package domain

// Category は経費申請で選択できる科目を表す。
type Category struct {
	ID        int64
	Name      string
	SortOrder int32
}
