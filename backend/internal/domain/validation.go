package domain

import (
	"fmt"
	"regexp"
	"strings"
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

// NormalizePhone turns any common spelling of a phone number into +<digits>.
// Russian numbers written as 8XXXXXXXXXX or 9XXXXXXXXX become +7XXXXXXXXXX.
func NormalizePhone(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if !phoneRe.MatchString(raw) {
		return "", invalid("phone", "must be a valid phone number")
	}
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	hasPlus := strings.HasPrefix(raw, "+")
	switch {
	case !hasPlus && len(digits) == 11 && digits[0] == '8':
		digits = "7" + digits[1:]
	case !hasPlus && len(digits) == 10 && digits[0] == '9':
		digits = "7" + digits
	}
	if len(digits) < 10 || len(digits) > 15 {
		return "", invalid("phone", "must be a valid phone number")
	}
	return "+" + digits, nil
}

// FormatPhone prints a normalized Russian number as +7 900 111-22-33 and leaves other numbers as is.
func FormatPhone(phone string) string {
	if len(phone) == 12 && strings.HasPrefix(phone, "+7") {
		return phone[:2] + " " + phone[2:5] + " " + phone[5:8] + "-" + phone[8:10] + "-" + phone[10:]
	}
	return phone
}
