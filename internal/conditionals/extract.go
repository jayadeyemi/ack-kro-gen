package conditionals

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// Match {{- if CONDITION }} ... {{- end }} or {{ if CONDITION }} ... {{ end }}
	ifBlockRegex = regexp.MustCompile(`(?s)\{\{-?\s*if\s+(.+?)\s*-?\}\}(.*?)\{\{-?\s*end\s*-?\}\}`)

	// Match comparison operators - must capture the whole expression including parentheses
	// "gt (int .Values.x) 5" should match with group 1="int .Values.x" and group 2="5"
	gtRegex = regexp.MustCompile(`gt\s+\(([^)]+)\)\s+(\d+)`)
	ltRegex = regexp.MustCompile(`lt\s+\(([^)]+)\)\s+(\d+)`)
	eqRegex = regexp.MustCompile(`eq\s+([^\s]+)\s+"([^"]*)"`)
	neRegex = regexp.MustCompile(`ne\s+([^\s]+)\s+"([^"]*)"`)

	// Match int conversion within parentheses
	intConvRegex = regexp.MustCompile(`\(int\s+([^)]+)\)`)

	// Match logical operators - "or OPERAND1 OPERAND2" or "and OPERAND1 OPERAND2"
	orRegex  = regexp.MustCompile(`\bor\s+(\S+)\s+(\S+)`)
	andRegex = regexp.MustCompile(`\band\s+(\S+)\s+(\S+)`)
)

// Condition represents a conditional block from Helm templates
type Condition struct {
	HelmExpr   string // The Go template condition (e.g., ".Values.serviceAccount.create")
	CELExpr    string // Converted CEL expression (e.g., "schema.spec.serviceAccount.create")
	Block      string // The content inside the if block
	ResourceID string // e.g., "ServiceAccount", "Role"
	FilePath   string // Template file path
}

// ExtractConditions finds all conditional blocks in Helm template
func ExtractConditions(templatePath, content string) []Condition {
	var conditions []Condition

	matches := ifBlockRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		helmExpr := strings.TrimSpace(match[1])
		block := match[2]

		cond := Condition{
			HelmExpr:   helmExpr,
			CELExpr:    ConvertToCEL(helmExpr),
			Block:      block,
			ResourceID: extractResourceKind(block),
			FilePath:   templatePath,
		}
		conditions = append(conditions, cond)
	}

	return conditions
}

// ConvertToCEL translates Helm template expressions to CEL
func ConvertToCEL(helmExpr string) string {
	expr := helmExpr

	// Step 1: Handle gt/lt comparison operators with int conversions
	// "gt (int .Values.x) 5" -> "int .Values.x > 5"
	expr = gtRegex.ReplaceAllStringFunc(expr, func(match string) string {
		parts := gtRegex.FindStringSubmatch(match)
		if len(parts) == 3 {
			return parts[1] + " > " + parts[2]
		}
		return match
	})

	expr = ltRegex.ReplaceAllStringFunc(expr, func(match string) string {
		parts := ltRegex.FindStringSubmatch(match)
		if len(parts) == 3 {
			return parts[1] + " < " + parts[2]
		}
		return match
	})

	// Step 2: Handle eq/ne operators
	expr = eqRegex.ReplaceAllString(expr, `$1 == "$2"`)
	expr = neRegex.ReplaceAllString(expr, `$1 != "$2"`)

	// Step 3: Handle int() conversions - "(int .Values.foo)" -> "int .Values.foo" (temporarily)
	expr = intConvRegex.ReplaceAllString(expr, "int $1")

	// Step 4: Handle logical operators (or/and -> ||/&&)
	expr = replaceLogicalOperators(expr)

	// Step 5: Replace .Values with schema.spec.values
	expr = strings.ReplaceAll(expr, ".Values.", "schema.spec.values.")

	// Step 6: Fix int conversion - add parentheses back: "int schema.spec..." -> "int(schema.spec...)"
	intFixRegex := regexp.MustCompile(`int\s+(schema\.\S+?)(\s|>|<|==|!=|$|\|)`)
	expr = intFixRegex.ReplaceAllString(expr, "int($1)$2")

	// Step 7: Handle not operator
	expr = strings.ReplaceAll(expr, " not ", " !")

	// Step 8: Clean up extra spaces
	expr = strings.Join(strings.Fields(expr), " ")

	return expr
}

// replaceLogicalOperators handles 'or' and 'and' operators with proper operand ordering
// Go templates use prefix notation: "or OPERAND1 OPERAND2"
// CEL uses infix notation: "OPERAND1 || OPERAND2"
func replaceLogicalOperators(expr string) string {
	// Handle 'or' - convert "or OPERAND1 OPERAND2" to "OPERAND1 || OPERAND2"
	expr = orRegex.ReplaceAllString(expr, "$1 || $2")
	// Handle 'and' - convert "and OPERAND1 OPERAND2" to "OPERAND1 && OPERAND2"
	expr = andRegex.ReplaceAllString(expr, "$1 && $2")
	return expr
}

// extractResourceKind pulls the Kind from a YAML block
func extractResourceKind(block string) string {
	kindRegex := regexp.MustCompile(`kind:\s*(\w+)`)
	matches := kindRegex.FindStringSubmatch(block)
	if len(matches) > 1 {
		return matches[1]
	}
	return "Unknown"
}

// MatchConditionToResource checks if a condition applies to a given resource
func MatchConditionToResource(cond Condition, resourceKind string, resourceYAML string) bool {
	// Direct kind match
	if cond.ResourceID == resourceKind {
		return true
	}

	// If condition block contains this resource's YAML (fuzzy match)
	if strings.Contains(cond.Block, fmt.Sprintf("kind: %s", resourceKind)) {
		return true
	}

	return false
}

// SimplifyCondition attempts to simplify complex CEL expressions
func SimplifyCondition(celExpr string) string {
	// Remove redundant parentheses
	celExpr = strings.TrimSpace(celExpr)

	// Simplify double negations
	celExpr = strings.ReplaceAll(celExpr, "! !", "")

	// Simplify "== true" to just the condition
	celExpr = strings.ReplaceAll(celExpr, " == true", "")

	// Simplify "!= false" to just the condition
	celExpr = strings.ReplaceAll(celExpr, " != false", "")

	return strings.TrimSpace(celExpr)
}
