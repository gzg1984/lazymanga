package rulebook

import "fmt"

// InvalidRuleBookError indicates malformed rule book content.
type InvalidRuleBookError struct {
	Reason string
}

func (e InvalidRuleBookError) Error() string {
	return fmt.Sprintf("invalid rule book: %s", e.Reason)
}

// ErrInvalidRuleBook builds a typed invalid-rulebook error.
func ErrInvalidRuleBook(reason string) error {
	return InvalidRuleBookError{Reason: reason}
}
