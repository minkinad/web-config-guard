package guard

import (
	"io/fs"
	"os"
	"sort"
)

type Checker struct {
	Rules []Rule
}

func NewChecker(rules []Rule) Checker {
	if len(rules) == 0 {
		rules = DefaultRules()
	}
	return Checker{Rules: rules}
}

func (checker Checker) Check(value any) []Problem {
	var problems []Problem
	for _, rule := range checker.Rules {
		problems = append(problems, rule.Check(value)...)
	}
	sortProblems(problems)
	return problems
}

func CheckFilePermissions(path string, info fs.FileInfo, hasSecrets bool) []Problem {
	if info == nil {
		var err error
		info, err = os.Stat(path)
		if err != nil {
			return []Problem{{
				Severity:       SeverityHigh,
				Rule:           "file-permissions",
				File:           path,
				Message:        "не удалось проверить права доступа к файлу",
				Recommendation: "Проверьте существование файла и права пользователя, запускающего утилиту",
			}}
		}
	}

	mode := info.Mode().Perm()
	var problems []Problem
	if mode&0o002 != 0 {
		problems = append(problems, Problem{
			Severity:       SeverityHigh,
			Rule:           "file-permissions",
			File:           path,
			Message:        "конфигурационный файл доступен на запись всем пользователям",
			Recommendation: "Ограничьте права доступа, например chmod 600 или chmod 640",
		})
	}
	if mode&0o020 != 0 {
		problems = append(problems, Problem{
			Severity:       SeverityMedium,
			Rule:           "file-permissions",
			File:           path,
			Message:        "конфигурационный файл доступен на запись группе",
			Recommendation: "Разрешайте запись только владельцу файла, если это не требуется явно",
		})
	}
	if hasSecrets && mode&0o044 != 0 {
		problems = append(problems, Problem{
			Severity:       SeverityMedium,
			Rule:           "file-permissions",
			File:           path,
			Message:        "файл с секретами доступен для чтения группе или всем пользователям",
			Recommendation: "Для конфигов с секретами используйте более строгие права, например chmod 600",
		})
	}

	sortProblems(problems)
	return problems
}

func HasSecretProblems(problems []Problem) bool {
	for _, problem := range problems {
		if problem.Rule == "plaintext-password" {
			return true
		}
	}
	return false
}

func SortProblems(problems []Problem) {
	sortProblems(problems)
}

func sortProblems(problems []Problem) {
	sort.SliceStable(problems, func(i, j int) bool {
		if severityRank(problems[i].Severity) != severityRank(problems[j].Severity) {
			return severityRank(problems[i].Severity) > severityRank(problems[j].Severity)
		}
		if problems[i].File != problems[j].File {
			return problems[i].File < problems[j].File
		}
		if problems[i].Path != problems[j].Path {
			return problems[i].Path < problems[j].Path
		}
		return problems[i].Rule < problems[j].Rule
	})
}

func severityRank(severity Severity) int {
	switch severity {
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 1
	default:
		return 0
	}
}
