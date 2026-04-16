package compat

import "testing"

func TestDetectSearchPreference(t *testing.T) {
	cases := []struct {
		keyword string
		want    SearchPreference
	}{
		{keyword: "周杰伦", want: SearchPreferenceLocalFirst},
		{keyword: "jay chou", want: SearchPreferenceJamendoFirst},
		{keyword: "周杰伦 jay", want: SearchPreferenceLocalFirst},
		{keyword: "12345", want: SearchPreferenceLocalFirst},
	}

	for _, tc := range cases {
		if got := DetectSearchPreference(tc.keyword); got != tc.want {
			t.Fatalf("keyword %q: want %v, got %v", tc.keyword, tc.want, got)
		}
	}
}

func TestJamendoVirtualPathRoundTrip(t *testing.T) {
	virtualPath := BuildJamendoVirtualPath(`May/Day: "Circus"`, "1218138")
	if virtualPath == "" {
		t.Fatal("expected non-empty virtual path")
	}
	if got, ok := ParseJamendoSourceID(virtualPath); !ok || got != "1218138" {
		t.Fatalf("expected source id 1218138, got %q ok=%v", got, ok)
	}
}
