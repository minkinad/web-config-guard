package guard

import "fmt"

type Severity string

const (
	SeverityLow    Severity = "LOW"
	SeverityMedium Severity = "MEDIUM"
	SeverityHigh   Severity = "HIGH"
)

type Problem struct {
	Severity       Severity `json:"severity"`
	Rule           string   `json:"rule"`
	Path           string   `json:"path,omitempty"`
	File           string   `json:"file,omitempty"`
	Message        string   `json:"message"`
	Recommendation string   `json:"recommendation"`
}

func (p Problem) Text() string {
	location := p.Path
	if p.File != "" && p.Path != "" {
		location = fmt.Sprintf("%s:%s", p.File, p.Path)
	} else if p.File != "" {
		location = p.File
	}
	if location == "" {
		return fmt.Sprintf("%s: %s. %s", p.Severity, p.Message, p.Recommendation)
	}
	return fmt.Sprintf("%s [%s]: %s. %s", p.Severity, location, p.Message, p.Recommendation)
}
