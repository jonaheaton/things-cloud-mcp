package thingscloud

import "testing"

func TestAccountPathEscapesEmail(t *testing.T) {
	got := accountPath("user/name@example.com")
	want := "/version/1/account/user%2Fname@example.com"
	if got != want {
		t.Fatalf("accountPath = %q, want %q", got, want)
	}
}
