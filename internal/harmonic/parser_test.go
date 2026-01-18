package harmonic

// NOTE: We are implementing table-driven tests here following
// https://dave.cheney.net/2019/05/07/prefer-table-driven-tests

import "testing"

func TestParse(t *testing.T) {
	tests := map[string]struct {
		input   string
		want    []Bookmark
		wantErr bool
	}{
		"single bookmark": {
			input: "3742902q1688536396765",
			want: []Bookmark{
				{ID: 3742902, Timestamp: 1688536396765},
			},
		},
		"multiple bookmarks": {
			input: "3742902q1688536396765-37392676q1748370394349-16582136q1768524091167",
			want: []Bookmark{
				{ID: 3742902, Timestamp: 1688536396765},
				{ID: 37392676, Timestamp: 1748370394349},
				{ID: 16582136, Timestamp: 1768524091167},
			},
		},
		"leading and trailing whitespaces": {
			input: "  3742902q1688536396765  \n",
			want: []Bookmark{
				{ID: 3742902, Timestamp: 1688536396765},
			},
		},
		"leading and trailing dashes": {
			input: "-3742902q1688536396765-",
			want: []Bookmark{
				{ID: 3742902, Timestamp: 1688536396765},
			},
		},
		"empty input": {
			input:   "",
			wantErr: true,
		},
		"consecutive dashes (empty bookmark in between)": {
			input: "3742902q1688536396765---37392676q1748370394349",
			want: []Bookmark{
				{ID: 3742902, Timestamp: 1688536396765},
				{ID: 37392676, Timestamp: 1748370394349},
			},
		},
		"whitespaces only": {
			input:   "  \t\n",
			wantErr: true,
		},
		"whitespace with dashes only": {
			input:   "- -",
			wantErr: true,
		},
		"missing separator": {
			input:   "3742902",
			wantErr: true,
		},
		"invalid item ID": {
			input:   "abc123q1688536396765",
			wantErr: true,
		},
		"missing item ID": {
			input:   "q1688536396765",
			wantErr: true,
		},
		"zero item ID": {
			input:   "0q1688536396765",
			wantErr: true,
		},
		"invalid timestamp": {
			input:   "3742902qabc123",
			wantErr: true,
		},
		"missing timestamp": {
			input:   "3742902q",
			wantErr: true,
		},
		"zero timestamp": {
			input:   "3742902q0",
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := Parse(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Parse() error = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("Parse() got %d bookmarks, want %d", len(got), len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("Parse()[%d] = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}
