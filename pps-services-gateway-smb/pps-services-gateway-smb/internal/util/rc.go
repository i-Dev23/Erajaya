package util

func ResolveRCPPS(smbResponseCode string) int {
	switch smbResponseCode {
	case "00":
		return 0
	case "28", "68":
		return 9
	case "":
		return 9
	default:
		return 1
	}
}

func StatusToBeFromRC(rcPPS int) string {
	switch rcPPS {
	case 0:
		return "F"
	case 1:
		return "C"
	case 9:
		return "S"
	default:
		return "C"
	}
}
