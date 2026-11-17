package commands

import "testing"

func TestRuleArgs(t *testing.T) {
	if got := ruleArgs("shop a.com/*=web:80 b.com/=api:81"); got != "shop --rule=a.com/*=web:80 --rule=b.com/=api:81" {
		t.Errorf("ruleArgs = %q", got)
	}
}
