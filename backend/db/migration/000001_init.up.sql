CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(100) NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'member'
        CHECK (role IN ('member', 'accounting')),
    approver_id BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT users_approver_not_self CHECK (approver_id IS NULL OR approver_id <> id)
);

CREATE TABLE expense_categories (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    sort_order INT NOT NULL
);

CREATE TABLE expense_reports (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    approver_id BIGINT REFERENCES users(id),
    title VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'submitted', 'approved', 'rejected', 'paid')),
    submitted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE expense_items (
    id BIGSERIAL PRIMARY KEY,
    report_id BIGINT NOT NULL REFERENCES expense_reports(id) ON DELETE CASCADE,
    category_id BIGINT NOT NULL REFERENCES expense_categories(id),
    used_on DATE NOT NULL,
    amount BIGINT NOT NULL CHECK (amount >= 1),
    description VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE approval_logs (
    id BIGSERIAL PRIMARY KEY,
    report_id BIGINT NOT NULL REFERENCES expense_reports(id) ON DELETE CASCADE,
    actor_id BIGINT NOT NULL REFERENCES users(id),
    action VARCHAR(20) NOT NULL CHECK (action IN ('submit', 'approve', 'reject', 'pay')),
    from_status VARCHAR(20) NOT NULL
        CHECK (from_status IN ('draft', 'submitted', 'approved', 'rejected', 'paid')),
    to_status VARCHAR(20) NOT NULL
        CHECK (to_status IN ('draft', 'submitted', 'approved', 'rejected', 'paid')),
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT approval_logs_reject_comment_required
        CHECK (action <> 'reject' OR (comment IS NOT NULL AND BTRIM(comment) <> ''))
);
