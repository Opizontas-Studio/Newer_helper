package utils

// Permission levels
const (
	AdminPermission = "admin"
	UserPermission  = "user"
	GuestPermission = "guest"
)

// contains checks if a slice of strings contains an element.
func contains(slice []string, item string) bool {
	for _, a := range slice {
		if a == item {
			return true
		}
	}
	return false
}

// CheckPermission checks the highest permission level for a given list of group IDs against the configured roles.
func CheckPermission(userRoleIDs []string, adminRoleIDs []string, userRoleIDsConfig []string) string {
	for _, groupID := range userRoleIDs {
		if contains(adminRoleIDs, groupID) {
			return AdminPermission
		}
	}
	for _, groupID := range userRoleIDs {
		if contains(userRoleIDsConfig, groupID) {
			return UserPermission
		}
	}
	return GuestPermission
}
