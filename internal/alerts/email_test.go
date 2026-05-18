package alerts

import (
	"strings"
	"testing"
)

func TestBuildMessage_SingleAlert_SubjectIsName(t *testing.T) {
	msg := buildMessage("monitor@example.com", "user@example.com", []namedAlert{
		{Key: "battery_critical", Name: "Battery Critically Low"},
	})
	if !strings.Contains(msg, "Subject: Home Monitor: Battery Critically Low") {
		t.Errorf("expected single-alert subject, got:\n%s", msg)
	}
}

func TestBuildMessage_MultipleAlerts_SubjectHasCount(t *testing.T) {
	msg := buildMessage("monitor@example.com", "user@example.com", []namedAlert{
		{Key: "battery_critical", Name: "Battery Critically Low"},
		{Key: "house_overload", Name: "House Power Overload"},
	})
	if !strings.Contains(msg, "Subject: Home Monitor: 2 Active Alerts") {
		t.Errorf("expected 2-alert subject, got:\n%s", msg)
	}
}

func TestBuildMessage_BodyContainsAllNames(t *testing.T) {
	msg := buildMessage("from@example.com", "to@example.com", []namedAlert{
		{Key: "a", Name: "Alpha Alert"},
		{Key: "b", Name: "Beta Alert"},
	})
	if !strings.Contains(msg, "Alpha Alert") {
		t.Error("body missing Alpha Alert")
	}
	if !strings.Contains(msg, "Beta Alert") {
		t.Error("body missing Beta Alert")
	}
}

func TestBuildMessage_Headers(t *testing.T) {
	msg := buildMessage("from@x.com", "to@y.com", []namedAlert{{Key: "k", Name: "N"}})
	if !strings.Contains(msg, "From: from@x.com") {
		t.Error("missing From header")
	}
	if !strings.Contains(msg, "To: to@y.com") {
		t.Error("missing To header")
	}
	if !strings.Contains(msg, "Content-Type: text/plain") {
		t.Error("missing Content-Type header")
	}
}
