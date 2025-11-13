package main

import (
	"testing"

	"github.com/censys/scan-takehome/pkg/scanning"
)

func TestNormalize_V1(t *testing.T) {
	raw := []byte("service response: 42")

	scan := scanning.Scan{
		DataVersion: scanning.V1,
		Data: &scanning.V1Data{
			ResponseBytesUtf8: raw,
		},
	}

	got := normalize(scan)
	want := "service response: 42"

	if got != want {
		t.Fatalf("normalize(V1) = %q, want %q", got, want)
	}
}

func TestNormalize_V2(t *testing.T) {
	resp := "service response: 42"

	scan := scanning.Scan{
		DataVersion: scanning.V2,
		Data: &scanning.V2Data{
			ResponseStr: resp,
		},
	}

	got := normalize(scan)
	if got != resp {
		t.Fatalf("normalize(V2) = %q, want %q", got, resp)
	}
}

func TestNormalize_CorruptedData(t *testing.T) {

	scan := scanning.Scan{
		DataVersion: scanning.V1,
		Data: map[string]any{
			"wrong_field": "corrupted_value",
		},
	}

	got := normalize(scan)
	if got != "" {
		t.Fatalf("normalize(corrupted) = %q, want empty string", got)
	}
}
