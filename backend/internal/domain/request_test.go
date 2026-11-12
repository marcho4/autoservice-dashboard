package domain

import (
	"errors"
	"testing"
)

func TestCanTransition(t *testing.T) {
	cases := []struct {
		from, to RequestStatus
		actor    Actor
		want     bool
	}{
		{StatusNew, StatusInProgress, ActorEmployee, true},
		{StatusNew, StatusRejected, ActorEmployee, true},
		{StatusNew, StatusCancelled, ActorClient, true},
		{StatusInProgress, StatusClosed, ActorEmployee, true},

		{StatusNew, StatusCancelled, ActorEmployee, false},
		{StatusNew, StatusInProgress, ActorClient, false},
		{StatusNew, StatusClosed, ActorEmployee, false},
		{StatusInProgress, StatusNew, ActorEmployee, false},
		{StatusInProgress, StatusRejected, ActorEmployee, false},
		{StatusInProgress, StatusCancelled, ActorClient, false},
		{StatusClosed, StatusInProgress, ActorEmployee, false},
		{StatusRejected, StatusNew, ActorEmployee, false},
		{StatusCancelled, StatusNew, ActorClient, false},
		{StatusNew, StatusNew, ActorEmployee, false},
	}
	for _, c := range cases {
		if got := CanTransition(c.from, c.to, c.actor); got != c.want {
			t.Errorf("CanTransition(%s, %s, %d) = %v, want %v", c.from, c.to, c.actor, got, c.want)
		}
	}
}

func TestRequestInputNormalize(t *testing.T) {
	in := RequestInput{
		ServiceName: " Диагностика ",
		Answers:     []Answer{{Label: " Автомобиль ", Value: " Toyota Camry "}, {Label: "Пробег", Value: "  "}},
		Phone:       "8 (999) 123-45-67",
	}
	got, err := in.Normalize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ServiceName != "Диагностика" || got.Phone != "+79991234567" {
		t.Errorf("not normalized: %+v", got)
	}
	if len(got.Answers) != 1 || got.Answers[0] != (Answer{Label: "Автомобиль", Value: "Toyota Camry"}) {
		t.Errorf("answers must be trimmed and empty ones dropped: %+v", got.Answers)
	}

	in.Phone = "call me"
	_, err = in.Normalize()
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "phone" || !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected phone validation error, got %v", err)
	}
}

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{
		"+7 (900) 111-22-33": "+79001112233",
		"89001112233":        "+79001112233",
		"9001112233":         "+79001112233",
		"79001112233":        "+79001112233",
		"+375 29 123-45-67":  "+375291234567",
	}
	for in, want := range cases {
		if got, err := NormalizePhone(in); err != nil || got != want {
			t.Errorf("NormalizePhone(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "12345", "позвоните мне", "+7 900 111"} {
		if _, err := NormalizePhone(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
	if got := FormatPhone("+79001112233"); got != "+7 900 111-22-33" {
		t.Errorf("FormatPhone = %q", got)
	}
}

func TestServiceInputNormalize(t *testing.T) {
	in := ServiceInput{Name: " Шиномонтаж ", Fields: []FormField{
		{ID: "car1", Label: "Автомобиль", Type: FieldText, Required: true},
		{ID: "car1", Label: "Вид работ", Type: FieldChoice, Options: []string{"Смена шин", " смена  шин ", "Балансировка", ""}},
		{Label: "Пробег", Type: FieldNumber, Options: []string{"лишнее"}},
	}}
	got, err := in.Normalize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Шиномонтаж" || len(got.Fields) != 3 {
		t.Fatalf("unexpected service: %+v", got)
	}
	ids := map[string]bool{}
	for _, f := range got.Fields {
		if !fieldIDRe.MatchString(f.ID) || ids[f.ID] {
			t.Errorf("field IDs must be valid and unique: %+v", got.Fields)
		}
		ids[f.ID] = true
	}
	if got.Fields[0].ID != "car1" {
		t.Errorf("a valid client ID must be kept: %q", got.Fields[0].ID)
	}
	if len(got.Fields[1].Options) != 2 || got.Fields[2].Options != nil {
		t.Errorf("options not cleaned: %+v", got.Fields)
	}

	for name, bad := range map[string]ServiceInput{
		"empty name":      {Name: " "},
		"unknown type":    {Name: "x", Fields: []FormField{{Label: "a", Type: "date"}}},
		"empty label":     {Name: "x", Fields: []FormField{{Type: FieldText}}},
		"one option":      {Name: "x", Fields: []FormField{{Label: "a", Type: FieldChoice, Options: []string{"only"}}}},
		"too many fields": {Name: "x", Fields: make([]FormField, MaxServiceFields+1)},
	} {
		if _, err := bad.Normalize(); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: expected validation error, got %v", name, err)
		}
	}

	for _, s := range DefaultServices() {
		if _, err := s.Normalize(); err != nil {
			t.Errorf("default service %q is invalid: %v", s.Name, err)
		}
		for _, f := range s.Fields {
			if !fieldIDRe.MatchString(f.ID) {
				t.Errorf("default service %q has a field without an ID: %+v", s.Name, f)
			}
		}
	}
}

func TestFormFieldParseAnswer(t *testing.T) {
	choice := FormField{Type: FieldChoice, Options: []string{"Смена шин", "Балансировка"}}
	cases := []struct {
		field FormField
		in    string
		want  string
		ok    bool
	}{
		{FormField{Type: FieldText}, "  Toyota   Camry ", "Toyota Camry", true},
		{FormField{Type: FieldText}, "   ", "", false},
		{FormField{Type: FieldNumber}, "120 000", "120000", true},
		{FormField{Type: FieldNumber}, "1,5", "1.5", true},
		{FormField{Type: FieldNumber}, "около 120 тысяч", "", false},
		{FormField{Type: FieldNumber}, "-5", "", false},
		{choice, "балансировка", "Балансировка", true},
		{choice, "2", "Балансировка", true},
		{choice, "покраска", "", false},
	}
	for _, c := range cases {
		if got, ok := c.field.ParseAnswer(c.in); got != c.want || ok != c.ok {
			t.Errorf("ParseAnswer(%s, %q) = %q, %v; want %q, %v", c.field.Type, c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestMaskBotToken(t *testing.T) {
	got := MaskBotToken("123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw")
	if got != "123456789:****Dsaw" {
		t.Errorf("unexpected mask: %s", got)
	}
}

func TestNormalizeEmail(t *testing.T) {
	if e, err := NormalizeEmail("  Master@Example.COM "); err != nil || e != "master@example.com" {
		t.Errorf("got %q, %v", e, err)
	}
	for _, bad := range []string{"", "no-at", "Name <a@b.c>"} {
		if _, err := NormalizeEmail(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}
