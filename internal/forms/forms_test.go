package forms

import "testing"

func TestBuildFilenameBCCMedia(t *testing.T) {
	f, ok := Get("bcc_media")
	if !ok {
		t.Fatal("bcc_media not registered")
	}
	// All optional fields empty -> ARR_SUB_navn pattern.
	got := BuildFilename(f, map[string]string{
		"arrangement": "ARR",
		"subEvent":    "SUB",
		"navn":        "temafilm",
	}, ".mov")
	if want := "ARR_SUB_temafilm.mov"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildFilenameSelectByLabel(t *testing.T) {
	f, _ := Get("bcc_media")
	// A select value given as the option label resolves to its code.
	got := BuildFilename(f, map[string]string{
		"arrangement": "Sommerstevne",
		"subEvent":    "Møte",
		"navn":        "opp tak",
	}, ".mp4")
	if want := "SMR_MØT_opp_tak.mp4"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildFilenameCameraDailies(t *testing.T) {
	f, _ := Get("camera_dailies")
	got := BuildFilename(f, map[string]string{
		"project": "PROJ",
		"title":   "cold open",
	}, ".r3d")
	if want := "PROJ_cold_open.r3d"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildFilenameAllEmpty(t *testing.T) {
	f, _ := Get("camera_dailies")
	got := BuildFilename(f, map[string]string{}, ".mov")
	if want := "upload.mov"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestValidate(t *testing.T) {
	f, _ := Get("bcc_media")
	err := Validate(f, map[string]string{"arrangement": "ARR", "subEvent": "SUB"})
	if err == nil {
		t.Error("expected error for missing required field navn")
	}
	if err := Validate(f, map[string]string{
		"arrangement": "ARR", "subEvent": "SUB", "navn": "x",
	}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateMinLength(t *testing.T) {
	f, _ := Get("camera_dailies")
	// Title shorter than 5 chars is rejected.
	if err := Validate(f, map[string]string{"project": "DOC", "title": "abc"}); err == nil {
		t.Error("expected error for title shorter than MinLength")
	}
	// 5+ chars passes.
	if err := Validate(f, map[string]string{"project": "DOC", "title": "cold open"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildFilenameOptionalSegments(t *testing.T) {
	f, _ := Get("camera_dailies")
	// Season + episode present.
	got := BuildFilename(f, map[string]string{
		"project": "DOC", "season": "S1", "episode": "E2", "title": "cold open",
	}, ".mov")
	if want := "DOC_S1_E2_cold_open.mov"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// Both optional segments blank collapse cleanly.
	got = BuildFilename(f, map[string]string{"project": "DOC", "title": "cold open"}, ".mov")
	if want := "DOC_cold_open.mov"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGetUnknown(t *testing.T) {
	if _, ok := Get("nope"); ok {
		t.Error("expected unknown key to return false")
	}
}

func TestKeysSorted(t *testing.T) {
	keys := Keys()
	if len(keys) != 2 || keys[0] != "bcc_media" || keys[1] != "camera_dailies" {
		t.Errorf("unexpected keys: %v", keys)
	}
}
