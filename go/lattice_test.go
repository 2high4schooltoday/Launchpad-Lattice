package launchpadlattice

import (
	"strings"
	"testing"
)

func TestParseSimplePayloadDocument(t *testing.T) {
	value, err := ParseDocument(`
lattice 1.

data:
  appId is "hello".
  retries is 3.
  enabled is true.
  architectures is list:
    item is "linux-x86_64".
    item is "linux-arm64".
  extension is record:
    bundle is uri "file:ext/demo.lext".
    sha256 is digest "sha256:abc".
`)
	if err != nil {
		t.Fatalf("ParseDocument returned error: %v", err)
	}

	record := value.(map[string]any)
	if record["appId"] != "hello" {
		t.Fatalf("appId = %v", record["appId"])
	}
	if record["retries"] != int64(3) {
		t.Fatalf("retries = %v", record["retries"])
	}
	if record["enabled"] != true {
		t.Fatalf("enabled = %v", record["enabled"])
	}
	architectures := record["architectures"].([]any)
	if architectures[0] != "linux-x86_64" {
		t.Fatalf("architectures[0] = %v", architectures[0])
	}
	extension := record["extension"].(map[string]any)
	if extension["bundle"] != "file:ext/demo.lext" {
		t.Fatalf("extension.bundle = %v", extension["bundle"])
	}
}

func TestIgnoresNonDataSectionsWhenDataIsPresent(t *testing.T) {
	value, err := ParseDocument(`
lattice 1.

document is "org.example.demo".
schema is "org.example/demo/1".

data is Defaults:
  installDir is path "/opt/demo".
`)
	if err != nil {
		t.Fatalf("ParseDocument returned error: %v", err)
	}

	record := value.(map[string]any)
	if record["installDir"] != "/opt/demo" {
		t.Fatalf("installDir = %v", record["installDir"])
	}
}

func TestRendersRoundTripPayloads(t *testing.T) {
	original := map[string]any{
		"appId":     "hello",
		"enabled":   true,
		"threshold": 2.5,
		"labels": map[string]any{
			"io.launchpad.app/name": "hello",
		},
		"architectures": []any{"linux-x86_64", "linux-arm64"},
	}

	rendered, err := Marshal(original)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	reparsed, err := ParseDocument(rendered)
	if err != nil {
		t.Fatalf("ParseDocument(rendered) returned error: %v", err)
	}

	record := reparsed.(map[string]any)
	if record["appId"] != "hello" || record["enabled"] != true {
		t.Fatalf("reparsed basic fields = %#v", record)
	}
}

func TestRejectsDuplicateKeys(t *testing.T) {
	_, err := ParseDocument(`
lattice 1.

data:
  appId is "hello".
  appId is "duplicate".
`)
	if err == nil {
		t.Fatal("expected ParseDocument to fail")
	}
	if !strings.Contains(err.Error(), "duplicate key `appId`") {
		t.Fatalf("unexpected error: %v", err)
	}
}
