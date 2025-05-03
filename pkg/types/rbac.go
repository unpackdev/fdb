package types

import "strings"

// Role represents a user role in the system.
type Role string

func (r Role) String() string {
	return string(r)
}

// Roles represents a group of user roles in the system.
type Roles []Role

func (r Roles) String() string {
	toReturn := strings.Builder{}
	for i, role := range r {
		toReturn.WriteString(role.String())
		if i > 0 && i < len(r)-1 {
			toReturn.WriteString(", ")
		}
	}
	return toReturn.String()
}

// Permission represents an action that can be performed.
type Permission string

func (p Permission) String() string {
	return string(p)
}
