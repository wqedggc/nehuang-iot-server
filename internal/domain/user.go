// Package domain defines the core business entities for the nehuang IoT platform.
package domain

import "time"

// UserType distinguishes between admin-side and farmer-side users.
type UserType uint8

const (
	UserTypeAdmin  UserType = 1 // admin-side user
	UserTypeFarmer UserType = 2 // farmer (mini-program) user
)

// Role defines the permission level of a user.
type Role uint8

const (
	RoleSuperAdmin Role = 1 // super_admin: full system access
	RoleAdmin      Role = 2 // admin: regular admin access
	RoleFarmer     Role = 3 // farmer: mini-program access only
)

// UserStatus represents the lifecycle state of a user account.
type UserStatus uint8

const (
	UserStatusActive   UserStatus = 1 // active: can log in
	UserStatusDisabled UserStatus = 2 // disabled: cannot log in
	UserStatusDeleted  UserStatus = 3 // deleted: soft-deleted
)

// User is the core user entity, mapping to the users table.
type User struct {
	UserID        uint64
	UserType      UserType
	Role          Role
	Username      *string    // nullable; used for admin login
	Mobile        *string    // nullable
	WechatOpenID  *string    // nullable; used for mini-program login
	WechatUnionID *string    // nullable
	PasswordHash  *string    // nullable; bcrypt hash
	DisplayName   *string    // nullable; shown in UI
	AvatarURL     *string    // nullable
	Status        UserStatus
	LastLoginAtMS *time.Time // nullable; last successful login timestamp
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
