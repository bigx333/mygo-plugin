package plugin

import (
	"errors"
	"fmt"
)

var (
	// ErrSignIn says the listener has to sign in for this.
	ErrSignIn = errors.New("sign in first")
	// ErrSetup says a setting the plugin needs is missing.
	ErrSetup = errors.New("the plugin needs setting up")
	// ErrUnavailable says what was asked for can't be had.
	ErrUnavailable = errors.New("not available")
	// ErrUnsupported says the plugin doesn't do this.
	ErrUnsupported = errors.New("not supported by this plugin")
)

// thrownError is the error a plugin threw, keeping its code as one the host
// can act on.
func thrownError(code, message string) error {
	var known error
	switch code {
	case "signIn":
		known = ErrSignIn
	case "setup":
		known = ErrSetup
	case "unavailable":
		known = ErrUnavailable
	case "unsupported":
		known = ErrUnsupported
	default:
		if message == "" {
			return errors.New("the plugin failed")
		}
		return errors.New(message)
	}
	if message == "" {
		return known
	}
	return fmt.Errorf("%w: %s", known, message)
}
