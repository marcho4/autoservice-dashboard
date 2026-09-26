package domain

import (
	"fmt"
	"regexp"
	"unicode/utf8"
)

var phoneRe = regexp.MustCompile(`^\+?[0-9 ()\-]{5,25}$`)

func validateRequiredText(field, value string, maxLen int) error {
	if value == "" || utf8.RuneCountInString(value) > maxLen {
		return invalid(field, fmt.Sprintf("must be 1-%d characters", maxLen))
	}
	return nil
}

func validateOptionalText(field, value string, maxLen int) error {
	if utf8.RuneCountInString(value) > maxLen {
		return invalid(field, fmt.Sprintf("must be at most %d characters", maxLen))
	}
	return nil
}

func validatePhone(phone string) error {
	if !phoneRe.MatchString(phone) {
		return invalid("phone", "must be a valid phone number")
	}
	return nil
}

func validateOptionalPhone(phone string) error {
	if phone == "" {
		return nil
	}
	return validatePhone(phone)
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
