package main

// Helper function to check if source IP should be used
func shouldUseSourceIP(src string) bool {
	return src != "" && src != "0.0.0.0"
}

func getSourceDisplay(src string) string {
	if shouldUseSourceIP(src) {
		return src
	}
	return "auto"
}

func getSourceKey(src string) string {
	if shouldUseSourceIP(src) {
		return src
	}
	return "auto"
}
