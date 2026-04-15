package runtime

import (
	"regexp"
	"strings"
)

// ClassifierAction represents the decision made by the classifier
type ClassifierAction string

const (
	ActionAllow ClassifierAction = "allow"
	ActionDeny  ClassifierAction = "deny"
	ActionAsk   ClassifierAction = "ask"
)

// ClassifierRule defines a single classification rule
type ClassifierRule struct {
	Pattern     string           // regex pattern to match
	Action      ClassifierAction // allow, deny, ask
	Category    string           // tool/command category
	Description string           // human-readable description
	Source      string           // "default", "user", "project"
}

// Classifier categorizes tool/command invocations for auto mode
type Classifier struct {
	denyRules  []ClassifierRule
	allowRules []ClassifierRule
	userRules  []ClassifierRule
}

// ClassifierResult holds the classification decision
type ClassifierResult struct {
	Action      ClassifierAction
	MatchedRule *ClassifierRule
}

// NewClassifier creates a new command classifier with built-in rules
func NewClassifier() *Classifier {
	c := &Classifier{}
	c.denyRules = defaultDenyRules()
	c.allowRules = defaultAllowRules()
	return c
}

// SetUserRules sets custom user-defined rules (highest priority)
func (c *Classifier) SetUserRules(rules []ClassifierRule) {
	c.userRules = rules
}

// Classify determines the action for a given tool and command
func (c *Classifier) Classify(toolName, command string) ClassifierResult {
	// 1. Check user rules first (highest priority)
	for i := range c.userRules {
		if matchRule(&c.userRules[i], toolName, command) {
			return ClassifierResult{Action: c.userRules[i].Action, MatchedRule: &c.userRules[i]}
		}
	}

	// 2. Check deny rules (dangerous commands always prompt)
	for i := range c.denyRules {
		if matchRule(&c.denyRules[i], toolName, command) {
			return ClassifierResult{Action: ActionDeny, MatchedRule: &c.denyRules[i]}
		}
	}

	// 3. Check allow rules (safe commands auto-allowed)
	for i := range c.allowRules {
		if matchRule(&c.allowRules[i], toolName, command) {
			return ClassifierResult{Action: ActionAllow, MatchedRule: &c.allowRules[i]}
		}
	}

	// 4. Default: ask user
	return ClassifierResult{Action: ActionAsk}
}

func matchRule(rule *ClassifierRule, toolName, command string) bool {
	pattern := rule.Pattern

	// Match against tool name
	if strings.EqualFold(pattern, toolName) {
		return true
	}

	// Match against command content using regex
	if command != "" {
		re, err := regexp.Compile("(?i)" + pattern)
		if err == nil && re.MatchString(command) {
			return true
		}
	}

	return false
}
