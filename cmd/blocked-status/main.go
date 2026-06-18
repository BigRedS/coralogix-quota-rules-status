// Command blocked-status reports how close a Coralogix team is to its capacity
// blocks. It talks straight to the Coralogix HTTP API (no cx CLI), so it only
// needs a region name and an API key.
//
// It fetches three things for today and does some simple arithmetic:
//   - the total daily quota, from the PromQL metric cx_data_plan_units_per_day
//   - the quota allocation rules
//   - today's usage so far, keyed by entity type (cx_data_usage_units)
//
// and then prints the proportion of the total, of each rule, and of the
// leftover ("unassigned") quota that has been used.
//
// Written plainly on purpose: standard library only, small functions, one
// obvious step after another. The real work lives in internal/blockedstatus so
// the web UI can reuse it.
package main

import (
	"flag"
	"fmt"
	"os"

	"blocked-status/internal/blockedstatus"
)

func main() {
	region := flag.String("region", "", "Coralogix region: eu1, eu2, us1, us2, ap1, ap2, ap3")
	apiKey := flag.String("api-key", os.Getenv("CX_API_KEY"), "Coralogix API key (or set CX_API_KEY)")
	flag.Parse()

	if *region == "" || *apiKey == "" {
		fmt.Fprintln(os.Stderr, "usage: blocked-status -region <name> -api-key <key>")
		fmt.Fprintln(os.Stderr, "       (api key may also come from the CX_API_KEY environment variable)")
		os.Exit(2)
	}

	host, err := blockedstatus.HostForRegion(*region)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	client := blockedstatus.NewClient(host, *apiKey)
	report, err := blockedstatus.FetchReport(client)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	printReport(report)
}

// printReport writes the report in a plain, readable layout.
func printReport(r blockedstatus.Report) {
	fmt.Println("Blocked-status report (today)")
	fmt.Println("=================================================")

	fmt.Printf("\nTotal quota: %.2f units used of %.2f (%.1f%%)\n",
		r.TotalUsed, r.TotalQuota, 100*r.TotalFrac)

	fmt.Println("\nPer quota-rule:")
	if len(r.Rules) == 0 {
		fmt.Println("  (no enabled rules)")
	}
	for _, rule := range r.Rules {
		fmt.Printf("  %-28s %.2f used of %.2f (%.1f%%)\n",
			rule.EntityType, rule.Used, rule.Reserved, 100*rule.Fraction)
	}

	if r.UnassignedQuota <= 0 {
		fmt.Printf("\nUnassigned: rules reserve the whole quota; no unassigned headroom (%.2f units used outside any rule)\n",
			r.UnassignedUsed)
	} else {
		fmt.Printf("\nUnassigned (everything no rule covers): %.2f used of %.2f (%.1f%%)\n",
			r.UnassignedUsed, r.UnassignedQuota, 100*r.UnassignedFrac)
	}

	fmt.Printf("\nBlocked today: %.2f units\n", r.Blocked)
}
