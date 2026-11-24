package engine

import "testing"

func TestParseProgressLine(t *testing.T) {
	dl, ok := ParseProgressLine("STASH_DL| 42.6%|10485760|24616960|NA|5242880.0|3")
	if !ok || dl.Kind != "download" {
		t.Fatalf("expected download event, got %+v (ok=%v)", dl, ok)
	}
	if dl.Percent != 42.6 || dl.DownloadedBytes != 10485760 || dl.TotalBytes != 24616960 || dl.SpeedBytes != 5242880 || dl.ETASeconds != 3 {
		t.Fatalf("unexpected download values: %+v", dl)
	}

	pp, ok := ParseProgressLine("STASH_PP|started")
	if !ok || pp.Kind != "postprocess" || pp.PostStatus != "started" {
		t.Fatalf("expected postprocess event, got %+v", pp)
	}

	fin, ok := ParseProgressLine("STASH_FINAL|/tmp/stash-jobs/abc/media.mp4")
	if !ok || fin.Kind != "final" || fin.FinalPath != "/tmp/stash-jobs/abc/media.mp4" {
		t.Fatalf("expected final event, got %+v", fin)
	}

	if _, ok := ParseProgressLine("unrelated noise"); ok {
		t.Fatal("expected unrelated line to be ignored")
	}
}
