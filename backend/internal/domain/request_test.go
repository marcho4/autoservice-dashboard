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
	in := RequestInput{CarBrand: " Toyota ", CarModel: "Camry", Description: "Стучит подвеска", Phone: "+7 (999) 123-45-67"}
	got, err := in.Normalize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CarBrand != "Toyota" {
		t.Errorf("brand not trimmed: %q", got.CarBrand)
	}

	in.Phone = "call me"
	_, err = in.Normalize()
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "phone" || !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected phone validation error, got %v", err)
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
