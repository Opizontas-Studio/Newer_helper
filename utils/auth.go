package utils

// Permission levels
const (
	SuperAdminPermission = "super_admin"
	DeveloperPermission  = "developer"
	AdminPermission      = "admin"
	UserPermission       = "user"
	GuestPermission      = "guest"
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
func CheckPermission(userRoleIDs []string, userID string, adminRoleIDs, userRoleIDsConfig, developerUserIDs, superAdminRoleIDs []string) string {
	if contains(developerUserIDs, userID) {
		return DeveloperPermission
	}

	// Super Admin check
	for _, groupID := range userRoleIDs {
		if contains(superAdminRoleIDs, groupID) {
			return SuperAdminPermission
		}
	}

	// Admin check
	for _, groupID := range userRoleIDs {
		if contains(adminRoleIDs, groupID) {
			return AdminPermission
		}
	}

	// User check
	for _, groupID := range userRoleIDs {
		if contains(userRoleIDsConfig, groupID) {
			return UserPermission
		}
	}

	return GuestPermission
}
