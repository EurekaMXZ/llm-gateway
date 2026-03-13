package domain

import "time"

type Role string

const (
	RoleSuperuser Role = "superuser"
	RoleAdmin     Role = "administrator"
	RoleUser      Role = "regular_user"
)

type User struct {
	ID           string
	Username     string
	DisplayName  string
	Role         Role
	ParentID     string
	PasswordHash string
	CreatedAt    time.Time
}
