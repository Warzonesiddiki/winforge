package config

import (
	"encoding/json"
	"testing"
	"testing/fstest"
)

func loadTweaks(t *testing.T, raw string) *TweakConfig {
	t.Helper()
	fsys := fstest.MapFS{"config/tweaks.json": &fstest.MapFile{Data: []byte(raw)}}
	l := &Loader{embedded: fsys}
	c, err := l.LoadTweaks()
	if err != nil {
		t.Fatalf("LoadTweaks: %v", err)
	}
	return c
}

func TestLoadTweaksValid(t *testing.T) {
	raw := `{"tweaks":[
		{"id":"t1","name":"One","category":"privacy","description":"d","risk":"low",
		 "reversible":true,
		 "operations":[{"type":"registry_set_dword","hive":"HKLM","path":"A","name":"B","value":1}]}
	]}`
	c := loadTweaks(t, raw)
	if len(c.Tweaks) != 1 {
		t.Fatalf("want 1 tweak, got %d", len(c.Tweaks))
	}
	if c.Tweaks[0].Risk != RiskLow {
		t.Errorf("want risk low, got %q", c.Tweaks[0].Risk)
	}
}

func TestLoadTweaksDuplicateID(t *testing.T) {
	raw := `{"tweaks":[
		{"id":"dup","name":"A","operations":[{"type":"command","value":"x"}]},
		{"id":"dup","name":"B","operations":[{"type":"command","value":"y"}]}
	]}`
	fsys := fstest.MapFS{"config/tweaks.json": &fstest.MapFile{Data: []byte(raw)}}
	l := &Loader{embedded: fsys}
	if _, err := l.LoadTweaks(); err == nil {
		t.Fatal("expected duplicate-id error")
	}
}

func TestOperationDwordValue(t *testing.T) {
	op := Operation{Value: json.RawMessage(`42`)}
	v, err := op.DwordValue()
	if err != nil {
		t.Fatal(err)
	}
	if v != 42 {
		t.Errorf("want 42, got %d", v)
	}
}

func TestOperationStringValue(t *testing.T) {
	op := Operation{Value: json.RawMessage(`"hello"`)}
	v, err := op.StringValue()
	if err != nil {
		t.Fatal(err)
	}
	if v != "hello" {
		t.Errorf("want hello, got %q", v)
	}
}
