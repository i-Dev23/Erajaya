package util

// ResolveRCPPS memetakan response code SMB ke RC PPS.
// Returns: 0 = success, 1 = failed, 9 = pending/retry.
func ResolveRCPPS(smbResponseCode string) int {
	switch smbResponseCode {
	case "00":
		return 0 // Success
	case "28", "68":
		return 9 // Pending/Timeout — perlu retry advice
	case "":
		return 9 // Empty response — perlu retry advice
	default:
		return 1 // Failed
	}
}

// StatusToBeFromRC mengkonversi RC PPS ke status_to_be untuk downstream consumer.
// F = Final success, C = Cancel/failed, S = Still processing.
func StatusToBeFromRC(rcPPS int) string {
	switch rcPPS {
	case 0:
		return "F" // Final success
	case 1:
		return "C" // Cancel/failed
	case 9:
		return "S" // Still processing
	default:
		return "C"
	}
}
