package main

import (
	"regexp"
	"testing"
)

func TestDescribeSIA(t *testing.T) {
	tests := map[string]string{
		"BA":  "Burglary Alarm",
		"BR":  "Burglary Restoral",
		"UA":  "Untyped Zone Alarm",
		"UR":  "Untyped Zone Restoral",
		"CL":  "Closing Report",
		"OP":  "Opening Report",
		"RP":  "Automatic Test",
		"YT":  "System Battery Trouble",
		"YR":  "System Battery Restoral",
		"FA":  "Fire Alarm",
		"AT":  "AC Trouble",
		"AR":  "AC Restoral",
		"TA":  "Tamper Alarm",
		"ZU":  "Freeze Unbypass",
		"QQ":  "",
		"ba":  "",
		"":    "",
		"BAX": "",
	}
	for code, want := range tests {
		if got := DescribeSIA(code); got != want {
			t.Errorf("DescribeSIA(%q) = %q, want %q", code, got, want)
		}
	}
}

func TestSIACodesTable(t *testing.T) {
	codeRegex := regexp.MustCompile(`^[A-Z]{2}$`)
	if len(siaCodes) < 200 {
		t.Errorf("only %d codes in table", len(siaCodes))
	}
	for code, desc := range siaCodes {
		if !codeRegex.MatchString(code) || desc == "" {
			t.Errorf("bad entry %q: %q", code, desc)
		}
	}
}

func TestParseMessageDescription(t *testing.T) {
	m := ParseMessage([]byte("01010053\"SIA-DCS\"0007R0075L0001[#001465|NBA1/QQ2/RP000*'DECKERS'NM]"))
	want := []string{"Burglary Alarm", "", "Automatic Test"}
	if len(m.Events) != len(want) {
		t.Fatalf("Events = %+v", m.Events)
	}
	for i, ev := range m.Events {
		if ev.Description != want[i] {
			t.Errorf("event %d (%s): Description = %q, want %q", i, ev.Code, ev.Description, want[i])
		}
	}
}
