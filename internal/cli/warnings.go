package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/agent2host/agent2host/internal/compatibility"
)

var (
	// ErrExecuteRefused is decision: refused. --accept-warnings is ignored.
	ErrExecuteRefused = errors.New("compatibility refused")
	// ErrWarningsNotAccepted is a declined or missing Execute-bound confirmation.
	ErrWarningsNotAccepted = errors.New("warnings not accepted")
	// ErrAcceptanceMismatch is a prior acceptance that does not match this Report.
	ErrAcceptanceMismatch = errors.New("warning acceptance does not match this report")
)

// WarningAcceptance is the user's confirmation of this Report's warnings.
// It is not ExecutionAuthorization.
type WarningAcceptance struct {
	Binding    compatibility.Binding
	Decision   string
	WarningSet string
}

// Matches is true when acceptance is for this Report binding and warning set.
func (a WarningAcceptance) Matches(r compatibility.Report) bool {
	return a.Decision == r.Decision &&
		a.WarningSet == compatibility.WarningSet(r) &&
		a.Binding == compatibility.BindingOf(r) &&
		a.Binding.Fingerprint != ""
}

// ConfirmWarnings records acceptance for an Execute-bound path.
// check must not call this: pure Project is not warning acceptance.
func ConfirmWarnings(r compatibility.Report, acceptFlag, tty bool, stdin io.Reader, stderr io.Writer) (WarningAcceptance, error) {
	switch r.Decision {
	case "refused":
		return WarningAcceptance{}, ErrExecuteRefused
	case "allowed":
		return acceptanceOf(r), nil
	case "allowed_with_warnings":
		if acceptFlag {
			return acceptanceOf(r), nil
		}
		if !tty {
			if stderr != nil {
				fmt.Fprintln(stderr, "allowed_with_warnings: pass --accept-warnings (non-TTY)")
			}
			return WarningAcceptance{}, ErrWarningsNotAccepted
		}
		if err := promptWarnings(r, stdin, stderr); err != nil {
			return WarningAcceptance{}, err
		}
		return acceptanceOf(r), nil
	default:
		return WarningAcceptance{}, fmt.Errorf("unknown decision %q", r.Decision)
	}
}

func acceptanceOf(r compatibility.Report) WarningAcceptance {
	return WarningAcceptance{
		Binding:    compatibility.BindingOf(r),
		Decision:   r.Decision,
		WarningSet: compatibility.WarningSet(r),
	}
}

func promptWarnings(r compatibility.Report, stdin io.Reader, stderr io.Writer) error {
	if stderr != nil {
		fmt.Fprintln(stderr, "This Agent can start, with the following limitations:")
		for _, message := range warningMessages(r) {
			fmt.Fprintf(stderr, "  - %s\n", message)
		}
		fmt.Fprint(stderr, "Start this session? [y/N] ")
	}
	if stdin == nil {
		return ErrWarningsNotAccepted
	}
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	default:
		return ErrWarningsNotAccepted
	}
}

func warningMessages(r compatibility.Report) []string {
	set := compatibility.WarningSet(r)
	lines := strings.Split(set, "\n")
	if len(lines) <= 1 {
		return []string{"Some requested Agent behavior may differ on this Host."}
	}
	seen := make(map[string]struct{}, len(lines)-1)
	var messages []string
	for _, code := range lines[1:] {
		message := warningMessage(code)
		if _, ok := seen[message]; ok {
			continue
		}
		seen[message] = struct{}{}
		messages = append(messages, message)
	}
	if len(messages) == 0 {
		return []string{"Some requested Agent behavior may differ on this Host."}
	}
	return messages
}

func warningMessage(code string) string {
	switch code {
	case "mapped_activation":
		return "This Host will use its standard interface instead of a native named-Agent view."
	case "visible_loss":
		return "Some optional Agent setup cannot be reproduced exactly on this Host."
	case "confidence_inferred":
		return "Some behavior was inferred for this Host version and has not been directly verified."
	case "permission_overgrant":
		return "This Host may allow more access than the Agent requested."
	default:
		return "Some requested Agent behavior may differ on this Host."
	}
}
