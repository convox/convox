package common

func AtomStatus(status string) string {
	switch status {
	case "Failed":
		return "running"
	case "Rollback":
		return "rollback"
	case "Deadline", "Error", "Pending", "Running":
		return "updating"
	default:
		return "running"
	}
}
