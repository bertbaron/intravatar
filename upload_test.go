package main

import "testing"

func TestVerifyEmail(t *testing.T) {
	emailDomains = []string{}

	if verifyEmail("bill.gates@microsoft.com") != nil {
        t.Errorf("Empty email whitelist allows all addresses.")
   }

	emailDomains = []string{"google.com", "google.de"}
	t.Run("larry.page@google.de",
			func(t *testing.T) {
				mail := t.Name()
				if verifyEmail(t.Name()) != nil {
			        t.Errorf("%s is supposed to be valid", mail)
			   }
			})
	t.Run("steve.jobs@apple.com",
		func(t *testing.T) {
			mail := t.Name()
			if verifyEmail(t.Name()) == nil {
				t.Errorf( "%s is not valid", mail)
			}
		})
}
