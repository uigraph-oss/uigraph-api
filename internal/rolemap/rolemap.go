// Package rolemap resolves an org role from IdP claims or attributes using an
// auth provider's ordered mapping rules. OIDC claims and SAML attributes are
// both normalised to map[string]any before they reach this package, so one
// engine serves both.
package rolemap

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/uigraph/app/internal/authz"
)

// Rule is one mapping condition. Rules belong to a single auth provider and are
// evaluated in ascending Priority; the first match decides the role.
type Rule struct {
	ID             string     `json:"id"`
	ProviderID     string     `json:"providerId"`
	Priority       int        `json:"priority"`
	AttributeKey   string     `json:"attributeKey"`
	Operator       string     `json:"operator"`
	AttributeValue string     `json:"attributeValue"`
	Role           authz.Role `json:"role"`
}

const (
	OpEquals      = "equals"
	OpNotEquals   = "notEquals"
	OpContains    = "contains"
	OpNotContains = "notContains"
	OpStartsWith  = "startsWith"
	OpEndsWith    = "endsWith"
	OpExists      = "exists"
	OpNotExists   = "notExists"
	OpRegex       = "regex"
)

// ValidOperators is the catalog served by the API so the admin rule builder
// populates its dropdown from the server instead of hardcoding the list.
var ValidOperators = []string{
	OpEquals, OpNotEquals,
	OpContains, OpNotContains,
	OpStartsWith, OpEndsWith,
	OpExists, OpNotExists,
	OpRegex,
}

func ValidOperator(op string) bool {
	for _, v := range ValidOperators {
		if v == op {
			return true
		}
	}
	return false
}

// TakesValue reports whether an operator reads Rule.AttributeValue. The
// existence operators do not; every other operator requires a value.
func TakesValue(op string) bool {
	if op == OpExists || op == OpNotExists {
		return false
	}
	return true
}

// ValidateRule checks a rule before it is persisted.
func ValidateRule(r Rule) error {
	if strings.TrimSpace(r.AttributeKey) == "" {
		return fmt.Errorf("rolemap: attribute key is required")
	}
	if !ValidOperator(r.Operator) {
		return fmt.Errorf("rolemap: unknown operator %q", r.Operator)
	}
	if TakesValue(r.Operator) && r.AttributeValue == "" {
		return fmt.Errorf("rolemap: operator %q requires a value", r.Operator)
	}
	if r.Operator == OpRegex {
		if _, err := regexp.Compile(r.AttributeValue); err != nil {
			return fmt.Errorf("rolemap: invalid regex %q: %w", r.AttributeValue, err)
		}
	}
	if r.Role != authz.RoleAdmin && r.Role != authz.RoleEditor && r.Role != authz.RoleViewer {
		return fmt.Errorf("rolemap: unknown role %q", r.Role)
	}
	return nil
}

// Resolve returns the role of the first matching rule in ascending priority
// order, or defaultRole when no rule matches.
//
// An unknown operator is an error rather than a silent non-match, so a
// malformed rule cannot quietly downgrade a user to the default role.
func Resolve(rules []Rule, attrs map[string]any, defaultRole authz.Role) (authz.Role, error) {
	ordered := make([]Rule, len(rules))
	copy(ordered, rules)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Priority < ordered[j].Priority })

	for _, r := range ordered {
		matched, err := Match(r, attrs)
		if err != nil {
			return "", err
		}
		if matched {
			return r.Role, nil
		}
	}
	return defaultRole, nil
}

// Match reports whether attrs satisfies r.
//
// The negative operators (notEquals, notContains) additionally require the
// attribute to be present: a claim the IdP never sent does not satisfy them, so
// a missing claim cannot grant a role by omission.
func Match(r Rule, attrs map[string]any) (bool, error) {
	raw := Lookup(attrs, r.AttributeKey)
	present := raw != nil
	values := stringValues(raw)

	switch r.Operator {
	case OpExists:
		return present, nil

	case OpNotExists:
		return !present, nil

	case OpEquals:
		for _, v := range values {
			if v == r.AttributeValue {
				return true, nil
			}
		}
		return false, nil

	case OpNotEquals:
		if !present {
			return false, nil
		}
		for _, v := range values {
			if v == r.AttributeValue {
				return false, nil
			}
		}
		return true, nil

	case OpContains:
		for _, v := range values {
			if strings.Contains(v, r.AttributeValue) {
				return true, nil
			}
		}
		return false, nil

	case OpNotContains:
		if !present {
			return false, nil
		}
		for _, v := range values {
			if strings.Contains(v, r.AttributeValue) {
				return false, nil
			}
		}
		return true, nil

	case OpStartsWith:
		for _, v := range values {
			if strings.HasPrefix(v, r.AttributeValue) {
				return true, nil
			}
		}
		return false, nil

	case OpEndsWith:
		for _, v := range values {
			if strings.HasSuffix(v, r.AttributeValue) {
				return true, nil
			}
		}
		return false, nil

	case OpRegex:
		re, err := regexp.Compile(r.AttributeValue)
		if err != nil {
			return false, fmt.Errorf("rolemap: rule %s: invalid regex %q: %w", r.ID, r.AttributeValue, err)
		}
		for _, v := range values {
			if re.MatchString(v) {
				return true, nil
			}
		}
		return false, nil

	default:
		return false, fmt.Errorf("rolemap: rule %s: unknown operator %q", r.ID, r.Operator)
	}
}

// Lookup resolves an attribute key against the claims.
//
// The whole key is tried as a literal name first, and only when no attribute
// carries that name is it split on "." and traversed as a path into nested
// maps, so a rule can still address a nested claim such as "user.role". The
// literal attempt is what makes SAML usable at all: every claim Entra, Okta and
// ADFS emit is named with a URI such as
// "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups", and
// splitting that on its first dot looks for a claim named "http://schemas".
//
// Returns nil when the key is neither a literal attribute nor a resolvable path.
func Lookup(attrs map[string]any, key string) any {
	if val, ok := attrs[key]; ok {
		return val
	}

	parts := strings.SplitN(key, ".", 2)
	val, ok := attrs[parts[0]]
	if !ok {
		return nil
	}
	if len(parts) == 1 {
		return val
	}
	nested, ok := val.(map[string]any)
	if !ok {
		return nil
	}
	return Lookup(nested, parts[1])
}

// stringValues flattens a claim into comparable strings. A claim may arrive as
// a scalar ("role": "admin"), an array ("groups": ["eng", "sre"]) or a
// non-string scalar ("email_verified": true), and rules compare against all of
// them uniformly.
func stringValues(raw any) []string {
	switch v := raw.(type) {
	case nil:
		return nil
	case string:
		return []string{v}
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if item == nil {
				continue
			}
			if s, ok := item.(string); ok {
				out = append(out, s)
				continue
			}
			out = append(out, fmt.Sprintf("%v", item))
		}
		return out
	default:
		return []string{fmt.Sprintf("%v", v)}
	}
}
