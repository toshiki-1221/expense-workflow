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
