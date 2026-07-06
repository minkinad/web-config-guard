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

func (DebugLoggingRule) ID() string { return "debug-logging" }

func (rule DebugLoggingRule) Check(value any) []Problem {
	var problems []Problem
	Walk(value, func(node Node) {
		key := normalizeName(node.Key)
		switch typed := node.Value.(type) {
		case string:
			if strings.EqualFold(strings.TrimSpace(typed), "debug") && (key == "level" || containsPathPart(node.Path, "log", "logging", "logger")) {
				problems = append(problems, Problem{
					Severity:       SeverityLow,
					Rule:           rule.ID(),
					Path:           FormatPath(node.Path),
					Message:        "логирование в debug-режиме",
					Recommendation: "Поменяйте уровень логирования на info или выше для production-окружений",
				})
			}
		case bool:
			if typed && (key == "debug" || key == "debugmode") {
				problems = append(problems, Problem{
					Severity:       SeverityLow,
					Rule:           rule.ID(),
					Path:           FormatPath(node.Path),
					Message:        "включен debug-режим",
					Recommendation: "Отключите debug-режим в production-конфигурации",
				})
			}
		}
	})
	return problems
}

type PlaintextPasswordRule struct{}

func (PlaintextPasswordRule) ID() string { return "plaintext-password" }

func (rule PlaintextPasswordRule) Check(value any) []Problem {
	var problems []Problem
	Walk(value, func(node Node) {
		key := normalizeName(node.Key)
		if !isPasswordKey(key) || isPasswordReferenceKey(key) {
			return
		}

		password, ok := node.Value.(string)
		if !ok || strings.TrimSpace(password) == "" || looksLikeSecretReference(password) {
			return
		}

		problems = append(problems, Problem{
			Severity:       SeverityHigh,
			Rule:           rule.ID(),
			Path:           FormatPath(node.Path),
			Message:        "пароль задан в конфигурации открытым текстом",
			Recommendation: "Передавайте пароль через переменные окружения, secret manager или внешний файл с ограниченными правами",
		})
	})
	return problems
}

type WildcardBindRule struct{}

func (WildcardBindRule) ID() string { return "wildcard-bind" }

func (rule WildcardBindRule) Check(value any) []Problem {
	if hasAccessRestriction(value) {
		return nil
	}

	var problems []Problem
	Walk(value, func(node Node) {
		address, ok := node.Value.(string)
		if !ok {
			return
		}

		if !isWildcardBindAddress(address) {
			return
		}

		key := normalizeName(node.Key)
		if key != "host" && key != "bind" && key != "bindaddress" && key != "listen" && key != "address" {
			return
		}

		problems = append(problems, Problem{
			Severity:       SeverityMedium,
			Rule:           rule.ID(),
			Path:           FormatPath(node.Path),
			Message:        "сервис слушает 0.0.0.0 без явных ограничений доступа",
			Recommendation: "Ограничьте адрес прослушивания, добавьте allowlist/firewall или явно настройте trusted networks",
		})
	})
	return problems
}

type TLSDisabledRule struct{}

func (TLSDisabledRule) ID() string { return "tls-disabled" }

func (rule TLSDisabledRule) Check(value any) []Problem {
	var problems []Problem
	Walk(value, func(node Node) {
		key := normalizeName(node.Key)

		disabled := false
		switch typed := node.Value.(type) {
		case bool:
			disabled = isTLSDisabledKey(key, typed)
		case string:
			disabled = isTLSDisabledString(key, typed)
		}

		if disabled {
			problems = append(problems, Problem{
				Severity:       SeverityHigh,
				Rule:           rule.ID(),
				Path:           FormatPath(node.Path),
				Message:        "отключена TLS-проверка или HTTPS/TLS",
				Recommendation: "Включите TLS и проверку сертификатов; не используйте insecure_skip_verify в production",
			})
		}
	})
	return problems
}

type WeakAlgorithmRule struct{}

func (WeakAlgorithmRule) ID() string { return "weak-algorithm" }

func (rule WeakAlgorithmRule) Check(value any) []Problem {
	var problems []Problem
	Walk(value, func(node Node) {
		algorithm, ok := node.Value.(string)
		if !ok || !isAlgorithmContext(node.Path, node.Key) {
			return
		}

		display, weak := weakAlgorithmName(algorithm)
		if !weak {
			return
		}

		problems = append(problems, Problem{
			Severity:       SeverityHigh,
			Rule:           rule.ID(),
			Path:           FormatPath(node.Path),
			Message:        fmt.Sprintf("используется устаревший или небезопасный алгоритм %s", display),
			Recommendation: "Замените алгоритм на современный вариант, например SHA-256/Argon2/bcrypt или актуальный TLS cipher suite по назначению",
		})
	})
	return problems
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
