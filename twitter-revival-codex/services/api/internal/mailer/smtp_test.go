package mailer

import "testing"

func TestSMTPConfiguration(t *testing.T) {
	if _, err := NewSMTP("127.0.0.1:1025", "Radura <no-reply@radura.test>", "", "", false); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		addr, from, user string
		tls              bool
	}{{"invalid", "a@example.test", "", false}, {"host:0", "a@example.test", "", false}, {"host:1025", "a@example.test\r\nBcc: hidden@example.test", "", false}, {"host:1025", "a@example.test", "user", false}} {
		if _, err := NewSMTP(tc.addr, tc.from, tc.user, "secret", tc.tls); err == nil {
			t.Fatal("unsafe SMTP configuration accepted")
		}
	}
}
