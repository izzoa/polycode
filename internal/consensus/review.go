package consensus

import (
	"regexp"
	"strings"
)

// ConsensusAnalysis holds the structured breakdown of a consensus synthesis.
type ConsensusAnalysis struct {
	Recommendation  string
	Confidence      string // "high", "medium", "low", or ""
	Agreements      []string
	MinorityReports []MinorityReport
	Evidence        []string
	Raw             string // the full original synthesis text
}

// MinorityReport captures a dissenting view from the consensus.
type MinorityReport struct {
	ProviderID string
	Position   string
	Reasoning  string
}

// confidenceRe matches high, medium, or low (case-insensitive) after
// the Confidence header.
var confidenceRe = regexp.MustCompile(`(?i)\b(high|medium|low)\b`)

// modelRe matches "[Model: name]" patterns in minority report text.
var modelRe = regexp.MustCompile(`\[Model:\s*([^\]]+)\]`)

// ParseConsensusAnalysis extracts structured sections from the primary model's
// synthesis output. If the output does not contain structured headers (## ),
// the full text is returned as both Recommendation and Raw (graceful degradation).
func ParseConsensusAnalysis(rawOutput string) *ConsensusAnalysis {
	ca := &ConsensusAnalysis{
		Raw: rawOutput,
	}

	// Split the output by ## headers.
	sections := splitSections(rawOutput)

	// Graceful degradation: no structured headers found.
	if len(sections) <= 1 && !strings.Contains(rawOutput, "## ") {
		ca.Recommendation = rawOutput
		return ca
	}

	for _, sec := range sections {
		name := strings.TrimSpace(sec.name)
		body := strings.TrimSpace(sec.body)

		switch {
		case name == "":
			// Text before the first header contributes to Recommendation.
			if body != "" {
				ca.Recommendation = body
			}

		case strings.HasPrefix(strings.ToLower(name), "recommendation"):
			ca.Recommendation = body

		case strings.HasPrefix(strings.ToLower(name), "confidence"):
			// The confidence level may appear on the header line itself
			// (e.g. "## Confidence: high") or in the body.
			combined := name + " " + body
			if m := confidenceRe.FindString(combined); m != "" {
				ca.Confidence = strings.ToLower(m)
			}

		case strings.HasPrefix(strings.ToLower(name), "agreement"):
			ca.Agreements = parseLines(body)

		case strings.HasPrefix(strings.ToLower(name), "minority report"):
			ca.MinorityReports = parseMinorityReports(body)

		case strings.HasPrefix(strings.ToLower(name), "evidence"):
			ca.Evidence = parseLines(body)
		}
	}

	return ca
}

// section is a parsed ## section from the synthesis output.
type section struct {
	name string // header text after "## " (empty for preamble)
	body string // body text below the header
}

// splitSections splits raw text by "## " header markers. The first element
// (if any text precedes the first header) has an empty name.
func splitSections(text string) []section {
	var sections []section
	lines := strings.Split(text, "\n")
	current := section{}
	var bodyLines []string

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			// Flush the previous section.
			current.body = strings.Join(bodyLines, "\n")
			sections = append(sections, current)
			current = section{name: strings.TrimPrefix(line, "## ")}
			bodyLines = nil
		} else {
			bodyLines = append(bodyLines, line)
		}
	}

	// Flush last section.
	current.body = strings.Join(bodyLines, "\n")
	sections = append(sections, current)

	return sections
}

// parseLines splits a body into non-empty, trimmed lines. Bullet markers
// (- or *) are stripped.
func parseLines(body string) []string {
	if strings.TrimSpace(body) == "" {
		return nil
	}

	var out []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		// Strip leading bullet markers.
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseMinorityReports attempts to extract structured minority reports from the
// Minority Report section body. It looks for [Model: name] markers. If none
// are found, the entire body is returned as a single MinorityReport.
func parseMinorityReports(body string) []MinorityReport {
	if strings.TrimSpace(body) == "" {
		return nil
	}

	// Check for "None" indicators.
	lower := strings.ToLower(strings.TrimSpace(body))
	if strings.HasPrefix(lower, "none") {
		return nil
	}

	matches := modelRe.FindAllStringSubmatchIndex(body, -1)
	if len(matches) == 0 {
		// Fallback: return the whole body as a single report.
		return []MinorityReport{
			{
				Position: strings.TrimSpace(body),
			},
		}
	}

	var reports []MinorityReport
	for i, match := range matches {
		providerID := strings.TrimSpace(body[match[2]:match[3]])

		// The text for this model runs from after the [Model: ...] tag
		// to the start of the next match (or end of string).
		start := match[1]
		end := len(body)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}

		text := strings.TrimSpace(body[start:end])
		// Try to split into position and reasoning on sentence/line boundaries,
		// but for simplicity store the whole thing as Position.
		reports = append(reports, MinorityReport{
			ProviderID: providerID,
			Position:   text,
		})
	}

	return reports
}
