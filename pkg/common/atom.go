package common

func AtomStatus(status string) string {
	switch status {
	case "Cancelled", "Deadline", "Error", "Rollback":
		return "rollback"
	case "Failure", "Reverted", "Running", "":
		return "running"
	case "Pending", "Updating":
		return "updating"
	case "Success", "Failed": // legacy
		return "running"
	default:
		return "unknown"
	}
}
