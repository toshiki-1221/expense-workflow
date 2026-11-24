package domain

import (
	"slices"
	"testing"
)

func ptr[T any](v T) *T { return &v }

// 登場人物: 申請者(1)、その承認者(2)、経理(3)、無関係の人(4)
var (
	owner      = &User{ID: 1, Role: RoleMember, ApproverID: ptr(int64(2))}
	approver   = &User{ID: 2, Role: RoleMember}
	accounting = &User{ID: 3, Role: RoleAccounting}
	stranger   = &User{ID: 4, Role: RoleMember}
)

func report(status ReportStatus) *Report {
	r := &Report{ID: 10, UserID: owner.ID, Status: status}
	if status != ReportStatusDraft {
		r.ApproverID = ptr(approver.ID) // 申請した時点で承認者が決まる
	}
	return r
}

func TestCanBeViewedBy(t *testing.T) {
	tests := []struct {
		name   string
		viewer *User
		want   bool
	}{
		{"申請者本人", owner, true},
		{"この申請の承認者", approver, true},
		{"経理", accounting, true},
		{"無関係の人", stranger, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := report(ReportStatusSubmitted).CanBeViewedBy(tt.viewer); got != tt.want {
				t.Errorf("CanBeViewedBy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllowedActions(t *testing.T) {
	ownerWithoutApprover := &User{ID: 1, Role: RoleMember}

	tests := []struct {
		name   string
		status ReportStatus
		viewer *User
		want   []ReportAction
	}{
		{"下書き・本人", ReportStatusDraft, owner, []ReportAction{ReportActionEdit, ReportActionSubmit, ReportActionDelete}},
		{"下書き・承認者がいない本人は申請できない", ReportStatusDraft, ownerWithoutApprover, []ReportAction{ReportActionEdit, ReportActionDelete}},
		{"差し戻し・本人は削除できない", ReportStatusRejected, owner, []ReportAction{ReportActionEdit, ReportActionSubmit}},
		{"申請中・本人は何もできない", ReportStatusSubmitted, owner, []ReportAction{}},
		{"申請中・承認者", ReportStatusSubmitted, approver, []ReportAction{ReportActionApprove, ReportActionReject}},
		{"申請中・経理は承認できない", ReportStatusSubmitted, accounting, []ReportAction{}},
		{"承認済み・経理", ReportStatusApproved, accounting, []ReportAction{ReportActionPay}},
		{"承認済み・承認者は支払えない", ReportStatusApproved, approver, []ReportAction{}},
		{"支払済み・経理", ReportStatusPaid, accounting, []ReportAction{}},
		{"下書き・無関係の人", ReportStatusDraft, stranger, []ReportAction{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := report(tt.status).AllowedActions(tt.viewer)
			if !slices.Equal(got, tt.want) {
				t.Errorf("AllowedActions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiffItems(t *testing.T) {
	existing := []ReportItem{{ID: 11}, {ID: 12}, {ID: 13}}
	ids := func(ins []ReportItemInput) []int64 {
		out := []int64{}
		for _, in := range ins {
			out = append(out, in.ID)
		}
		return out
	}

	tests := []struct {
		name       string
		inputs     []ReportItemInput
		wantCreate int
		wantUpdate []int64
		wantDelete []int64
		wantErr    error
	}{
		{"そのまま送り返す", []ReportItemInput{{ID: 11}, {ID: 12}, {ID: 13}}, 0, []int64{11, 12, 13}, []int64{}, nil},
		{"1件追加", []ReportItemInput{{ID: 11}, {ID: 12}, {ID: 13}, {ID: 0}}, 1, []int64{11, 12, 13}, []int64{}, nil},
		{"送られてこなかった明細は削除", []ReportItemInput{{ID: 13}}, 0, []int64{13}, []int64{11, 12}, nil},
		{"追加・更新・削除が混ざる", []ReportItemInput{{ID: 0}, {ID: 12}, {ID: 0}}, 2, []int64{12}, []int64{11, 13}, nil},
		{"空で送ると全部削除", []ReportItemInput{}, 0, []int64{}, []int64{11, 12, 13}, nil},
		{"この申請にない id", []ReportItemInput{{ID: 99}}, 0, nil, nil, ErrUnknownItem},
		{"同じ id が2回", []ReportItemInput{{ID: 11}, {ID: 11}}, 0, nil, nil, ErrUnknownItem},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DiffItems(existing, tt.inputs)
			if err != tt.wantErr {
				t.Fatalf("DiffItems() err = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if len(got.Create) != tt.wantCreate {
				t.Errorf("Create = %d件, want %d件", len(got.Create), tt.wantCreate)
			}
			if !slices.Equal(ids(got.Update), tt.wantUpdate) {
				t.Errorf("Update = %v, want %v", ids(got.Update), tt.wantUpdate)
			}
			if !slices.Equal(got.Delete, tt.wantDelete) {
				t.Errorf("Delete = %v, want %v", got.Delete, tt.wantDelete)
			}
		})
	}
}

func TestCheckAction(t *testing.T) {
	ownerWithoutApprover := &User{ID: 1, Role: RoleMember}

	tests := []struct {
		name   string
		status ReportStatus
		viewer *User
		action ReportAction
		want   error
	}{
		{"下書きを本人が申請", ReportStatusDraft, owner, ReportActionSubmit, nil},
		{"差し戻しを本人が再申請", ReportStatusRejected, owner, ReportActionSubmit, nil},
		{"申請中を本人がもう一度申請", ReportStatusSubmitted, owner, ReportActionSubmit, ErrInvalidStatus},
		{"承認者がいない本人が申請", ReportStatusDraft, ownerWithoutApprover, ReportActionSubmit, ErrNoApprover},
		{"経理が他人の下書きを申請", ReportStatusDraft, accounting, ReportActionSubmit, ErrForbidden},
		{"無関係の人が申請", ReportStatusDraft, stranger, ReportActionSubmit, ErrNotFound},
		{"承認者が承認", ReportStatusSubmitted, approver, ReportActionApprove, nil},
		{"承認者が承認済みをもう一度承認", ReportStatusApproved, approver, ReportActionApprove, ErrInvalidStatus},
		{"経理が承認", ReportStatusSubmitted, accounting, ReportActionApprove, ErrForbidden},
		{"経理が申請中を支払", ReportStatusSubmitted, accounting, ReportActionPay, ErrInvalidStatus},
		{"承認者が支払", ReportStatusApproved, approver, ReportActionPay, ErrForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := report(tt.status).CheckAction(tt.viewer, tt.action); got != tt.want {
				t.Errorf("CheckAction() = %v, want %v", got, tt.want)
			}
		})
	}
}
