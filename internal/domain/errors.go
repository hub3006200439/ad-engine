package domain

import "errors"

var (
	ErrNoFill         = errors.New("no fill")
	ErrBudgetExceeded = errors.New("budget exceeded")
)
