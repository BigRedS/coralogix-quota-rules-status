package blockedstatus

import (
	"math"
	"strings"
)

// This file turns the raw API data into the three numbers the brief asked for:
//   1. proportion of the total quota used
//   2. proportion of each quota-rule used
//   3. proportion of the "unassigned" quota (the leftover after the rules) used
//
// Usage arrives already keyed by entity type (from cx_data_usage_units), using
// the same entity-type names as the quota rules, so matching a rule to its
// usage is a plain map lookup — no pillar mapping needed.

// ruleUnits returns how many units a rule reserves. A LOCKED_UNITS rule already
// names a unit figure; a PERCENTAGE rule is that percentage of the total quota.
func ruleUnits(rule QuotaRule, totalQuota float64) float64 {
	if strings.Contains(rule.AllocationType, "PERCENTAGE") {
		return rule.Allocation / 100 * totalQuota
	}
	// Treat anything else (LOCKED_UNITS, or unspecified) as a units figure.
	return rule.Allocation
}

// RuleStatus is the computed result for one quota rule.
type RuleStatus struct {
	EntityType string  `json:"entity_type"`
	Used       float64 `json:"used"`
	Reserved   float64 `json:"reserved"`
	Fraction   float64 `json:"fraction"` // Used / Reserved
}

// Report holds everything we want to print or render.
type Report struct {
	TotalQuota float64 `json:"total_quota"`
	TotalUsed  float64 `json:"total_used"`
	TotalFrac  float64 `json:"total_fraction"`

	Rules []RuleStatus `json:"rules"`

	UnassignedQuota float64 `json:"unassigned_quota"`
	UnassignedUsed  float64 `json:"unassigned_used"`
	UnassignedFrac  float64 `json:"unassigned_fraction"`

	Blocked float64 `json:"blocked"`
}

func safeFraction(used, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return used / total
}

// ComputeReport does all the arithmetic. Only enabled rules count.
func ComputeReport(totalQuota float64, rules []QuotaRule, usage map[string]float64, blocked float64) Report {
	r := Report{
		TotalQuota: totalQuota,
		Blocked:    blocked,
	}

	// Total used is everything we saw usage for, across all entity types.
	for _, used := range usage {
		r.TotalUsed += used
	}
	r.TotalFrac = safeFraction(r.TotalUsed, totalQuota)

	covered := map[string]bool{} // entity types claimed by an enabled rule
	reservedTotal := 0.0

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		reserved := ruleUnits(rule, totalQuota)
		used := usage[rule.EntityType] // 0 if no usage today

		covered[rule.EntityType] = true
		reservedTotal += reserved

		r.Rules = append(r.Rules, RuleStatus{
			EntityType: rule.EntityType,
			Used:       used,
			Reserved:   reserved,
			Fraction:   safeFraction(used, reserved),
		})
	}

	// The unassigned quota is whatever the rules didn't reserve; the unassigned
	// usage is every entity type no enabled rule claimed.
	r.UnassignedQuota = totalQuota - reservedTotal
	// Percentage allocations rarely sum to exactly 100%, so the leftover can be
	// a tiny floating-point residue. Treat anything negligible as no headroom,
	// which also stops safeFraction from dividing by a near-zero denominator.
	if math.Abs(r.UnassignedQuota) < totalQuota*1e-6 {
		r.UnassignedQuota = 0
	}
	for entityType, used := range usage {
		if !covered[entityType] {
			r.UnassignedUsed += used
		}
	}
	r.UnassignedFrac = safeFraction(r.UnassignedUsed, r.UnassignedQuota)

	return r
}

// FetchReport runs the three API calls and computes the report. Both the CLI
// and the web UI use this so they behave identically.
func FetchReport(client *Client) (Report, error) {
	var report Report

	totalQuota, err := client.FetchTotalQuota()
	if err != nil {
		return report, err
	}
	rules, err := client.FetchQuotaRules()
	if err != nil {
		return report, err
	}
	usage, err := client.FetchUsageByEntityType()
	if err != nil {
		return report, err
	}
	blocked, err := client.FetchBlockedUnits()
	if err != nil {
		return report, err
	}

	return ComputeReport(totalQuota, rules, usage, blocked), nil
}
