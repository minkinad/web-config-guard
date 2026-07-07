package guard

import (
	"fmt"
	"net"
	"strings"
)

type Rule interface {
	ID() string
	Check(value any) []Problem
}

const (
	RuleDebugLogging      = "debug-logging"
	RulePlaintextPassword = "plaintext-password"
	RuleWildcardBind      = "wildcard-bind"
	RuleTLSDisabled       = "tls-disabled"
	RuleWeakAlgorithm     = "weak-algorithm"
	RuleFilePermissions   = "file-permissions"
)

func DefaultRules() []Rule {
	return []Rule{
		DebugLoggingRule{},
		PlaintextPasswordRule{},
		WildcardBindRule{},
		TLSDisabledRule{},
		WeakAlgorithmRule{},
	}
}

type DebugLoggingRule struct{}

func (DebugLoggingRule) ID() string { return RuleDebugLogging }

func (rule DebugLoggingRule) Check(value any) []Problem {
	return collectProblems(value, func(node Node) (Problem, bool) {
		key := normalizeName(node.Key)
		switch typed := node.Value.(type) {
		case string:
			if strings.EqualFold(strings.TrimSpace(typed), "debug") && (key == "level" || containsPathPart(node.Path, "log", "logging", "logger")) {
				return newRuleProblem(
					rule,
					SeverityLow,
					node.Path,
					"логирование в debug-режиме",
					"Поменяйте уровень логирования на info или выше для production-окружений",
				), true
			}
		case bool:
			if typed && (key == "debug" || key == "debugmode") {
				return newRuleProblem(
					rule,
					SeverityLow,
					node.Path,
					"включен debug-режим",
					"Отключите debug-режим в production-конфигурации",
				), true
			}
		}
		return Problem{}, false
	})
}

type PlaintextPasswordRule struct{}

func (PlaintextPasswordRule) ID() string { return RulePlaintextPassword }

func (rule PlaintextPasswordRule) Check(value any) []Problem {
	return collectProblems(value, func(node Node) (Problem, bool) {
		key := normalizeName(node.Key)
		if !isPasswordKey(key) || isPasswordReferenceKey(key) {
			return Problem{}, false
		}

		password, ok := node.Value.(string)
		if !ok || strings.TrimSpace(password) == "" || looksLikeSecretReference(password) {
			return Problem{}, false
		}

		return newRuleProblem(
			rule,
			SeverityHigh,
			node.Path,
			"пароль задан в конфигурации открытым текстом",
			"Передавайте пароль через переменные окружения, secret manager или внешний файл с ограниченными правами",
		), true
	})
}

type WildcardBindRule struct{}

func (WildcardBindRule) ID() string { return RuleWildcardBind }

func (rule WildcardBindRule) Check(value any) []Problem {
	if hasAccessRestriction(value) {
		return nil
	}

	return collectProblems(value, func(node Node) (Problem, bool) {
		address, ok := node.Value.(string)
		if !ok {
			return Problem{}, false
		}

		if !isWildcardBindAddress(address) {
			return Problem{}, false
		}

		if !isBindAddressKey(node.Key) {
			return Problem{}, false
		}

		return newRuleProblem(
			rule,
			SeverityMedium,
			node.Path,
			"сервис слушает 0.0.0.0 без явных ограничений доступа",
			"Ограничьте адрес прослушивания, добавьте allowlist/firewall или явно настройте trusted networks",
		), true
	})
}

type TLSDisabledRule struct{}

func (TLSDisabledRule) ID() string { return RuleTLSDisabled }

func (rule TLSDisabledRule) Check(value any) []Problem {
	return collectProblems(value, func(node Node) (Problem, bool) {
		key := normalizeName(node.Key)

		disabled := false
		switch typed := node.Value.(type) {
		case bool:
			disabled = isTLSDisabledKey(key, typed)
		case string:
			disabled = isTLSDisabledString(key, typed)
		}

		if disabled {
			return newRuleProblem(
				rule,
				SeverityHigh,
				node.Path,
				"отключена TLS-проверка или HTTPS/TLS",
				"Включите TLS и проверку сертификатов; не используйте insecure_skip_verify в production",
			), true
		}
		return Problem{}, false
	})
}

type WeakAlgorithmRule struct{}

func (WeakAlgorithmRule) ID() string { return RuleWeakAlgorithm }

func (rule WeakAlgorithmRule) Check(value any) []Problem {
	return collectProblems(value, func(node Node) (Problem, bool) {
		algorithm, ok := node.Value.(string)
		if !ok || !isAlgorithmContext(node.Path, node.Key) {
			return Problem{}, false
		}

		display, weak := weakAlgorithmName(algorithm)
		if !weak {
			return Problem{}, false
		}

		return newRuleProblem(
			rule,
			SeverityHigh,
			node.Path,
			fmt.Sprintf("используется устаревший или небезопасный алгоритм %s", display),
			"Замените алгоритм на современный вариант, например SHA-256/Argon2/bcrypt или актуальный TLS cipher suite по назначению",
		), true
	})
}

func collectProblems(value any, evaluate func(Node) (Problem, bool)) []Problem {
	var problems []Problem
	Walk(value, func(node Node) {
		problem, ok := evaluate(node)
		if ok {
			problems = append(problems, problem)
		}
	})
	return problems
}

func newRuleProblem(rule Rule, severity Severity, path []string, message, recommendation string) Problem {
	return Problem{
		Severity:       severity,
		Rule:           rule.ID(),
		Path:           FormatPath(path),
		Message:        message,
		Recommendation: recommendation,
	}
}

func normalizeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("-", "", "_", "", ".", "", " ", "")
	return replacer.Replace(value)
}

func containsPathPart(path []string, candidates ...string) bool {
	for _, part := range path {
		normalized := normalizeName(part)
		for _, candidate := range candidates {
			if strings.Contains(normalized, normalizeName(candidate)) {
				return true
			}
		}
	}
	return false
}

func isPasswordKey(key string) bool {
	return key == "password" || key == "passwd" || key == "pwd" || strings.HasSuffix(key, "password")
}

func isPasswordReferenceKey(key string) bool {
	return strings.Contains(key, "file") ||
		strings.Contains(key, "path") ||
		strings.Contains(key, "env") ||
		strings.Contains(key, "secretref") ||
		strings.Contains(key, "secretname")
}

func looksLikeSecretReference(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return true
	}

	referencePrefixes := []string{"$", "${", "env:", "vault:", "secret:", "secretsmanager:", "arn:", "gcp-secret:", "azure-keyvault:"}
	for _, prefix := range referencePrefixes {
		if strings.HasPrefix(normalized, prefix) {
			return true
		}
	}

	return strings.Contains(normalized, "{{") ||
		strings.Contains(normalized, "}}") ||
		strings.Contains(normalized, "****") ||
		normalized == "<redacted>"
}

func hasAccessRestriction(value any) bool {
	found := false
	Walk(value, func(node Node) {
		if found {
			return
		}
		key := normalizeName(node.Key)
		restrictionKeys := []string{
			"allowlist", "whitelist", "allowedips", "allowedcidrs", "trustednetworks",
			"firewall", "acl", "accesscontrol", "networkpolicy", "cidr",
		}
		for _, candidate := range restrictionKeys {
			if strings.Contains(key, candidate) {
				found = true
				return
			}
		}
	})
	return found
}

func isBindAddressKey(key string) bool {
	switch normalizeName(key) {
	case "host", "bind", "bindaddress", "listen", "address":
		return true
	default:
		return false
	}
}

func isWildcardBindAddress(value string) bool {
	address := strings.TrimSpace(value)
	if address == "0.0.0.0" || address == "::" || address == "[::]" {
		return true
	}

	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	host = strings.Trim(host, "[]")
	return host == "0.0.0.0" || host == "::"
}

func isTLSDisabledKey(key string, value bool) bool {
	switch {
	case (key == "tls" || key == "ssl" || key == "https" || key == "tlsenabled" || key == "sslenabled" || key == "httpsenabled") && !value:
		return true
	case strings.Contains(key, "insecure") && strings.Contains(key, "skip") && value:
		return true
	case strings.Contains(key, "skip") && strings.Contains(key, "verify") && value:
		return true
	case strings.Contains(key, "verify") && (strings.Contains(key, "tls") || strings.Contains(key, "ssl") || strings.Contains(key, "cert")) && !value:
		return true
	default:
		return false
	}
}

func isTLSDisabledString(key string, value string) bool {
	normalizedValue := normalizeName(value)
	falseLike := normalizedValue == "false" ||
		normalizedValue == "no" ||
		normalizedValue == "off" ||
		normalizedValue == "disabled" ||
		normalizedValue == "disable" ||
		normalizedValue == "0"
	trueLike := normalizedValue == "true" ||
		normalizedValue == "yes" ||
		normalizedValue == "on" ||
		normalizedValue == "enabled" ||
		normalizedValue == "enable" ||
		normalizedValue == "1"

	switch {
	case falseLike && (key == "tls" || key == "ssl" || key == "https" || key == "tlsenabled" || key == "sslenabled" || key == "httpsenabled"):
		return true
	case trueLike && strings.Contains(key, "insecure") && strings.Contains(key, "skip"):
		return true
	case trueLike && strings.Contains(key, "skip") && strings.Contains(key, "verify"):
		return true
	case falseLike && strings.Contains(key, "verify") && (strings.Contains(key, "tls") || strings.Contains(key, "ssl") || strings.Contains(key, "cert")):
		return true
	case key == "sslmode" && (normalizedValue == "disable" || normalizedValue == "disabled"):
		return true
	default:
		return false
	}
}

func isAlgorithmContext(path []string, key string) bool {
	key = normalizeName(key)
	if strings.Contains(key, "algorithm") || strings.Contains(key, "cipher") || strings.Contains(key, "digest") || strings.Contains(key, "hash") {
		return true
	}
	return containsPathPart(path, "crypto", "cipher", "digest", "hash", "algorithm")
}

func weakAlgorithmName(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	normalized := strings.ToLower(trimmed)
	normalized = strings.NewReplacer("-", "", "_", "", " ", "").Replace(normalized)

	weak := map[string]string{
		"md5":       "MD5",
		"sha1":      "SHA-1",
		"des":       "DES",
		"3des":      "3DES",
		"tripledes": "3DES",
		"rc4":       "RC4",
		"blowfish":  "Blowfish",
		"none":      "none",
		"plaintext": "plaintext",
		"crc":       "CRC",
		"crc32":     "CRC32",
	}
	display, ok := weak[normalized]
	if ok {
		return display, true
	}
	return trimmed, false
}
