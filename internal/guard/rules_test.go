package guard

import "testing"

func TestDefaultRulesFindSecurityProblems(t *testing.T) {
	config := map[string]any{
		"log": map[string]any{
			"level": "debug",
		},
		"database": map[string]any{
			"password": "super-secret",
		},
		"server": map[string]any{
			"host": "0.0.0.0",
			"tls":  false,
		},
		"storage": map[string]any{
			"digest-algorithm": "MD5",
		},
	}

	problems := NewChecker(nil).Check(config)
	assertProblem(t, problems, RuleDebugLogging)
	assertProblem(t, problems, RulePlaintextPassword)
	assertProblem(t, problems, RuleWildcardBind)
	assertProblem(t, problems, RuleTLSDisabled)
	assertProblem(t, problems, RuleWeakAlgorithm)
}

func TestSecretReferencesAreNotPlaintextPasswords(t *testing.T) {
	config := map[string]any{
		"database": map[string]any{
			"password": "${DB_PASSWORD}",
		},
	}

	problems := PlaintextPasswordRule{}.Check(config)
	if len(problems) != 0 {
		t.Fatalf("PlaintextPasswordRule found %d problems, want 0: %#v", len(problems), problems)
	}
}

func TestWildcardBindAllowsExplicitRestrictions(t *testing.T) {
	config := map[string]any{
		"server": map[string]any{
			"host": "0.0.0.0",
		},
		"network": map[string]any{
			"allowed_ips": []any{"10.0.0.0/8"},
		},
	}

	problems := WildcardBindRule{}.Check(config)
	if len(problems) != 0 {
		t.Fatalf("WildcardBindRule found %d problems, want 0: %#v", len(problems), problems)
	}
}

func TestWildcardBindFindsAddressWithPort(t *testing.T) {
	config := map[string]any{
		"server": map[string]any{
			"listen": "0.0.0.0:8080",
		},
	}

	problems := WildcardBindRule{}.Check(config)
	assertProblem(t, problems, RuleWildcardBind)
}

func TestWildcardBindFindsIPv6AnyAddress(t *testing.T) {
	config := map[string]any{
		"server": map[string]any{
			"listen": "[::]:8080",
		},
	}

	problems := WildcardBindRule{}.Check(config)
	assertProblem(t, problems, RuleWildcardBind)
}

func TestTLSDisabledFindsStringValues(t *testing.T) {
	config := map[string]any{
		"database": map[string]any{
			"sslmode": "disable",
		},
		"client": map[string]any{
			"insecure_skip_verify": "true",
		},
	}

	problems := TLSDisabledRule{}.Check(config)
	if len(problems) != 2 {
		t.Fatalf("TLSDisabledRule found %d problems, want 2: %#v", len(problems), problems)
	}
}

func TestCheckFilePermissions(t *testing.T) {
	file := t.TempDir() + "/config.yaml"
	if err := writeTestFile(file, []byte("password: plain\n"), 0o666); err != nil {
		t.Fatal(err)
	}

	problems := CheckFilePermissions(file, nil, true)
	assertProblem(t, problems, RuleFilePermissions)
}

func assertProblem(t *testing.T, problems []Problem, rule string) {
	t.Helper()
	for _, problem := range problems {
		if problem.Rule == rule {
			return
		}
	}
	t.Fatalf("problem with rule %q not found in %#v", rule, problems)
}
