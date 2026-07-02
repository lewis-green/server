package messages

import "strings"

// serviceErrorTokens identify send failures caused by the sending device's own
// cellular service being unavailable (SIM has no service, radio off) rather than
// a problem with the recipient. These are worth routing around by cooling the
// device down, since a different device may still deliver.
var serviceErrorTokens = []string{
	"RESULT_ERROR_NO_SERVICE",
	"RESULT_ERROR_RADIO_OFF",
}

// isServiceError reports whether a recipient error string indicates the device
// itself has no cellular service.
func isServiceError(err *string) bool {
	if err == nil {
		return false
	}

	for _, token := range serviceErrorTokens {
		if strings.Contains(*err, token) {
			return true
		}
	}

	return false
}
