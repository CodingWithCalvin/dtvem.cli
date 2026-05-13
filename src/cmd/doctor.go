package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/doctor"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/ui"
	"github.com/spf13/cobra"
)

var (
	doctorFix   bool
	doctorYes   bool
	doctorNoFix bool
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose dtvem configuration issues",
	Long: `Run a battery of checks against your dtvem installation and surface any
configuration problems it finds.

By default, doctor is read-only: it prints a report and exits non-zero
when an error-severity finding is present, but it does not modify any
state. Pass --fix to enable interactive remediation; pair with --yes
to apply every fixable finding without prompting.

Findings are grouped by severity. Each one is marked either [fixable]
(doctor knows how to remediate it) or [manual] (you'll see step-by-step
instructions). Manual findings are never auto-applied, even with --fix.

Examples:
  dtvem doctor              # Report problems, don't change anything
  dtvem doctor --fix        # Prompt to fix each fixable finding
  dtvem doctor --fix --yes  # Apply every fixable finding non-interactively
  dtvem doctor --no-fix     # Explicit read-only mode (for scripts)`,
	Run: func(cmd *cobra.Command, args []string) {
		if doctorFix && doctorNoFix {
			ui.Error("--fix and --no-fix are mutually exclusive")
			os.Exit(2)
		}

		result := doctor.RunAll()
		renderReport(result)

		// Apply fixes if requested. We run this even when there are no
		// error-severity findings — warnings can also be fixable, and
		// nothing prevents the user from cleaning those up too.
		if doctorFix {
			applyFixes(result, doctorYes)
		}

		if result.HasErrors() {
			// Non-zero exit so CI / wrapping scripts can react. We use
			// os.Exit rather than returning an error from Run so we
			// don't trigger Cobra's "Error:" prefix on the rendered
			// report.
			os.Exit(1)
		}
	},
}

// renderReport prints findings grouped into a passing section and a
// problems section. We deliberately keep the layout close to the
// example in the GitHub issue so the report is greppable and the user
// can find specific finding shapes by eye.
func renderReport(r doctor.Result) {
	ui.Header("dtvem doctor")
	fmt.Println()

	var ok, problems []doctor.CheckResult
	for _, cr := range r.Results {
		if cr.Finding.OK {
			ok = append(ok, cr)
		} else {
			problems = append(problems, cr)
		}
	}

	for _, cr := range problems {
		printFinding(cr.Finding)
	}

	for _, cr := range ok {
		ui.Success("%s", cr.Finding.Title)
	}

	fmt.Println()
	summarize(len(ok), len(problems), r.HasErrors())
}

// printFinding renders a single non-OK finding: a severity-colored
// title line with [fixable] or [manual] tag, the aligned details
// block, and the resolution text.
func printFinding(f doctor.Finding) {
	tag := "[manual]"
	if f.Fixable() {
		tag = "[fixable]"
	}

	header := fmt.Sprintf("%s  %s", f.Title, tag)
	switch f.Severity {
	case doctor.SeverityError:
		ui.Error("%s", header)
	case doctor.SeverityWarning:
		ui.Warning("%s", header)
	default:
		ui.Info("%s", header)
	}

	// Align the keys so the values form a tidy column. The longest key
	// drives column width; we don't pad past it.
	maxKey := 0
	for _, d := range f.Details {
		if l := len(d.Key); l > maxKey {
			maxKey = l
		}
	}
	for _, d := range f.Details {
		pad := strings.Repeat(" ", maxKey-len(d.Key))
		fmt.Printf("  %s:%s  %s\n", d.Key, pad, d.Value)
	}

	if f.Resolution != "" {
		// Indent every line of the resolution so multi-line manual
		// instructions stay visually grouped under the finding.
		for _, line := range strings.Split(f.Resolution, "\n") {
			fmt.Printf("  %s\n", line)
		}
	}
	fmt.Println()
}

// summarize prints the closing one-liner so users (and CI logs) see a
// quick result without having to count findings themselves.
func summarize(ok, problems int, hasErrors bool) {
	if problems == 0 {
		ui.Success("All %d check(s) passed", ok)
		return
	}
	if hasErrors {
		ui.Error("%d problem(s) found across %d check(s)", problems, ok+problems)
	} else {
		ui.Warning("%d non-error problem(s) found across %d check(s)", problems, ok+problems)
	}
}

// applyFixes walks the fixable findings and applies each one — either
// after a y/N prompt, or immediately when --yes was passed. We print a
// running tally so users see what doctor did, in the order it did it.
func applyFixes(r doctor.Result, yes bool) {
	fixable := r.Fixable()
	if len(fixable) == 0 {
		ui.Info("No fixable findings to apply.")
		return
	}

	fmt.Println()
	ui.Header("Applying fixes")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	for _, cr := range fixable {
		if !yes {
			fmt.Printf("Fix: %s? [y/N] ", cr.Finding.Title)
			line, _ := reader.ReadString('\n')
			line = strings.ToLower(strings.TrimSpace(line))
			if line != "y" && line != "yes" {
				ui.Info("Skipped: %s", cr.Finding.Title)
				continue
			}
		}

		if err := cr.Finding.Fix(); err != nil {
			ui.Error("Fix failed for %s: %v", cr.Finding.Title, err)
			continue
		}
		ui.Success("Fixed: %s", cr.Finding.Title)
	}
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "Interactively apply fixes for fixable findings")
	doctorCmd.Flags().BoolVarP(&doctorYes, "yes", "y", false, "Skip prompts when --fix is set; apply all fixable findings")
	doctorCmd.Flags().BoolVar(&doctorNoFix, "no-fix", false, "Explicit read-only mode (for scripts)")
	rootCmd.AddCommand(doctorCmd)
}
