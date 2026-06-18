package forms

import "testing"

func TestBuildFilenameCameraDailies(t *testing.T) {
	f, _ := Get("masters")
	got := BuildFilename(f, map[string]string{
		"project": "PROJ",
		"title":   "cold open",
	}, ".r3d")
	if want := "PROJ_cold_open.r3d"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildFilenameAllEmpty(t *testing.T) {
	f, _ := Get("masters")
	got := BuildFilename(f, map[string]string{}, ".mov")
	if want := "upload.mov"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestValidateMinLength(t *testing.T) {
	f, _ := Get("masters")
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
	f, _ := Get("masters")
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

func TestBuildFilenameOslofjord(t *testing.T) {
	f, ok := Get("oslofjord_delivery")
	if !ok {
		t.Fatal("oslofjord_delivery not registered")
	}
	// All present.
	got := BuildFilename(f, map[string]string{
		"arrangement": "SMR", "subEvent": "MOT", "post": "12", "type": "VIDEO", "navn": "temafilm",
	}, ".mov")
	if want := "SMR_MOT_12_VIDEO_temafilm.mov"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// Sub event "-" (empty) + no post/type collapse out.
	got = BuildFilename(f, map[string]string{
		"arrangement": "SMR", "subEvent": "", "navn": "temafilm",
	}, ".mov")
	if want := "SMR_temafilm.mov"; got != want {
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
	want := []string{"masters", "oslofjord_delivery"}
	if len(keys) != len(want) {
		t.Fatalf("unexpected keys: %v", keys)
	}
	for i, k := range want {
		if keys[i] != k {
			t.Errorf("keys[%d] = %q, want %q (full: %v)", i, keys[i], k, keys)
		}
	}
}
