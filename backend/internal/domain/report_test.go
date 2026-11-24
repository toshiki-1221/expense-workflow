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
