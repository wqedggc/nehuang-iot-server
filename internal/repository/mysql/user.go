package mysql

import (
	"context"
	"database/sql"
	"time"

	"nehuang-iot-server/internal/domain"
	"nehuang-iot-server/internal/idgen"
)

// GetUserByUsername retrieves a user by their username. Used for admin login.
// Returns (nil, nil) when no matching user is found.
func (r *Repository) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT
  user_id, user_type, role, username, mobile,
  address, remark,
  wechat_openid, wechat_unionid, password_hash,
  display_name, avatar_url, status,
  last_login_at_ms, created_at, updated_at
FROM users
WHERE username = ?
LIMIT 1`, username)
	return scanUser(row)
}

// GetUserByOpenID retrieves a user by their WeChat open_id. Used for mini-program login.
// Returns (nil, nil) when no matching user is found — callers should create a new user in that case.
func (r *Repository) GetUserByOpenID(ctx context.Context, openid string) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT
  user_id, user_type, role, username, mobile,
  address, remark,
  wechat_openid, wechat_unionid, password_hash,
  display_name, avatar_url, status,
  last_login_at_ms, created_at, updated_at
FROM users
WHERE wechat_openid = ?
LIMIT 1`, openid)
	return scanUser(row)
}

// CreateUser inserts a new user record. The UserID field is populated using the
// Snowflake generator if it is zero.
func (r *Repository) CreateUser(ctx context.Context, u *domain.User) error {
	if u.UserID == 0 {
		u.UserID = idgen.NextID()
	}

	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now

	_, err := r.db.ExecContext(ctx, `
INSERT INTO users
  (user_id, user_type, role, username, mobile,
   address, remark,
   wechat_openid, wechat_unionid, password_hash,
   display_name, avatar_url, status,
   last_login_at_ms, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?,
   ?, ?,
   ?, ?, ?,
   ?, ?, ?,
   ?, ?, ?)`,
		u.UserID, uint8(u.UserType), uint8(u.Role),
		nullableString(u.Username), nullableString(u.Mobile),
		nullableString(u.Address), nullableString(u.Remark),
		nullableString(u.WechatOpenID), nullableString(u.WechatUnionID),
		nullableString(u.PasswordHash),
		nullableString(u.DisplayName), nullableString(u.AvatarURL),
		uint8(u.Status),
		nullableTime(u.LastLoginAtMS),
		u.CreatedAt, u.UpdatedAt,
	)
	return err
}

// UpdateLastLogin updates the last_login_at_ms column for the given user.
func (r *Repository) UpdateLastLogin(ctx context.Context, userID uint64, t time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET last_login_at_ms = ? WHERE user_id = ?`,
		t, userID,
	)
	return err
}

// GetUserByID retrieves a user by their primary key. Used by the /admin/me endpoint.
// Returns (nil, nil) when no matching user is found.
func (r *Repository) GetUserByID(ctx context.Context, userID uint64) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT
  user_id, user_type, role, username, mobile,
  address, remark,
  wechat_openid, wechat_unionid, password_hash,
  display_name, avatar_url, status,
  last_login_at_ms, created_at, updated_at
FROM users
WHERE user_id = ?
LIMIT 1`, userID)
	return scanUser(row)
}

// scanUser maps a single sql.Row into a domain.User, handling nullable columns.
// Returns (nil, nil) for sql.ErrNoRows.
func scanUser(row *sql.Row) (*domain.User, error) {
	var u domain.User
	var (
		username      sql.NullString
		mobile        sql.NullString
		address       sql.NullString
		remark        sql.NullString
		wechatOpenID  sql.NullString
		wechatUnionID sql.NullString
		passwordHash  sql.NullString
		displayName   sql.NullString
		avatarURL     sql.NullString
		lastLoginAtMS sql.NullTime
	)

	err := row.Scan(
		&u.UserID, &u.UserType, &u.Role,
		&username, &mobile,
		&address, &remark,
		&wechatOpenID, &wechatUnionID, &passwordHash,
		&displayName, &avatarURL, &u.Status,
		&lastLoginAtMS, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if username.Valid {
		u.Username = &username.String
	}
	if mobile.Valid {
		u.Mobile = &mobile.String
	}
	if address.Valid {
		u.Address = &address.String
	}
	if remark.Valid {
		u.Remark = &remark.String
	}
	if wechatOpenID.Valid {
		u.WechatOpenID = &wechatOpenID.String
	}
	if wechatUnionID.Valid {
		u.WechatUnionID = &wechatUnionID.String
	}
	if passwordHash.Valid {
		u.PasswordHash = &passwordHash.String
	}
	if displayName.Valid {
		u.DisplayName = &displayName.String
	}
	if avatarURL.Valid {
		u.AvatarURL = &avatarURL.String
	}
	if lastLoginAtMS.Valid {
		u.LastLoginAtMS = &lastLoginAtMS.Time
	}

	return &u, nil
}

// nullableString converts a *string to sql.NullString for use in queries.
func nullableString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

// nullableTime converts a *time.Time to sql.NullTime for use in queries.
func nullableTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

// GetFarmerByID retrieves a farmer user (user_type=2) with active binding count.
// Returns (nil, nil) when no matching farmer is found.
func (r *Repository) GetFarmerByID(ctx context.Context, userID uint64) (*domain.User, int, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT
  u.user_id, u.user_type, u.role, u.username, u.mobile,
  u.address, u.remark,
  u.wechat_openid, u.wechat_unionid, u.password_hash,
  u.display_name, u.avatar_url, u.status,
  u.last_login_at_ms, u.created_at, u.updated_at,
  COALESCE(bc.cnt, 0) AS binding_count
FROM users u
LEFT JOIN (
  SELECT user_id, COUNT(*) AS cnt
  FROM bind_relations
  WHERE bind_status = 1
  GROUP BY user_id
) bc ON u.user_id = bc.user_id
WHERE u.user_id = ? AND u.user_type = 2
LIMIT 1`, userID)

	var u domain.User
	var bindingCount int
	var (
		username      sql.NullString
		mobile        sql.NullString
		address       sql.NullString
		remark        sql.NullString
		wechatOpenID  sql.NullString
		wechatUnionID sql.NullString
		passwordHash  sql.NullString
		displayName   sql.NullString
		avatarURL     sql.NullString
		lastLoginAtMS sql.NullTime
	)

	err := row.Scan(
		&u.UserID, &u.UserType, &u.Role,
		&username, &mobile,
		&address, &remark,
		&wechatOpenID, &wechatUnionID, &passwordHash,
		&displayName, &avatarURL, &u.Status,
		&lastLoginAtMS, &u.CreatedAt, &u.UpdatedAt,
		&bindingCount,
	)
	if err == sql.ErrNoRows {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}

	if username.Valid {
		u.Username = &username.String
	}
	if mobile.Valid {
		u.Mobile = &mobile.String
	}
	if address.Valid {
		u.Address = &address.String
	}
	if remark.Valid {
		u.Remark = &remark.String
	}
	if wechatOpenID.Valid {
		u.WechatOpenID = &wechatOpenID.String
	}
	if wechatUnionID.Valid {
		u.WechatUnionID = &wechatUnionID.String
	}
	if passwordHash.Valid {
		u.PasswordHash = &passwordHash.String
	}
	if displayName.Valid {
		u.DisplayName = &displayName.String
	}
	if avatarURL.Valid {
		u.AvatarURL = &avatarURL.String
	}
	if lastLoginAtMS.Valid {
		u.LastLoginAtMS = &lastLoginAtMS.Time
	}

	return &u, bindingCount, nil
}

// UpdateFarmer updates editable fields for a farmer user.
func (r *Repository) UpdateFarmer(ctx context.Context, userID uint64, updates map[string]any) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
UPDATE users
SET display_name = COALESCE(?, display_name),
    mobile       = COALESCE(?, mobile),
    address      = COALESCE(?, address),
    remark       = COALESCE(?, remark),
    status       = COALESCE(?, status),
    updated_at   = ?
WHERE user_id = ? AND user_type = 2`,
		updates["display_name"],
		updates["mobile"],
		updates["address"],
		updates["remark"],
		updates["status"],
		now,
		userID,
	)
	return err
}
