package thingscloud

import "fmt"

const maxDebugLogBytes = 16 * 1024

func formatDebugLogBody(bs []byte) string {
	if len(bs) <= maxDebugLogBytes {
		return string(bs)
	}
	return fmt.Sprintf("%s\n...[truncated %d bytes]", string(bs[:maxDebugLogBytes]), len(bs)-maxDebugLogBytes)
}
